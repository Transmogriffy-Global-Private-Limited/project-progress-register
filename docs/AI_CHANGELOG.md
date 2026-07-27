# AI changelog

## 2026-07-27 — Backend completion slice implemented

Status: Implemented and automated-verified; database-live/restore verification pending

- Added neutral authorized dashboard aggregates and complete filtered Admin audit access with bounded opaque keyset pagination while retaining the identity-only compatibility endpoint.
- Added migration `000007` and atomic before/after task revisions, then exposed a project-scoped oldest-first task timeline covering task/progress changes, attachment lifecycle/access, comments, accepted suggestions, and assessments with typed metadata and sanitized Markdown projections.
- Added guarded coordinated PostgreSQL/attachment backup and confirmed-empty-target restore scripts plus operator documentation. Neither script was executed.
- Kept the unresolved “needs progress update” policy out of backend semantics; the dashboard exposes factual counts and timestamps only.
- Added focused tests, route/OpenAPI contracts, and extended the zero-user live verifier. Focused packages, OpenAPI/route coverage, the full formatter/tidy/vet/test/build verifier, race detection, Bash syntax, and all three loopback docs/base-path smoke states pass. Database-live migration/lifecycle and empty-target restore verification were not run.

## 2026-07-27 — Review workflow implemented

Status: Implemented and automated-verified; database-live verification pending

- Added migration `000006` with immutable progress comments, one-to-one accepted suggestions, append-only versioned task assessments, indexes, constraints, and update/delete rejection triggers.
- Added scoped progress/project services and seven REST operations for comment list/create, idempotent Admin acceptance, task-level accepted suggestions, current assessment, Admin append, and Admin history. Markdown source remains authoritative and responses return sanitized HTML.
- Added complete OpenAPI/human contracts, service/route tests, the guarded zero-user `verify-live-review.ps1`, the review educational guide, and synchronized architecture, persistence, permission, state, and planning documents.
- Recorded the operator's implementation-first cadence: continue writing verification assets but do not execute tests until the backend feature set is complete.

## 2026-07-27 — Prefix-correct hosted response and Swagger contracts

Status: Verified locally; production rehost pending

- Moved attachment `content_path` construction to the HTTP presentation boundary so list, create, detail, and update responses include the configured `BASE_PATH` while storage and authorization remain in the progress service.
- Kept the committed OpenAPI file authoritative and added a deterministic served representation that changes only the `basePath` server-variable default. Hosted Swagger therefore selects `/backend` by default while root-hosted development remains `/`.
- Added focused Go regression coverage and a `/backend` loopback state to the existing smoke script. Focused contract tests, formatting, module tidiness, `go vet`, all Go tests, build, PowerShell syntax, and all three root/prefixed loopback smoke states pass. No rehost or database operation was performed.

## 2026-07-27 — Rehost now deploys current source changes

Status: Implemented; live deployment cycle intentionally not executed

- Extended `scripts/rehost-ppr.sh` from a restart-only wrapper into a guarded source deployment: validate the PPR checkout, run all Go tests, build a stripped static binary, atomically install it, restart through the shared handler, and wait for database readiness.
- Retain one prior production binary and automatically restore it when restart or readiness verification fails. Concurrent rehosts are rejected with a host lock.
- Kept production migrations, Caddy, and environment changes explicit. A build or test failure leaves the running service and binary unchanged; migrations required by new code must be reviewed and applied separately.

## 2026-07-27 — PPR rehost handler

Status: Implemented; live restart intentionally not executed

- Added tracked `scripts/rehost-ppr.sh`, installed as `/usr/local/bin/rehost-ppr-service`, and exposed the thin `rehost-ppr` function through `~/.bash_aliases`.
- The handler preserves the shared `rehost-service` delay/reload/log options and waits up to 30 seconds for `/backend/api/v1/health/ready` before reporting success.
- Added production usage and behavior documentation. Static shell parsing, installed-source identity, alias discovery, and help output pass. The active production service was not restarted because this request authorized handler installation, not a live rehost cycle.

## 2026-07-27 — Production docs enabled and first Admin bootstrapped

Status: Verified

- Added `BASE_PATH=/backend` to the ignored repository `.env` and kept `API_DOCS_ENABLED=true` there.
- Enabled API documentation in the protected production environment, restarted only `ppr.service`, and verified public setup, Swagger, raw OpenAPI, and readiness routes return `200`.
- The first supplied credential had 11 characters and was rejected by the documented 12-character minimum with `422`; no user was created by that attempt, so bootstrap remained available for a corrected retry.
- Retried with the operator's policy-valid replacement credential. Bootstrap returned `201`, the enabled Admin row was verified, and login, session recovery, and logout each returned `200`.
- Removed `BOOTSTRAP_TOKEN`, restarted only `ppr.service`, and verified setup now returns `404` while public Swagger, raw OpenAPI, and readiness remain `200`.
- No credential or generated secret was written to tracked repository content.

## 2026-07-27 — Production hosting at `/backend`

Status: Verified

Changed:

- Added validated `BASE_PATH` configuration and mounted the complete HTTP surface beneath it, including redirects, compatibility pages, assets, scoped session cookies, OpenAPI server selection, and embedded Swagger addressing.
- Added prefix-focused configuration/HTTP tests, a reusable hardened systemd unit, a Caddy site template, a production operations guide, and hosting plan 0007.
- Built a static Linux binary and installed it outside Git at `/opt/ppr/bin/ppr`; installed/enabled `ppr.service` as the unprivileged `ppr` user with private `/var/lib/ppr/attachments`.
- Corrected the database source from `.env.example` to the operator-created ignored `.env`. Removed the mistakenly created empty `project_progress_register` database and its unused role before service activation.
- Backed up the selected `pprdb`, verified the dump archive, dropped/recreated only that database, applied migrations `000001` through `000005`, and confirmed zero users and business rows.
- Generated protected production database, CSRF, and bootstrap secrets outside the repository. Added a non-superuser `ppr_runtime` login with application DML grants while retaining the `.env` PostgreSQL URL only for explicit migrations.
- Backed up, validated, and reloaded the active Caddyfile with `ppr.transev.site/backend/*` preserved to `127.0.0.1:18090`; root and unprefixed application routes remain unavailable.

Verification:

- Formatting, module tidiness, vet, all tests, race tests, static build, OpenAPI validation/route coverage, systemd-unit validation, Caddy-template validation, and `git diff --check` pass.
- Migration status reports five applied and zero pending. Production row counts are zero for users, projects, tasks, progress updates, and attachments.
- At initial activation, `ppr.service` was enabled and active; the application listened only on `127.0.0.1:18090`; prefixed liveness/readiness returned `200`; disabled docs and unprefixed routes returned `404`; setup returned `200` before bootstrap. The later operational follow-up above records the intentional docs enablement and closed setup route.
- The complete Caddyfile validates/reloads and HTTP hostname traffic redirects to HTTPS. The initial certificate attempt correctly failed while DNS was absent. After A and AAAA records were published, Caddy obtained a valid Let's Encrypt certificate and public IPv4/IPv6 liveness, readiness, setup, disabled-doc, root, and unprefixed-route checks all passed.

Compatibility and artifacts:

- Root hosting remains the default when `BASE_PATH` is empty. Production now requires `/backend` and scopes its cookie accordingly.
- Production environment, generated secrets, binary, database dump, attachment data, systemd/Caddy runtime files, and logs remain ignored or outside the repository. The hosting source and documentation were later committed and pushed; no runtime secrets or artifacts were included.

## 2026-07-27 — Progress updates and attachments plan approved

Status: Implemented; automated verification passes, database-live verification pending

Approved direction:

- Implement one larger backend slice for chronological progress updates, immutable revisions, image/document/video attachments, private recoverable filesystem storage, authorized downloads, audit, and frontend-ready contracts.
- Location presence gates file bytes, while geofence verification does not. Preserve valid browser coordinates for camera and existing-file uploads even when outside, inaccurate, or lacking a configured geofence; text-only updates may record unavailable/not-supplied location.
- Verification is attachment-specific: only an in-Chrome camera image with server-accepted location becomes verified. Existing images, documents, and videos remain explicitly non-verified while retaining their upload geotag.
- Use bounded streaming multipart uploads with up to ten 100 MiB files, strict common-format allowlists, opaque storage keys, SHA-256, pending-to-available recovery, and no public filesystem exposure.

Repository memory:

- Added detailed plan `docs/plans/0006-progress-updates-and-attachments.md` and advanced the living plan/current state to Step 06 before implementation.

Authored:

- Added migration `000005` for idempotent progress, immutable revisions, location/geofence snapshots, and attachment metadata/state.
- Added streaming private filesystem storage with type/size enforcement, opaque keys, SHA-256, atomic finalization, and recovery primitives.
- Added progress application/persistence flows for scoped chronological reads, authorization before file staging, author/Admin edits, policy-stable evidence, per-file verification, retry-safe pending recovery, downloads, and audit.
- Added nested multipart/list/detail/update/download HTTP routes, runtime reconciliation, OpenAPI schemas, focused tests, and `verify-live-progress.ps1`.
- Updated product, evidence, architecture, domain, permission, API, configuration, PostgreSQL, storage integration, operational, plan, state, ADR, and agent documentation.

Verification:

- Fixed the download content-disposition compile error found by focused testing and restricted plain-text detection to `.txt`, `.md`, and `.csv` extensions so unrelated filenames cannot enter through the text allowlist.
- Focused progress/filestore/HTTP tests, OpenAPI validation and route coverage, module tidiness, vet, all Go tests, build, race detection, PowerShell syntax, `git diff --check`, and both loopback API-documentation smoke modes pass.
- Migration application and `verify-live-progress.ps1` remain pending because they require an explicitly disposable, fully migrated database with zero users.

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
