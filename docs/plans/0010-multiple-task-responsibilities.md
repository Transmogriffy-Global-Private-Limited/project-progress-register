# Plan 0010 — Multiple editable task responsibilities

Status: Verified and deployed

## Objective

Allow each task to have zero or more responsible project Members while preserving the existing task ownership, authorization, optimistic concurrency, immutable revision history, timeline reconstruction, and membership-removal guarantees.

## Decisions

- New `/api/v2/projects/{project_id}/tasks...` list/create/detail/update operations expose the plural contract. `responsible_user_ids` is the complete desired set; task update atomically replaces the current set together with every other mutable task field.
- Existing `/api/v1/projects/{project_id}/tasks...` operations remain available with their singular `responsible_user_id` / `responsible_member` contract while integrations migrate.
- V1 create/update continues normal singular behavior while the task has zero or one current assignee. Once V2 assigns multiple Members, V1 reads expose a deterministic compatibility member but V1 update returns a documented conflict instead of silently discarding assignments the old client cannot represent.
- V2 returns `responsible_members` as a deterministic array. The two transport versions adapt one shared domain model and one authoritative assignment table; they are not competing durable sources of truth.
- Every supplied user ID must be unique and identify an enabled Member with current membership in the same project. An empty array means no assignment.
- Responsibility remains work assignment only. It grants neither project access, task ownership, nor edit permission.
- A join table is authoritative for current assignments. Migration `000008` copies every existing singular assignment before removing the obsolete task column and index.
- Task revisions store complete before/after responsible-user ID arrays. Existing revision rows are converted without losing their historical singular value.
- Task edits lock the task, validate the complete assignment set, replace it transactionally, increment the task version once, append one revision, and append one audit event.
- Removing project membership deletes that Member from every affected task assignment set, increments each affected task once, appends one revision per affected task, and records the affected-task count in the membership audit event.

## Surface map

- Migration: current responsibility join table and array-based revision snapshots
- Domain/API types: plural assignment input and response fields
- PostgreSQL: deterministic reads, set validation/replacement, revisions, and membership removal
- HTTP/OpenAPI: additive V2 task routes and schemas plus retained, explicitly bounded V1 compatibility behavior
- Timeline: plural assignment snapshots in task creation and update metadata
- Verification: service/HTTP/migration/timeline tests and all guarded live workflow scripts
- Documentation: product, architecture, domain, permissions, API, frontend handoff, state, plan, and changelog

## Acceptance criteria

- Task create accepts zero or more unique current enabled project Member UUIDs and returns the resolved Member array.
- Admins and authorized Member creators can atomically add, remove, or replace assignments through task update with `expected_version`.
- Invalid, duplicate, disabled, non-Member-role, or out-of-project users fail without partially changing the task.
- Assignment-only changes increment the task version exactly once and appear in immutable timeline before/after metadata.
- Membership removal clears only the removed Member from each assignment set, retains all other assignees, and versions each affected task once.
- Existing singular production assignments migrate losslessly.
- OpenAPI, frontend integration documentation, guarded scripts, focused tests, and broad verification agree with both the retained V1 and plural V2 contracts.

## Verification

- Focused projects and HTTP tests
- Migration discovery and schema tests
- OpenAPI validation and route coverage
- Frontend integration documentation drift verification
- Full formatter, tidy, vet, tests, build, race detector, PowerShell parsing, loopback smoke states, residue scan, and Git checks
- Database-live execution remains restricted to an explicitly disposable local target

## Result

Implemented migration `000008`, the shared plural task domain/persistence flow, additive V2 task routes, safe V1 compatibility, complete assignment revision/timeline metadata, membership-removal cleanup, OpenAPI, frontend contracts, and verification assets. A loopback-only disposable PostgreSQL 17 target proved lossless migration of existing task/revision responsibility values and the complete guarded V2/V1 workflow. Focused tests, OpenAPI and frontend drift validation, the full verifier, all-package race detection, and all three root/prefixed API-documentation smoke states pass.

Production was backed up as one database/filesystem recovery point, advanced to eight applied and zero pending migrations, and rehosted from clean `main`. The migration copied the existing singular assignment into `task_responsibilities`; schema inspection confirms the obsolete task column is absent and the plural revision column is present. Loopback and public IPv4/IPv6 readiness, Swagger, and the hosted V2 OpenAPI contract pass. The guarded data-creating workflow remains restricted to disposable targets.
