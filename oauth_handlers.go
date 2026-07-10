package main

// Third-party (OAuth 2.0) sign-in, proxied through this gateway.
//
// The frontends never see the OAuth client secret or perform token exchange.
// Two entry points cover the two frontend architectures:
//
//   - Browser flow (public Front-End, which talks to this gateway directly):
//       GET /api/auth/{provider}/start     -> 302 to the identity provider
//       GET /api/auth/{provider}/callback  -> exchange code, set sessionid, 302 back
//
//   - Server-to-server exchange (Admin Front-End, whose Next.js server proxies
//     everything so the sessionid cookie must be issued on the admin origin):
//       POST /api/auth/{provider}/exchange -> exchange code, return sessionid cookie
//
// A dev mode (OAUTH_DEV_MODE=true) short-circuits the external provider so the
// whole redirect + cookie-session flow is testable locally without Google
// credentials, mirroring the frontend "mock-oauth" provider.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// oauthProvider describes a third-party identity provider. Endpoints are fixed
// per provider; client credentials are read from the environment.
type oauthProvider struct {
	name         string
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	userinfoURL  string
	scopes       string
}

// knownOAuthEndpoints holds the fixed OAuth endpoints for supported providers.
// Credentials come from {NAME}_CLIENT_ID / {NAME}_CLIENT_SECRET.
var knownOAuthEndpoints = map[string]oauthProvider{
	"google": {
		authURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL:    "https://oauth2.googleapis.com/token",
		userinfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
		scopes:      "openid email profile",
	},
	"microsoft": {
		authURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		tokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		userinfoURL: "https://graph.microsoft.com/oidc/userinfo",
		scopes:      "openid email profile",
	},
}

// getOAuthProvider resolves a provider by name, layering the environment
// credentials onto its fixed endpoints. Returns false for unknown providers.
func getOAuthProvider(name string) (oauthProvider, bool) {
	base, ok := knownOAuthEndpoints[name]
	if !ok {
		return oauthProvider{}, false
	}
	envName := strings.ToUpper(name)
	base.name = name
	base.clientID = os.Getenv(envName + "_CLIENT_ID")
	base.clientSecret = os.Getenv(envName + "_CLIENT_SECRET")
	return base, true
}

func oauthDevMode() bool {
	return strings.EqualFold(os.Getenv("OAUTH_DEV_MODE"), "true")
}

// oauthHTTPClient is used for the token and userinfo requests.
var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

// oauthFindOrCreateUser resolves a third-party identity (by email) to a local
// user id, provisioning the user on first sign-in. It is a package var so tests
// can stub out the database dependency (real implementation in db_api.go).
var oauthFindOrCreateUser = findOrCreateOAuthUserDB

// --- CSRF state store --------------------------------------------------------

type oauthFlowState struct {
	provider    string
	redirectURI string // frontend URL to return the user to
	createdAt   time.Time
}

const oauthStateTTL = 10 * time.Minute

var (
	oauthStatesMu sync.Mutex
	oauthStates   = map[string]oauthFlowState{}
)

func newOAuthState(provider, redirectURI string) string {
	state := generateSessionID()
	oauthStatesMu.Lock()
	oauthStates[state] = oauthFlowState{provider: provider, redirectURI: redirectURI, createdAt: time.Now()}
	oauthStatesMu.Unlock()
	return state
}

// consumeOAuthState looks up and removes a state token (single use), rejecting
// unknown or expired tokens.
func consumeOAuthState(state string) (oauthFlowState, bool) {
	oauthStatesMu.Lock()
	defer oauthStatesMu.Unlock()
	fs, ok := oauthStates[state]
	if !ok {
		return oauthFlowState{}, false
	}
	delete(oauthStates, state)
	if time.Since(fs.createdAt) > oauthStateTTL {
		return oauthFlowState{}, false
	}
	return fs, true
}

// --- Helpers -----------------------------------------------------------------

// defaultClientOrigin returns the first configured CORS origin, used as the
// fallback frontend for post-login redirects.
func defaultClientOrigin() string {
	if origins := allowedOrigins(); len(origins) > 0 {
		return origins[0]
	}
	return "http://localhost:3000"
}

// resolveFrontendRedirect validates the requested post-login redirect URL,
// requiring its origin to be in our CORS allowlist to prevent open redirects.
// Falls back to the default client origin's /auth/callback when missing/invalid.
func resolveFrontendRedirect(raw string) string {
	fallback := strings.TrimRight(defaultClientOrigin(), "/") + "/auth/callback"
	if raw == "" {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fallback
	}
	if !originAllowed(u.Scheme + "://" + u.Host) {
		log.Printf("[oauth] rejected redirect_uri with disallowed origin: %s://%s", u.Scheme, u.Host)
		return fallback
	}
	return raw
}

// backendBaseURL returns this gateway's public base URL, used to build the
// provider's redirect_uri (which must point back to our own callback). Prefers
// the explicit PUBLIC_API_URL; otherwise derives it from the request.
func backendBaseURL(r *http.Request) string {
	if v := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/"); v != "" {
		return v
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func backendCallbackURL(r *http.Request, provider string) string {
	return backendBaseURL(r) + "/api/auth/" + url.PathEscape(provider) + "/callback"
}

// appendQuery adds a single key/value pair to a URL, choosing ? or & as needed.
func appendQuery(rawURL, key, value string) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

// devUserEmail is the identity provisioned when OAUTH_DEV_MODE is enabled.
func devUserEmail() string {
	return getenvDefault("OAUTH_DEV_EMAIL", "dev.oauth@motzklist.local")
}

// --- Core exchange logic -----------------------------------------------------

// resolveOAuthUser runs the authorization-code exchange and userinfo lookup for
// a real provider, returning the local user id (provisioning on first login).
func resolveOAuthUser(provider oauthProvider, code, redirectURI string) (string, error) {
	token, err := exchangeOAuthCode(provider, code, redirectURI)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	email, name, err := fetchOAuthUserInfo(provider, token)
	if err != nil {
		return "", fmt.Errorf("userinfo: %w", err)
	}
	if email == "" {
		return "", fmt.Errorf("provider returned no email")
	}
	return oauthFindOrCreateUser(email, name)
}

func exchangeOAuthCode(p oauthProvider, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	resp, err := oauthHTTPClient.PostForm(p.tokenURL, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("no access_token in token response")
	}
	return tok.AccessToken, nil
}

func fetchOAuthUserInfo(p oauthProvider, accessToken string) (email, name string, err error) {
	req, err := http.NewRequest(http.MethodGet, p.userinfoURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", err
	}
	return info.Email, info.Name, nil
}

// --- Handlers ----------------------------------------------------------------

// oauthStartHandler begins the browser redirect flow (public Front-End).
func oauthStartHandler(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	frontendRedirect := resolveFrontendRedirect(r.URL.Query().Get("redirect_uri"))

	// Dev mode: skip the identity provider entirely and log in a dev user, so
	// the redirect + cookie flow works locally without external credentials.
	if oauthDevMode() {
		uid, err := oauthFindOrCreateUser(devUserEmail(), "OAuth Dev User")
		if err != nil {
			log.Printf("[oauth] dev-mode provisioning failed: %v", err)
			http.Redirect(w, r, appendQuery(frontendRedirect, "error", "server_error"), http.StatusFound)
			return
		}
		startSession(w, uid)
		http.Redirect(w, r, frontendRedirect, http.StatusFound)
		return
	}

	provider, ok := getOAuthProvider(providerName)
	if !ok {
		JSONError(w, "Unknown auth provider", http.StatusNotFound)
		return
	}
	if provider.clientID == "" || provider.clientSecret == "" {
		JSONError(w, fmt.Sprintf("Auth provider %q is not configured", providerName), http.StatusServiceUnavailable)
		return
	}

	state := newOAuthState(providerName, frontendRedirect)
	q := url.Values{}
	q.Set("client_id", provider.clientID)
	q.Set("redirect_uri", backendCallbackURL(r, providerName))
	q.Set("response_type", "code")
	q.Set("scope", provider.scopes)
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "select_account")

	http.Redirect(w, r, provider.authURL+"?"+q.Encode(), http.StatusFound)
}

// oauthCallbackHandler completes the browser redirect flow: it exchanges the
// code, establishes the session, and redirects back to the frontend.
func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	query := r.URL.Query()

	flow, ok := consumeOAuthState(query.Get("state"))
	if !ok || flow.provider != providerName {
		JSONError(w, "Invalid or expired OAuth state", http.StatusBadRequest)
		return
	}
	// From here we redirect failures back to the frontend so it can show them.
	if errParam := query.Get("error"); errParam != "" {
		http.Redirect(w, r, appendQuery(flow.redirectURI, "error", errParam), http.StatusFound)
		return
	}
	code := query.Get("code")
	if code == "" {
		http.Redirect(w, r, appendQuery(flow.redirectURI, "error", "missing_code"), http.StatusFound)
		return
	}

	provider, ok := getOAuthProvider(providerName)
	if !ok {
		JSONError(w, "Unknown auth provider", http.StatusNotFound)
		return
	}

	uid, err := resolveOAuthUser(provider, code, backendCallbackURL(r, providerName))
	if err != nil {
		log.Printf("[oauth] sign-in failed: %v", err)
		http.Redirect(w, r, appendQuery(flow.redirectURI, "error", "signin_failed"), http.StatusFound)
		return
	}
	startSession(w, uid)
	http.Redirect(w, r, flow.redirectURI, http.StatusFound)
}

// oauthExchangeHandler completes a server-to-server code exchange (Admin
// Front-End). The admin's Next.js server ran the browser redirect to the
// provider (with its own callback + state), and posts the resulting code here.
// We set the sessionid cookie on the response so the admin proxy can relay it
// onto the admin origin, and return the user for convenience.
func oauthExchangeHandler(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")

	var body struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	var uid string
	if oauthDevMode() {
		var err error
		if uid, err = oauthFindOrCreateUser(devUserEmail(), "OAuth Dev User"); err != nil {
			log.Printf("[oauth] dev-mode provisioning failed: %v", err)
			JSONError(w, "Sign-in failed", http.StatusInternalServerError)
			return
		}
	} else {
		provider, ok := getOAuthProvider(providerName)
		if !ok {
			JSONError(w, "Unknown auth provider", http.StatusNotFound)
			return
		}
		if body.Code == "" {
			JSONError(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		var err error
		if uid, err = resolveOAuthUser(provider, body.Code, body.RedirectURI); err != nil {
			log.Printf("[oauth] exchange failed: %v", err)
			JSONError(w, "Sign-in failed", http.StatusUnauthorized)
			return
		}
	}

	// Set the sessionid cookie for the admin proxy to relay onto its own origin.
	// The admin re-reads the profile via /api/auth/status once the cookie lands,
	// so we only need to return the user id here.
	startSession(w, uid)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"userid": uid}); err != nil {
		log.Printf("[oauth] failed to encode exchange response: %v", err)
	}
}
