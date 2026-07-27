# AI changelog

## 2026-07-27 — Task register and safe Markdown slice

Status: Implemented; automated verification passes, database-live lifecycle verification pending

Approved scope:

- One larger backend-only slice for project-scoped tasks, immutable creator ownership, optional responsibility/date, safe Markdown, optimistic updates, transactional audit, REST/OpenAPI, tests, scripts, and documentation.

Authored:

- Added migration `000004` for creator-owned tasks, Markdown source, optional responsible user/date, versions, constraints, and indexes.
- Extended the projects aggregate with active-project/current-membership command locking, Admin-or-creator edit policy, responsible-Member validation, scoped reads, conflicts, and task audit.
- Added Goldmark and Bluemonday as focused Go dependencies; source Markdown remains durable truth and allowlist-sanitized HTML is derived for project/task responses.
- Added task list/create/detail/update routes, OpenAPI schemas/semantics, route coverage, domain/HTTP/sanitizer tests, and `verify-live-task-register.ps1` without execution.
- Added the safe-Markdown guide and ADR 0009, and synchronized architecture, domain, permissions, API, PostgreSQL, state, plan, operations, and repository instructions.
- Completed a read-only affected-surface and stale-wording scan; no remaining task-is-planned claims were found in current authoritative documentation.

Verification:

- Dependency integrity, formatting, vet, all Go tests, focused sanitizer/domain/HTTP coverage, OpenAPI validation/route coverage, build, race detection, `git diff --check`, and both loopback documentation-toggle smoke modes pass.
- Migrations and guarded database-live lifecycle scripts were not executed because the configured database is not the required disposable zero-user target.

## 2026-07-27 — Project access and geofence policy slice

Status: Implemented; automated verification passes, database-live lifecycle verification pending

Approved scope:

- One larger backend-only slice for project lifecycle, temporal Member access, versioned geofence policy, scoped queries, Admin commands, transactional audit, JSON/OpenAPI, tests, scripts, and documentation.
- Continue authoring verification coverage without executing tests, builds, migrations, contract validation, smoke checks, race checks, or live scripts.

Authored:

- Added migration `000003` with projects, historical membership rows, immutable geofence versions, database range constraints, and partial unique indexes for current state.
- Added the `projects` module with application validation, Admin policy, Member-scoped reads, optimistic project/geofence conflicts, membership eligibility, transactional state changes, and audit.
- Added project list/create/detail/update, membership list/add/remove, and geofence replacement APIs with authentication, CSRF, strict paths/payloads, error mapping, and complete OpenAPI schemas.
- Added domain and HTTP test coverage plus `scripts/verify-live-project-access.ps1` without executing them.
- Updated the architecture, domain, permissions, API, PostgreSQL, local-development, planning, state, and repository-agent documentation.

Verification:

- Vet, all Go tests, OpenAPI validation/route coverage, build, race detection, `git diff --check`, and loopback smoke checks pass across the combined worktree. The guarded project database-live verifier remains pending.

## 2026-07-27 — Account administration slice

Status: Implemented; automated verification passes, database-live lifecycle verification pending

Approved scope:

- One larger backend vertical slice for Admin user inventory, creation, enable/disable, roles, password reset, forced temporary-password replacement, session revocation, final-enabled-Admin safety, audit query, JSON/OpenAPI, tests, scripts, and documentation.
- Continue writing verification assets but do not execute tests, builds, migrations, smoke checks, or live verification until the human resumes testing.
- Keep changes local and uncommitted because repository commit policy requires completed verification.

Non-goals:

- Email, external identity providers, MFA, username/email rename, project permissions, deployment, or database reset.

Authored:

- Added migration `000002` for `must_change_password`; the existing enabled/role index supports the Admin guard query.
- Added Admin-authorized list/create/update/reset services, generated temporary passwords, self-service password replacement, optimistic version checks, atomic session revocation, final-enabled-Admin locking, and new audit actions.
- Added backend user inventory, lifecycle, one-time credential, forced-password, and recent identity-audit JSON routes.
- Expanded the User contract and OpenAPI with Admin operations, forced-change state, one-time credential semantics, conflicts, CSRF, and side effects.
- Added service/HTTP tests and `scripts/verify-live-account-admin.ps1` without executing them.

Verification:

- Formatting, vet, all Go tests, OpenAPI validation/route coverage, build, race detection, `git diff --check`, and loopback smoke checks pass across the combined worktree. The guarded account database-live verifier remains pending.

Scope correction:

- The human clarified that this repository is backend-only. Removed the newly authored account-management HTML routes/templates/styles, retained only APIs/contracts, superseded ADR 0005 with ADR 0008 for future work, and updated repository invariants and plans.

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
