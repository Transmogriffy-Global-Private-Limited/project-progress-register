# Development plan

Status: Approved product direction; production hosting verified over public IPv4 and IPv6 HTTPS

## Project objective

Deliver a low-memory internal web register that makes day-to-day project progress as simple as a paper diary while enforcing authorization, durable history, explicit evidence trust, safe attachments, and complete auditing.

## System boundaries and direction

- Backend-only Go modular monolith and PostgreSQL with authoritative JSON/OpenAPI contracts. Product frontend work is outside this repository.
- One organization and no v1 multi-tenancy.
- Caddy terminates production TLS; the Go listener remains loopback-only.
- Local filesystem attachments are private and replaceable behind an application boundary.
- PostgreSQL is durable truth; browser evidence is operational rather than cryptographic.
- Specification-first OpenAPI covers every implemented JSON endpoint.
- No Redis, event broker, microservice, SPA, container, public listener, blockchain, or production Node.js runtime in v1.

## Development phases

1. **Foundation** — repository memory, runtime, configuration, PostgreSQL/migrations, health, OpenAPI, scripts, and verification.
2. **Trusted identity** — initial Admin bootstrap, authentication, sessions, CSRF, throttling, and audit foundation.
3. **Account administration** — backend user lifecycle APIs, roles, reset flow, and identity audit query.
4. **Project access** — projects, membership, Markdown description, and geofences.
5. **Task register** — tasks, ownership, responsible member, target date, Markdown, and concurrency.
6. **Verified progress** — chronological updates, camera capture, location verification, attachments, revisions, and recovery.
7. **Review** — comments, accepted suggestions, and Admin assessments.
8. **Reporting and operational hardening** — dashboard query APIs, full audit query, backup/restore, and deployment documentation.

Each phase depends on the security and state boundaries before it and must complete backend application, persistence, authorization, API contract, recovery, verification assets, and documentation together.

## Feature registry

### Feature: Application foundation

Status: Verified

Phase: 1

Objective: Provide the smallest runnable, documented, contract-checked Go/PostgreSQL base on which complete vertical features can be built safely.

Scope: repository instructions, project memory, one binary, loopback config, home page, health/readiness, migration ledger, OpenAPI/Swagger toggle, PowerShell scripts, and tests.

Non-goals: domain tables, authentication, business workflows, uploads, deployment, or speculative module scaffolding.

Acceptance and verification are detailed in `plans/0001-foundation.md`.

### Feature: Trusted identity and audit foundation

Status: Verified

Phase: 2

Depends on: application foundation and its pending live PostgreSQL success-path verification.

Objective: Bootstrap the first Admin once, authenticate enabled users securely, manage opaque database sessions and CSRF, throttle login, and append required authentication audit events.

Acceptance criteria include no public registration, secure hashes/tokens/cookies, session revocation, backend enforcement, complete contracts, and focused security tests. The final-enabled-Admin mutation guard belongs to account administration because this slice exposes no role/disable command.

Detailed plan: `plans/0002-trusted-identity.md`.

### Feature: Backend account administration

Status: Verified

Phase: 3

Depends on: trusted identity.

Objective: Expose secure APIs for Admins to list, create, disable, re-enable, reset, inspect, and role-change users while users can replace temporary credentials safely.

Scope: one larger backend vertical slice covering persistence, final-enabled-Admin concurrency safety, generated temporary credentials returned once, forced password change, session revocation, audit query, JSON/OpenAPI, tests, verification scripts, and authoritative documentation.

Detailed plan: `plans/0003-account-administration.md`.

### Feature: Projects, membership, and geofences

Status: Implemented and verified; OpenAPI corrections deployed

Phase: 4

Depends on: identity and account administration.

Objective: Create project-scoped access and immutable evidence-policy history without exposing projects through guessed identifiers.

Scope: one larger backend vertical slice covering project lifecycle, temporal membership, versioned geofence policy, scoped queries, Admin commands, audit, JSON/OpenAPI, tests, scripts, and authoritative documentation.

Detailed plan: `plans/0004-project-access.md`.

### Feature: Tasks and safe Markdown

Status: Implemented

Phase: 5

Depends on: project access.

Objective: Provide a simple task register with immutable creator ownership, optional responsibility/target date, safe Markdown, and concurrency protection.

Scope: one larger backend vertical slice covering task lifecycle, project scope, creator ownership, responsible Member/date, derived sanitized HTML, optimistic conflicts, audit, JSON/OpenAPI, tests, scripts, and documentation.

Detailed plan: `plans/0005-task-register.md`.

### Feature: Multiple editable task responsibilities

Status: Verified locally; not deployed

Phase: 5

Depends on: tasks, project membership, and immutable task revisions.

Objective: Let authorized task editors atomically assign zero or more current project Members without changing task ownership or access policy.

Scope: Migrate existing singular assignments into an authoritative join table; add plural V2 task contracts while retaining safe singular V1 compatibility; preserve optimistic versions, complete before/after timeline reconstruction, membership-removal cleanup, audit, tests, guarded scripts, OpenAPI, and frontend documentation.

Detailed plan: `plans/0010-multiple-task-responsibilities.md`.

### Feature: Verified progress updates and attachments

Status: Implemented

Phase: 6

Depends on: tasks and geofences.

Objective: Deliver the central mobile camera/location submission, immutable revision history, recoverable file storage, metadata trust labels, and chronological task register.

Scope: one larger backend slice covering non-blocking location evidence, immutable progress revisions, mixed image/document/video attachments, private recoverable storage, authorized downloads, audit, REST/OpenAPI, tests, scripts, and documentation.

Detailed plan: `plans/0006-progress-updates-and-attachments.md`.

### Feature: Comments and accepted suggestions

Status: Implemented and deployed; database-live verification pending

Phase: 7

Depends on: progress updates.

Objective: Add project discussion and separate Admin acceptance actions that surface official suggestions without modifying comments.

Scope and acceptance: `plans/0008-review-comments-suggestions-assessments.md`.

### Feature: Admin assessments

Status: Implemented and deployed; database-live verification pending

Phase: 7

Depends on: tasks and identity.

Objective: Maintain a prominent current verdict/remark plus immutable assessment history.

Scope and acceptance: `plans/0008-review-comments-suggestions-assessments.md`.

### Feature: Reporting APIs, audit access, and operational hardening

Status: Implemented and deployed; database-live/restore verification pending

Phase: 8

Depends on: core workflows.

Objective: Complete useful dashboard query contracts, full Admin audit access, backup/recovery documentation, and production handoff for the separate frontend and operators.

Scope and acceptance: `plans/0009-reporting-audit-and-recovery.md`.

### Feature: Production hosting at `/backend`

Status: Verified and deployed

Phase: 8

Depends on: foundation, trusted identity, and production migration safety.

Objective: Run the backend durably behind Caddy at `ppr.transev.site/backend` with a loopback-only service, clean migrated PostgreSQL state, protected secrets, private attachments, recovery evidence, and prefix-correct routes, response links, and interactive contracts.

Verification: Application, database, systemd, listener, loopback health, route isolation, Caddy validation/reload, authoritative IPv4/IPv6 DNS, Let's Encrypt certificate, and public HTTPS behavior pass.

Detailed plan: `plans/0007-production-hosting.md`.

### Feature: Complete frontend integration handoff

Status: Implemented

Phase: 8

Depends on: the complete deployed backend contract.

Objective: Let a separate frontend engineer build the whole product without backend code archaeology or undocumented chat context.

Scope: One self-contained browser guide covering same-origin cookie/CSRF transport, roles and ownership, all OpenAPI operations and schemas, mutation bodies, multipart uploads, evidence labels, optimistic versions, pagination, errors, retries, screen data flows, and a drift verifier registered in the full suite.

Canonical guide: `integrations/FRONTEND_INTEGRATION.md`.

## Current execution

Current phase: Operational verification

Active feature: Database lifecycle and restore verification

Current implementation slice: None

Last completed slice: Implemented and locally verified multiple editable task responsibilities through additive V2 task endpoints while retaining V1 compatibility

Next expected slice: Return to database-live lifecycle verification and the coordinated restore drill on explicitly disposable local targets

Blocked by: None

## Next approved work

1. Run the remaining lifecycle verification and coordinated restore drill only on explicitly disposable database/filesystem targets.

## Risks and unresolved decisions

- The dashboard intentionally exposes facts only; a future product decision may define “needs progress update” without changing current factual fields.
- Choose the off-host backup target, retention window, and systemd timer schedule before operational activation; the coordinated scripts and restore procedure are implemented.
- Email delivery remains deferred; account creation and reset use generated temporary credentials displayed exactly once and require replacement after login.
- Members with current project access may read other authors' progress revision history; responsibility and creator ownership affect mutation, not project read visibility.
- Confirm display timezone policy; storage remains UTC.
- A live migration integration test needs an explicitly disposable database workflow; existing databases are never assumed disposable.

## Verification strategy

Every slice normally runs formatting, focused tests, package tests, `go vet`, build, contract validation, relevant loopback integration checks, residue scan, full verification, `git diff --check`, and `git status --short`. At the operator's direction, execution of tests, builds, schema validation, and live verifiers is temporarily deferred until backend feature completion; verification assets continue to be authored in each slice. Earlier checks retain their recorded results. Database-modifying verification uses only an explicitly configured disposable test database; guarded lifecycle scripts never belong in automatic verification.

By explicit operator direction, implementation after the hosted-contract correction proceeded without executing tests until the backend feature set was complete. At completion, focused packages, OpenAPI validation/route coverage, the full formatter/tidy/vet/test/build verifier, race detection, Bash syntax, and all three loopback API-documentation/base-path smoke states passed. Database-live migration/lifecycle verification and the coordinated empty-target restore drill remain pending because no disposable targets were authorized in this pass.

For Step 06, focused progress/filestore/HTTP tests, OpenAPI validation and route coverage, module tidiness, vet, all Go tests, build, race detection, PowerShell syntax, `git diff --check`, and both loopback API-documentation smoke modes pass. All seven migrations are applied to production `pprdb`; guarded data-creating database-live verifiers remain intentionally pending for a separate disposable target.

For production hosting, prefix-focused tests, Linux formatting/tidiness/vet/tests/race/build checks, OpenAPI validation, systemd/Caddy validation, migration status, clean row counts, service/listener inspection, loopback liveness/readiness, route isolation, both docs-toggle states, Caddy reload, authoritative IPv4/IPv6 DNS, certificate inspection, and public HTTPS behavior pass. Docs are currently enabled by explicit operator request.

The hosted-contract correction has passing focused coverage for prefixed attachment `content_path` values and deployment-resolved OpenAPI server defaults. The full verifier and all three loopback smoke states pass, including root docs enabled/disabled and `/backend` docs enabled with unprefixed isolation. Production serves the corrected current binary; public TLS readiness and the raw schema's resolved `/backend` default are verified.

The reporting/recovery review follow-up is also deployed. Production remains at seven applied migrations, public IPv4/IPv6 readiness passes, and the hosted OpenAPI timeline operation exposes the verified bounded `limit`, opaque `cursor`, and optional `next_cursor` contract.

The complete frontend handoff consolidates every browser-visible contract and workflow into `integrations/FRONTEND_INTEGRATION.md`. Its verifier checks that every OpenAPI operation ID and path remains represented and runs inside `scripts/verify-all.ps1`.

## V1 completion criteria

All approved backend workflows operate end to end with authorization, durable constraints, audit events, failure/recovery behavior, authoritative OpenAPI, frontend-ready semantics, current documentation, focused tests, and broad verification. Deployment is a separately authorized operation.
