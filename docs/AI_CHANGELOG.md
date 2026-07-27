# AI changelog

## 2026-07-27 — Trusted identity slice

Status: Verified

Approved scope:

- One-time first-Admin bootstrap guarded by an environment secret.
- Argon2id password hashing, generic authentication failures, and durable login throttling.
- Opaque PostgreSQL-backed sessions in secure HTTP-only same-site cookies.
- Session-bound CSRF protection for authenticated writes.
- Login, logout, bootstrap, and relevant failure audit events in an append-only table.
- Server-rendered and versioned JSON transports backed by the same application service.

Non-goals:

- Public registration, JWT, Redis, email delivery, password reset, general account administration, project permissions, or deployment.

Changed:

- Added `users`, `sessions`, `login_throttles`, and append-only `audit_events` migration with transactional identity/audit operations.
- Added Argon2id passwords, guarded first-Admin bootstrap, generic login failures, durable throttling, opaque session tokens, session-bound HMAC CSRF, current-session recovery, and logout revocation.
- Added server-rendered setup/login/authenticated-home/logout flows and matching versioned JSON endpoints.
- Added random request correlation, loopback-proxy-aware client IP handling, secure cookie policy, authoritative OpenAPI operations/schemas, and synchronized security/operational documentation.
- Added password/token/service tests and HTTP cookie, CSRF, error, method, docs-toggle, and contract coverage.

Verification:

- `gofmt`, `go mod tidy`, `go vet`, `go test ./...`, and complete build — passed through `scripts/verify-all.ps1`.
- `go test -race ./...` — passed.
- `scripts/smoke-foundation.ps1` — loopback listener, login-page redirect, liveness, unavailable-database readiness, and docs enabled/disabled states passed.
- OpenAPI parse, validation, and registered-route coverage — passed.

Remaining before Verified:

- None for the identity behavior. Production data reset and permanent secret configuration remain operational prerequisites, not missing implementation.

Compatibility and migration impact:

- Adds the first business migration. It is forward-only and creates new identity tables/functions/triggers; it does not alter or delete prior business data.
- `ppr serve` now requires `SESSION_CSRF_KEY`; migration commands do not. Existing health and optional API-documentation routes remain compatible.

Live verification follow-up:

- Applied migration `000001` to the human-approved, initially empty remote `pprdb` PostgreSQL 18.4 target; status became one applied and zero pending.
- The first live bootstrap request exposed that the documented `bootstrap_token` field was rejected by the strict decoder. Added canonical JSON tags plus a transport regression test, reran full verification, and then reran live verification successfully.
- Added `scripts/verify-live-identity.ps1`, which uses ephemeral in-memory security values and requires zero pre-existing users.
- Verified concurrent bootstrap produced one `201` and one `404`; readiness, login, session recovery, CSRF rejection, throttling, logout, and post-logout behavior matched contracts.
- Verified one Argon2id password hash, one 32-byte stored session hash, expected audit action counts, object-only audit details, complete request correlation, and trigger rejection of audit updates/deletes.
- The approved verifier left temporary identity data in the production database; the human stated that the database will be reset before real use.

## 2026-07-27 — Foundation slice

Status: Verified

Changed:

- Established repository-specific agent instructions and authoritative documentation navigation.
- Added the approved product, architecture, domain, permission, evidence/trust, configuration, development-plan, project-state, detailed-plan, and ADR documentation.
- Implemented the Go modular-monolith foundation, strict loopback configuration, server-rendered shell, health/readiness, PostgreSQL migration framework, OpenAPI source, embedded Swagger UI toggle, and PowerShell workflow.
- Recorded the decision to defer blockchain while preserving ordinary integrity hashes and a future external-checkpoint path.

Why:

- Provide a small, explicit, verifiable base before identity and business-domain work.
- Keep documentation, contracts, runtime behavior, and verification synchronized from the first slice.

Verification:

- `gofmt -w ./cmd ./internal ./api`
- `go mod tidy`
- `go test ./...` — passed
- `.\scripts\validate-openapi.ps1` — passed
- `.\scripts\build.ps1` — passed
- `.\scripts\verify-all.ps1` — PowerShell syntax, format, module tidiness, vet, tests, and build passed
- `.\scripts\smoke-foundation.ps1` — passed in docs-enabled and docs-disabled states on `127.0.0.1`
- `go test -race ./...` — passed
- Residue scan and `git diff --check` — passed before final staging

Remaining before Verified:

- None. The later trusted-identity verification applied the first migration and proved `200 ready` against the explicitly approved PostgreSQL 18.4 target.

Compatibility and migration impact:

- New repository; no prior application compatibility surface.
- The migration command creates only `ppr_schema_migrations` in this slice; there are no business migrations yet.
