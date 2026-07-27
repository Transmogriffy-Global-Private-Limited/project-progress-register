# Plan 0002 — Trusted identity and audit foundation

Status: Implemented; live PostgreSQL success-path verification pending

## Objective

Establish the smallest complete authentication boundary for the internal application: securely create the first Admin once, authenticate enabled users, maintain revocable sessions, protect authenticated writes from CSRF, throttle password guessing durably, and append authentication audit events.

## Surface map

- Migration: `users`, `sessions`, `login_throttles`, `audit_events`, indexes, constraints, and append-only enforcement
- Identity domain: normalization, validation, roles, password hashing, opaque tokens, session-bound CSRF, and typed outcomes
- PostgreSQL store: atomic bootstrap/audit, login session/audit, failure/audit, throttle, session lookup, and logout/audit operations
- HTTP: request identity context, trusted client IP handling behind loopback Caddy, cookies, CSRF, HTML forms, JSON endpoints, and error mapping
- UI: login, one-time setup, authenticated home, and logout
- Contract: OpenAPI security scheme, authentication operations, schemas, examples, statuses, and human-readable semantics
- Verification: unit, transport, contract, migration-loader, race, loopback, and full-suite checks
- Documentation: architecture, domain, permissions, configuration, operations, project state, changelog, plan, and ADR reconciliation

## Security and policy decisions

- Passwords use Argon2id with 19 MiB memory, 2 iterations, 1 lane, 16-byte random salt, and 32-byte key. Passwords allow all characters, require at least 12 Unicode characters, and allow at most 128 UTF-8 bytes.
- Session tokens contain 32 random bytes. Only SHA-256 token hashes are stored. Tokens rotate on every successful login.
- The `ppr_session` cookie is host-only, `HttpOnly`, `SameSite=Lax`, path `/`, and `Secure` in production. Default session lifetime is 12 hours.
- CSRF tokens are HMAC-SHA-256 values derived from the raw session token with a separate configured 32-byte key. They are never stored and rotate with the session.
- Authenticated `POST`, `PUT`, `PATCH`, and `DELETE` operations require the session-bound token. HTML uses `_csrf`; JSON uses `X-CSRF-Token`.
- Login and bootstrap HTML posts require same-origin browser metadata. JSON login requires `application/json`, which cannot be submitted cross-origin as a simple form without a successful CORS preflight; the application exposes no CORS policy.
- Login identifiers are normalized and hashed for throttle storage. Five failures within 15 minutes block that identifier/IP pair for 15 minutes. Successful login clears the bucket.
- Unknown, disabled, and wrong-password accounts return the same `invalid_credentials` result. A dummy Argon2id comparison reduces username timing differences.
- The first Admin setup is available only when no user exists and `BOOTSTRAP_TOKEN` is configured. The secret is at least 24 characters, is never stored or logged, and is compared in constant time.
- Audit rows are append-only. Database triggers reject update and delete. Application transitions insert their state change and audit event in the same transaction where applicable.

## Routes and contracts

HTML:

- `GET /login`
- `POST /login`
- `GET /setup`
- `POST /setup`
- `POST /logout`
- `GET /` — authenticated application home

JSON API:

- `POST /api/v1/setup/bootstrap`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/session`
- `POST /api/v1/auth/logout`

Health and optional API-documentation routes remain backward compatible.

## Audit actions

- `identity.bootstrap_succeeded`
- `identity.bootstrap_failed`
- `auth.login_succeeded`
- `auth.login_failed`
- `auth.login_throttled`
- `auth.logout_succeeded`

Records include actor when known, target when safe, outcome, server timestamp, request ID, client IP, user agent, and bounded structured details without passwords, bootstrap secrets, session tokens, CSRF tokens, or request bodies.

## Acceptance criteria

- No registration route exists.
- Concurrent bootstrap attempts can create at most one first Admin.
- Password hashes and salts are adaptive and unique; plaintext never reaches persistence or logs.
- Successful login creates a new revocable opaque session and audit event atomically.
- Failed, disabled, unknown, and throttled attempts have generic client behavior and useful secret-free audit records.
- Session authentication checks token hash, expiry, revocation, and current enabled user state on every request.
- Logout revokes the current session, records audit, clears the cookie, and is idempotent from the user’s perspective.
- Authenticated writes without a correct session-bound CSRF token fail before application mutation.
- The home page requires authentication; health, login, setup availability, schema, and docs behavior remain intentional.
- OpenAPI and human documentation match every JSON endpoint and error response.
- Migration and application behavior are tested without modifying an unidentified database.

## Verification

1. Formatter and PowerShell syntax.
2. Password, token, normalization, CSRF, and validation unit tests.
3. Service outcome, dummy-check, throttle, session, and audit orchestration tests with a deterministic fake store.
4. HTTP cookie, redirect, CSRF, content-type, status, non-enumeration, docs-toggle, and compatibility tests.
5. OpenAPI validation and route coverage.
6. Migration discovery/checksum tests and SQL residue review.
7. Full `go vet`, `go test ./...`, build, race detector, loopback smoke, residue scan, and Git checks.
8. Live PostgreSQL success-path verification only against an explicitly approved safe database.

## Recovery and limitations

- Sessions, throttles, users, and audit rows survive restart in PostgreSQL.
- A database outage makes identity-dependent requests fail closed and readiness fail; liveness remains available.
- Expired/revoked sessions remain as bounded audit-supporting metadata until later retention policy is approved.
- MFA, password reset, breached-password checking, and general user management are later slices.

## Result

The migration, shared identity service, PostgreSQL transitions, HTML setup/login/logout pages, authenticated home, JSON API, session cookie, CSRF enforcement, generic failures, request correlation, OpenAPI contract, and focused tests are implemented. Formatting, vet, all unit/HTTP/contract tests, build, race detection, PowerShell verification, and loopback smoke tests pass.

No identified database was modified. Applying migration `000001` and verifying successful bootstrap/login/logout/readiness against a live PostgreSQL database remains pending until an explicitly safe database and credentials are provided.
