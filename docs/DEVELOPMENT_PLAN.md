# Development plan

Status: Approved product direction; foundation implemented with live PostgreSQL success-path verification pending

## Project objective

Deliver a low-memory internal web register that makes day-to-day project progress as simple as a paper diary while enforcing authorization, durable history, explicit evidence trust, safe attachments, and complete auditing.

## System boundaries and direction

- Go modular monolith, PostgreSQL, server-rendered HTML, focused HTMX, and minimal vanilla JavaScript.
- One organization and no v1 multi-tenancy.
- Caddy terminates production TLS; the Go listener remains loopback-only.
- Local filesystem attachments are private and replaceable behind an application boundary.
- PostgreSQL is durable truth; browser evidence is operational rather than cryptographic.
- Specification-first OpenAPI covers every implemented JSON endpoint.
- No Redis, event broker, microservice, SPA, container, public listener, blockchain, or production Node.js runtime in v1.

## Development phases

1. **Foundation** — repository memory, runtime, configuration, PostgreSQL/migrations, health, OpenAPI, scripts, and verification.
2. **Trusted identity** — initial Admin bootstrap, authentication, sessions, CSRF, throttling, and audit foundation.
3. **Account administration** — website user lifecycle, roles, reset flow, and account audit view.
4. **Project access** — projects, membership, Markdown description, and geofences.
5. **Task register** — tasks, ownership, responsible member, target date, Markdown, and concurrency.
6. **Verified progress** — chronological updates, camera capture, location verification, attachments, revisions, and recovery.
7. **Review** — comments, accepted suggestions, and Admin assessments.
8. **Home and operational hardening** — dashboards, audit viewer, accessibility, backup/restore, and deployment documentation.

Each phase depends on the security and state boundaries before it and must complete UI, application, persistence, authorization, API contract, verification, and documentation together.

## Feature registry

### Feature: Application foundation

Status: Implemented

Phase: 1

Objective: Provide the smallest runnable, documented, contract-checked Go/PostgreSQL base on which complete vertical features can be built safely.

Scope: repository instructions, project memory, one binary, loopback config, home page, health/readiness, migration ledger, OpenAPI/Swagger toggle, PowerShell scripts, and tests.

Non-goals: domain tables, authentication, business workflows, uploads, deployment, or speculative module scaffolding.

Acceptance and verification are detailed in `plans/0001-foundation.md`.

### Feature: Trusted identity and audit foundation

Status: Ready

Phase: 2

Depends on: application foundation and its pending live PostgreSQL success-path verification.

Objective: Bootstrap the first Admin once, authenticate enabled users securely, manage opaque database sessions and CSRF, throttle login, and append required authentication audit events.

Acceptance criteria include no public registration, secure hashes/tokens/cookies, session revocation, final-Admin protection, backend enforcement, complete contracts, and focused security tests.

### Feature: Website account administration

Status: Approved

Phase: 3

Depends on: trusted identity.

Objective: Let Admins create, disable, re-enable, reset, inspect, and role-change users entirely through the website.

### Feature: Projects, membership, and geofences

Status: Approved

Phase: 4

Depends on: identity and account administration.

Objective: Create project-scoped access and immutable evidence-policy history without exposing projects through guessed identifiers.

### Feature: Tasks and safe Markdown

Status: Approved

Phase: 5

Depends on: project access.

Objective: Provide a simple task register with immutable creator ownership, optional responsibility/target date, safe Markdown, and concurrency protection.

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

### Feature: Home, audit viewer, and operational hardening

Status: Approved

Phase: 8

Depends on: core workflows.

Objective: Complete useful dashboards, full Admin audit access, accessibility/browser checks, backup/recovery documentation, and production handoff.

## Current execution

Current phase: Foundation

Active feature: Application foundation

Current implementation slice: Step 01 — live PostgreSQL migration/readiness success-path verification

Last completed slice: Foundation implementation and all non-database verification

Next expected slice: Trusted identity and audit foundation

Blocked by: No explicit safe PostgreSQL connection is configured; the existing local database is not assumed disposable

## Next approved work

1. Run migration and ready-state verification against an explicitly approved development/test database.
2. Implement one-time Admin bootstrap, login/logout, sessions, CSRF, throttling, and authentication audit events.
3. Implement website-based account administration.
4. Implement projects, membership, and geofence policy.

## Risks and unresolved decisions

- Define the exact dashboard meaning of “needs progress update” before implementing it.
- Confirm attachment allowlist, per-file limit, per-update limit, and retention/backup targets before the upload slice.
- Confirm whether password reset remains Admin-displayed one-time links or later gains email delivery.
- Confirm the Member visibility policy for other authors’ update revision history.
- Confirm display timezone policy; storage remains UTC.
- A live migration integration test needs an explicitly disposable database workflow; existing databases are never assumed disposable.

## Verification strategy

Every slice runs formatting, focused tests, package tests, `go vet`, build, contract validation, relevant loopback integration checks, residue scan, full verification, `git diff --check`, and `git status --short`. Database-modifying verification uses only an explicitly configured disposable test database.

## V1 completion criteria

All approved workflows operate end to end with backend authorization, durable constraints, audit events, failure/recovery behavior, mobile-accessible UI, authoritative OpenAPI, current documentation, focused tests, and broad verification. Deployment is a separately authorized operation.
