# Development plan

Status: Approved product direction; account, project, and task slices implemented with automated verification passing and database-live verification pending

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

Status: Implemented

Phase: 3

Depends on: trusted identity.

Objective: Expose secure APIs for Admins to list, create, disable, re-enable, reset, inspect, and role-change users while users can replace temporary credentials safely.

Scope: one larger backend vertical slice covering persistence, final-enabled-Admin concurrency safety, generated temporary credentials returned once, forced password change, session revocation, audit query, JSON/OpenAPI, tests, verification scripts, and authoritative documentation.

Detailed plan: `plans/0003-account-administration.md`.

### Feature: Projects, membership, and geofences

Status: Implemented

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

### Feature: Verified progress updates and attachments

Status: Approved

Phase: 6

Depends on: tasks and geofences.

Objective: Deliver the central mobile camera/location submission, immutable revision history, recoverable file storage, metadata trust labels, and chronological task register.

### Feature: Comments and accepted suggestions

Status: Approved

Phase: 7

Depends on: progress updates.

Objective: Add project discussion and separate Admin acceptance actions that surface official suggestions without modifying comments.

### Feature: Admin assessments

Status: Approved

Phase: 7

Depends on: tasks and identity.

Objective: Maintain a prominent current verdict/remark plus immutable assessment history.

### Feature: Reporting APIs, audit access, and operational hardening

Status: Approved

Phase: 8

Depends on: core workflows.

Objective: Complete useful dashboard query contracts, full Admin audit access, backup/recovery documentation, and production handoff for the separate frontend and operators.

## Current execution

Current phase: Task register

Active feature: Tasks and safe Markdown

Current implementation slice: Step 05 implemented and automated verification complete — database-live lifecycle verification pending

Last completed slice: Step 04 — project access and geofence policy implemented with automated verification passing

Next expected slice: Verified progress updates and attachments

Blocked by: Database-live verification requires an explicitly disposable, fully migrated database with zero users

## Next approved work

1. Run the guarded account, project, and task lifecycle verifiers only on an explicitly disposable, fully migrated database with zero users.
2. Reset the explicitly disposable verification database before real use and configure persistent production security values.
3. Implement verified progress updates and attachments as the next larger backend slice if coding continues before verification resumes.

## Risks and unresolved decisions

- Define the exact dashboard meaning of “needs progress update” before implementing it.
- Confirm attachment allowlist, per-file limit, per-update limit, and retention/backup targets before the upload slice.
- Email delivery remains deferred; account creation and reset use generated temporary credentials displayed exactly once and require replacement after login.
- Confirm the Member visibility policy for other authors’ update revision history.
- Confirm display timezone policy; storage remains UTC.
- A live migration integration test needs an explicitly disposable database workflow; existing databases are never assumed disposable.

## Verification strategy

Every slice normally runs formatting, focused tests, package tests, `go vet`, build, contract validation, relevant loopback integration checks, residue scan, full verification, `git diff --check`, and `git status --short`. Those checks pass for Steps 03–05, including race detection and both API-documentation toggle smoke modes. Database-modifying verification uses only an explicitly configured disposable test database; the guarded account/project/task lifecycle scripts remain pending because the configured database is not empty.

## V1 completion criteria

All approved backend workflows operate end to end with authorization, durable constraints, audit events, failure/recovery behavior, authoritative OpenAPI, frontend-ready semantics, current documentation, focused tests, and broad verification. Deployment is a separately authorized operation.
