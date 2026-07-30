# Plan 0005 — Task register and safe Markdown

Status: Implemented; automated verification passes, database-live lifecycle verification pending

The original singular responsibility transport remains available as V1 compatibility. Plan 0010 supersedes only the assignment cardinality for new V2 task integrations; creator ownership, Markdown, dates, authorization, and concurrency decisions in this plan remain active.

## Objective

Deliver the complete backend task register as one larger vertical slice: project-scoped create/list/detail/update, immutable creator ownership, optional responsible Member and target date, sanitized Markdown projections, optimistic concurrency, transactional audit, JSON/OpenAPI contracts, and verification assets.

## Decisions

- `tasks` stores authoritative Markdown source. Sanitized HTML is derived on every API response and is never a second durable source of truth.
- Goldmark parses GitHub-flavored Markdown and Bluemonday applies an explicit UGC allowlist. Raw embedded HTML, scripts, unsafe URLs, and event attributes never pass through to clients.
- The same renderer now supplies sanitized HTML for existing project descriptions and task goals/descriptions.
- Admins may create and edit tasks in any active project. Members require current project membership and may edit only tasks whose immutable `created_by` matches their authenticated identity.
- `responsible_user_id` is optional and never grants project access or edit ownership. When set, it must identify an enabled Member with current membership in the same project.
- Removing project membership atomically clears that user's task responsibilities and increments affected task versions; the membership audit records the affected count.
- `target_date` is an optional calendar date (`YYYY-MM-DD`) with no invented timezone or future-date requirement.
- Task edits use optimistic `version` checks. V1 does not delete tasks or add status/Kanban workflow.
- Inactive projects remain readable but reject new task creation and task edits.

## Surface map

- Dependencies: Goldmark Markdown parser and Bluemonday sanitizer
- Migration: `tasks` with ownership, optional responsibility/date, constraints, version, and indexes
- Domain: validation, safe derived HTML, access/ownership policy, and audit semantics
- PostgreSQL: authorized project locks, scoped reads, responsible-Member validation, optimistic updates, and transactions
- HTTP: nested project/task routes, authentication, CSRF, strict payloads, and error mapping
- API: task list/create/detail/update and expanded project Markdown projection
- Verification assets: renderer/domain/HTTP tests, route coverage, and disposable live task verifier
- Documentation: architecture, domain, permissions, API, PostgreSQL, state, plan, changelog, and dependency rationale

## Routes

- `GET /api/v1/projects/{project_id}/tasks`
- `POST /api/v1/projects/{project_id}/tasks`
- `GET /api/v1/projects/{project_id}/tasks/{task_id}`
- `PATCH /api/v1/projects/{project_id}/tasks/{task_id}`

## Acceptance criteria

- Authorized users can list and read tasks; inaccessible project/task identifiers return `404` without revealing existence.
- Admins and current project Members can create tasks only in active projects.
- Only Admins or the immutable task creator can update a task, and Member creators still require current project access.
- Responsible assignment requires a current enabled Member in the same project and does not confer access or ownership.
- Stale expected versions return `409` without mutation.
- Markdown source is preserved exactly within size bounds; returned HTML is sanitized and unsafe HTML/URLs are absent.
- All writes require session-bound CSRF and atomically append `task.created` or `task.updated`.
- OpenAPI and human contracts allow a separate frontend to implement the task workflow without undocumented assumptions.
- Automated sanitizer, domain, HTTP, contract, race, build, and loopback smoke checks pass; the guarded database-live script remains pending.

## Verification backlog

Completed: dependency integrity, formatter, Markdown safety tests, focused domain/HTTP authorization tests, OpenAPI validation/route coverage, build, vet, race detector, loopback foundation/docs smoke checks, residue scan, full suite, and Git checks. Pending: migration application and the task lifecycle verifier on an explicitly disposable zero-user database.
