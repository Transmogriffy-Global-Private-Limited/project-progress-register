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

## Verification state

Formatting, module tidiness, `go vet`, all Go tests, OpenAPI validation and route coverage, full build, race-detector tests, PowerShell syntax, residue scanning, and live loopback smoke checks pass. The smoke check confirmed home `200`, liveness `200`, unavailable-database readiness `503`, actual `127.0.0.1` listeners, and API documentation `200` when enabled versus `404` when disabled.

No live PostgreSQL migration was applied because no explicitly disposable development/test database or credentials were identified. PostgreSQL is listening at local port 5432, but unauthenticated read-only connection attempts correctly failed. Database integration is implemented, unit-tested, and failure-path smoke-tested; its successful live migration/readiness path remains unverified. The foundation is therefore Implemented, not Verified.

## Not implemented

- Users, bootstrap Admin, authentication, sessions, CSRF, throttling, or password reset.
- Audit-event schema or viewer.
- Projects, membership, geofences, tasks, Markdown editing, progress updates, revisions, attachments, comments, suggestions, assessments, or dashboards.
- Deployment, Caddy configuration, production database roles, backup/restore automation, or attachment storage.

## Next slice

Implement trusted identity and the audit foundation as one vertical slice: first-Admin bootstrap, login/logout, opaque sessions, CSRF, throttling, session revocation, final-Admin invariant, authentication audit events, UI/API contracts, and security verification.
