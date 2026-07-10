package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// oauthMux builds a ServeMux with the OAuth routes so r.PathValue("provider")
// is populated the same way it is in production (main.go).
func oauthMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/{provider}/start", oauthStartHandler)
	mux.HandleFunc("GET /api/auth/{provider}/callback", oauthCallbackHandler)
	mux.HandleFunc("POST /api/auth/{provider}/exchange", oauthExchangeHandler)
	return mux
}

// stubOAuthUser overrides user provisioning so tests never touch the database,
// restoring the real implementation on cleanup.
func stubOAuthUser(t *testing.T, uid string) {
	t.Helper()
	prev := oauthFindOrCreateUser
	oauthFindOrCreateUser = func(email, name string) (string, error) { return uid, nil }
	t.Cleanup(func() { oauthFindOrCreateUser = prev })
}

// ==========================================
// State store
// ==========================================

func TestOAuthState_RoundTripSingleUse(t *testing.T) {
	state := newOAuthState("google", "http://localhost:3000/auth/callback")

	flow, ok := consumeOAuthState(state)
	if !ok {
		t.Fatal("expected freshly created state to be valid")
	}
	if flow.provider != "google" || flow.redirectURI != "http://localhost:3000/auth/callback" {
		t.Errorf("state did not round-trip: %+v", flow)
	}
	if _, ok := consumeOAuthState(state); ok {
		t.Error("state should be single-use (second consume must fail)")
	}
}

func TestOAuthState_UnknownRejected(t *testing.T) {
	if _, ok := consumeOAuthState("never-issued"); ok {
		t.Error("unknown state must be rejected")
	}
}

// ==========================================
// resolveFrontendRedirect (open-redirect guard)
// ==========================================

func TestResolveFrontendRedirect(t *testing.T) {
	t.Setenv("CLIENT_ORIGIN", "http://localhost:3000")

	if got := resolveFrontendRedirect("http://localhost:3000/auth/callback"); got != "http://localhost:3000/auth/callback" {
		t.Errorf("allowed origin should pass through, got %q", got)
	}
	// Disallowed origin -> fallback to the configured client origin.
	if got := resolveFrontendRedirect("https://evil.example.com/steal"); got != "http://localhost:3000/auth/callback" {
		t.Errorf("disallowed origin should fall back, got %q", got)
	}
	// Empty -> fallback.
	if got := resolveFrontendRedirect(""); got != "http://localhost:3000/auth/callback" {
		t.Errorf("empty redirect should fall back, got %q", got)
	}
}

// ==========================================
// Start handler
// ==========================================

func TestOAuthStart_RealRedirect(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "")
	t.Setenv("CLIENT_ORIGIN", "http://localhost:3000")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	t.Setenv("PUBLIC_API_URL", "http://localhost:8080")

	req := httptest.NewRequest("GET", "/api/auth/google/start?redirect_uri=http://localhost:3000/auth/callback", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://accounts.google.com/o/oauth2/v2/auth") {
		t.Errorf("should redirect to Google authorize endpoint, got %q", loc)
	}
	for _, want := range []string{"client_id=test-client-id", "state=", "response_type=code", "redirect_uri="} {
		if !strings.Contains(loc, want) {
			t.Errorf("authorize URL missing %q: %s", want, loc)
		}
	}
	if !strings.Contains(loc, "localhost%3A8080%2Fapi%2Fauth%2Fgoogle%2Fcallback") {
		t.Errorf("redirect_uri should point at our backend callback: %s", loc)
	}
}

func TestOAuthStart_UnknownProvider(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "")
	req := httptest.NewRequest("GET", "/api/auth/wat/start", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown provider should be 404, got %d", rr.Code)
	}
}

func TestOAuthStart_NotConfigured(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	req := httptest.NewRequest("GET", "/api/auth/google/start", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured provider should be 503, got %d", rr.Code)
	}
}

func TestOAuthStart_DevMode(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "true")
	t.Setenv("CLIENT_ORIGIN", "http://localhost:3000")
	stubOAuthUser(t, "42")

	req := httptest.NewRequest("GET", "/api/auth/google/start?redirect_uri=http://localhost:3000/auth/callback", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("dev-mode start should redirect (302), got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "http://localhost:3000/auth/callback" {
		t.Errorf("dev-mode should redirect straight back to the frontend, got %q", loc)
	}
	var sid string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "sessionid" {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("dev-mode start should set a sessionid cookie")
	}
	if got := sessions[sid]; got != "42" {
		t.Errorf("session should map to stubbed uid 42, got %q", got)
	}
	delete(sessions, sid)
}

// ==========================================
// Callback handler
// ==========================================

func TestOAuthCallback_BadState(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/auth/google/callback?state=forged&code=abc", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("forged state should be 400, got %d", rr.Code)
	}
}

func TestOAuthCallback_ProviderErrorRedirects(t *testing.T) {
	t.Setenv("CLIENT_ORIGIN", "http://localhost:3000")
	state := newOAuthState("google", "http://localhost:3000/auth/callback")

	req := httptest.NewRequest("GET", "/api/auth/google/callback?state="+state+"&error=access_denied", nil)
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("provider error should redirect back to frontend (302), got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "error=access_denied") {
		t.Errorf("frontend redirect should carry the error, got %q", loc)
	}
}

// ==========================================
// Server-to-server exchange (Admin Front-End)
// ==========================================

func TestOAuthExchange_DevMode(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "true")
	stubOAuthUser(t, "7")

	req := httptest.NewRequest("POST", "/api/auth/google/exchange", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev-mode exchange should be 200, got %d", rr.Code)
	}
	var sid string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "sessionid" {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("exchange should set a sessionid cookie to relay")
	}
	if got := sessions[sid]; got != "7" {
		t.Errorf("session should map to stubbed uid 7, got %q", got)
	}
	delete(sessions, sid)
}

func TestOAuthExchange_MissingCode(t *testing.T) {
	t.Setenv("OAUTH_DEV_MODE", "")
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")

	req := httptest.NewRequest("POST", "/api/auth/google/exchange", strings.NewReader(`{"redirect_uri":"http://localhost:3001/api/auth/google/callback"}`))
	rr := httptest.NewRecorder()
	oauthMux().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("missing code should be 400, got %d", rr.Code)
	}
}
