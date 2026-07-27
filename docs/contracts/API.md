# HTTP API contract

The authoritative machine-readable source is `../../api/openapi/v1/openapi.yaml`. This document owns causal semantics that are awkward to express in schema. It describes only implemented behavior.

## Versioning and representation

Programmatic application routes use `/api/v1` and JSON UTF-8. Health, bootstrap, and login are unauthenticated; session recovery and logout use the `ppr_session` cookie. JSON request bodies require `Content-Type: application/json`, reject unknown fields, and are limited to 64 KiB.

## Identity trust and error model

Successful login sets a host-only `ppr_session` cookie with path `/`, `HttpOnly`, `SameSite=Lax`, absolute expiry, and `Secure` in production. The cookie contains 32 random bytes encoded for URLs; PostgreSQL stores only its SHA-256 digest. There is no JWT, bearer token, local-storage token, or public registration route.

Every current-session lookup checks the token digest, expiry, revocation, and enabled user state. Login and session recovery return a session-bound `csrf_token`. An authenticated JSON write sends it in `X-CSRF-Token`; HTML forms send `_csrf`. Logout without a current session is idempotently successful.

Errors use `{"error":{"code":"...","message":"..."}}`. Unknown users, disabled users, incorrect passwords, and throttled attempts all return `401 invalid_credentials`; clients must not infer the hidden cause. Server errors use a generic message and keep details in secret-free structured logs.

## `POST /api/v1/setup/bootstrap`

Creates the first Admin only when `BOOTSTRAP_TOKEN` is configured and no user exists. The body contains `bootstrap_token`, normalized `username`, `email`, and `password`. The service validates the guarded secret, username/email, and password, hashes the password with Argon2id, then atomically inserts the Admin and `identity.bootstrap_succeeded` audit row under a transaction-level advisory lock. Concurrent calls can create at most one first user.

Success is `201` with `{"user": ...}`. Validation is `422`; wrong setup secret is `403`; absent configuration or an existing user is `404`. The bootstrap secret and password are never logged, stored, or returned.

## `POST /api/v1/auth/login`

Accepts `identifier` and `password`. The normalized identifier and trusted client IP address one durable throttle bucket. Five failures in a 15-minute window block the pair for 15 minutes. Unknown-user password work uses a dummy Argon2id verifier to reduce timing disclosure.

On success, one transaction creates a new hashed session, updates `last_login_at`, clears the throttle bucket, and appends `auth.login_succeeded`. The response is `200` with user, `csrf_token`, and `expires_at`, plus the cookie. Failure is generic `401 invalid_credentials`; the system records `auth.login_failed` or `auth.login_throttled` without plaintext identifiers or secrets.

## `GET /api/v1/auth/session`

Authenticates the cookie against current PostgreSQL state and returns user, a re-derived CSRF token, and expiry. This is the authoritative browser recovery path after reload or process restart. Missing, expired, revoked, or disabled-user sessions return `401 unauthenticated`.

## `POST /api/v1/auth/logout`

With a valid session, requires `X-CSRF-Token`, then atomically revokes the session and appends `auth.logout_succeeded`; the cookie is expired. Missing/invalid CSRF is `403 csrf_invalid`. Without a valid session, it still expires the cookie and returns `200 {"logged_out":true}` so retry is safe.

## `GET /api/v1/health/live`

Purpose: prove that the Go process and HTTP transport can respond.

Flow:

```text
HTTP request -> method check -> static process response
```

It does not query PostgreSQL. Success is `200` with `{"status":"ok"}`. Other methods return `405` and `Allow: GET`.

## `GET /api/v1/health/ready`

Purpose: determine whether the instance may receive stateful application traffic.

Flow:

```text
HTTP request
-> method check
-> bounded PostgreSQL ping
-> migration ledger existence/checksum/current-state verification
-> 200 ready or 503 not_ready
```

Success is `200` with `{"status":"ready"}`. Any dependency or schema failure is `503` with `{"status":"not_ready"}`. Failures are safe to retry. Other methods return `405` and `Allow: GET`.

## API documentation routes

When `API_DOCS_ENABLED=true`:

- `GET /api/openapi/v1/openapi.yaml` serves the exact embedded authoritative schema.
- `GET /api/docs/` serves an embedded Swagger UI configured to read that route.
- `GET /api/docs` redirects to the canonical trailing-slash viewer route.

When false, none of these routes are registered and they return `404`. The toggle does not affect ordinary application or health routes.

## Common transport behavior

Responses receive MIME-sniffing, frame, referrer, and content-security headers. Each request receives a random `X-Request-ID`; logs and audits correlate on it. Client IP comes from the socket unless the direct peer is loopback, in which case the first valid `X-Forwarded-For` address is trusted for Caddy. Unexpected handler panics are contained and mapped to `500`; details remain in structured logs. Request logs contain request ID, method, path, status, and duration but no request body or credentials.
