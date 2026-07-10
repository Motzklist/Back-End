# API-Gateway

This repository holds the Go [Gin-Gonic](https://gin-gonic.com) API Gateway for the Motzklist project.

## Authentication

Sessions are cookie-based (`sessionid`, HttpOnly). Two ways to sign in:

- **Username / password:** `POST /api/login`, `POST /api/logout`, `GET /api/auth/status`.
- **Third-party (OAuth) — Google, Microsoft, …** (see [oauth_handlers.go](oauth_handlers.go)):
  - `GET /api/auth/{provider}/start?redirect_uri=<frontend/auth/callback>` — browser
    flow for the public Front-End (which calls this gateway directly). Redirects to
    the provider, then to `/api/auth/{provider}/callback`, which exchanges the code,
    provisions/looks up the user (by email), sets the session cookie, and redirects
    back to `redirect_uri`.
  - `POST /api/auth/{provider}/exchange` `{code, redirect_uri}` — server-to-server
    exchange for the Admin Front-End, whose Next.js server runs the browser redirect
    itself and needs the `sessionid` cookie issued on its own origin.

OAuth client credentials live **only** on the backend (`{PROVIDER}_CLIENT_ID` /
`{PROVIDER}_CLIENT_SECRET`); the frontends never see the secret. Set
`OAUTH_DEV_MODE=true` to exercise the whole redirect + session flow locally without
real credentials. See [.env.example](.env.example) for all variables.

TODO: 
- Fill up documentation.
- Fork, clone and setup local environment.
- Setup commit sign-off: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/managing-repository-settings/managing-the-commit-signoff-policy-for-your-repository
- Setup commit signature verification: https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification