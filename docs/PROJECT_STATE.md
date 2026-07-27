# Project state

As of 2026-07-27

## Implemented

- Go 1.25 modular-monolith foundation with one `ppr` binary.
- `serve`, `migrate up`, and `migrate status` commands.
- Environment validation with hard loopback and unprivileged-port enforcement.
- Low-footprint pgx PostgreSQL pool.
- Embedded forward-only migration discovery, SHA-256 checksums, advisory locking, transactional application, and `ppr_schema_migrations` ledger.
- Bounded readiness that requires PostgreSQL connectivity and current migration state.
- Server-rendered responsive home page and embedded CSS.
- `GET /api/v1/health/live` and `GET /api/v1/health/ready`.
- Specification-first OpenAPI 3.1, raw schema route, and embedded Swagger UI controlled by `API_DOCS_ENABLED`.
- Baseline HTTP security headers, structured request logs, panic containment, server timeouts, and graceful shutdown.
- Native PowerShell formatting, build, test, run, migration, OpenAPI, and full-verification scripts.
- Authoritative product, architecture, domain, permission, evidence, operational, planning, and decision documentation.
- One-time first-Admin bootstrap guarded by an optional environment secret and a PostgreSQL advisory lock.
- Local Admin/Member account schema and Argon2id password hashing with bounded concurrent work.
- Revocable PostgreSQL-backed opaque sessions, host-only `HttpOnly`/`SameSite=Lax` cookies, production `Secure`, and session-bound HMAC CSRF tokens.
- Durable identifier/IP login throttling, generic credential failures, and a dummy password verifier for unknown accounts.
- Append-only authentication audit events correlated by random request ID and trusted client IP.
- Server-rendered setup, login, authenticated home, and logout flows.
- JSON bootstrap, login, session recovery, and logout endpoints with complete OpenAPI/Swagger coverage.

## Verification state

Formatting, module tidiness, `go vet`, all Go tests, identity policy tests, HTTP cookie/CSRF/generic-error tests, OpenAPI validation and route coverage, full build, race-detector tests, PowerShell syntax, and live loopback smoke checks pass. The smoke check confirmed the redirected login page `200`, liveness `200`, unavailable-database readiness `503`, actual `127.0.0.1` listeners, and API documentation `200` when enabled versus `404` when disabled.

The human explicitly approved the initially empty remote `pprdb` PostgreSQL 18.4 database for disposable production-target verification. Migration `000001` applied with one ledger entry and zero pending migrations. Live verification passed readiness, concurrent one-winner bootstrap, generic invalid/throttled authentication, successful login and session recovery, CSRF rejection, logout revocation, durable counts/actions, password/session-hash format, complete audit context, and audit update/delete rejection. Foundation and trusted identity are Verified.

The live verifier left one temporary Admin, one revoked session, one throttle row, and 11 authentication audit events in `pprdb`. The human stated that this production database will be reset before real use. `.env.local` still needs a persistent valid `SESSION_CSRF_KEY` and real one-time `BOOTSTRAP_TOKEN`; the verifier intentionally used ephemeral in-memory values.

## Not implemented

- Full cross-domain audit viewer; the account slice authors a bounded identity-only viewer.
- Progress updates, revisions, attachments, comments, suggestions, assessments, or dashboard queries.
- Deployment, Caddy configuration, production database roles, backup/restore automation, or attachment storage.

## Implemented; database-live verification pending

The backend contains the complete account-administration slice: migration `000002`, Admin list/create/update/reset APIs, generated one-time credentials, forced password replacement, session revocation, optimistic versions, final-enabled-Admin locking, audit actions/query API, OpenAPI, tests, and live verification script.

It also contains the backend project-access slice: migration `000003`, project create/list/detail/update, temporal Member access, immediate scoped-query revocation, versioned current geofence policy, optimistic conflicts, transactional audit, OpenAPI, tests, and `verify-live-project-access.ps1`.

The task-register slice adds migration `000004`, project-scoped task list/create/detail/update, immutable creator ownership, optional responsible Member/date, active-project enforcement, optimistic versions, Goldmark plus Bluemonday sanitized HTML projections, transactional audit, OpenAPI, tests, ADR 0009, the safe-Markdown guide, and `verify-live-task-register.ps1`.

Formatting, module tidiness, vet, all Go tests, focused sanitizer/domain/HTTP coverage, OpenAPI validation and route coverage, build, race detection, `git diff --check`, and both loopback API-documentation toggle smoke modes pass for Steps 03–05. The smoke checks served only on `127.0.0.1` and used an intentionally unavailable database.

Migrations `000002` through `000004` have not been applied to the configured database, and the guarded account/project/task lifecycle scripts have not been run. They require an explicitly disposable, fully migrated database with zero users; the retained identity-verification database is not eligible. No database was modified during this verification run.

## Next slice

Publish the reviewed Steps 03–05 changes through `anubhab-work` and merge them into `main`. Run database-live lifecycle verification later on an explicitly disposable zero-user database. Verified progress updates and attachments remain the next approved larger backend slice.
