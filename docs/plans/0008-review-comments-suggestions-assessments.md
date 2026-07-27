# Plan 0008 — Comments, accepted suggestions, and Admin assessments

Status: Implemented and automated-verified; database-live verification pending

## Objective

Complete the backend review workflow in one coherent slice: immutable comments on progress updates, separate Admin acceptance actions surfaced at task level, and an Admin-owned current assessment with immutable history.

## Approved behavior

- Admins and current project Members may list and add comments to progress updates in active projects. Inactive projects remain readable but reject new comments.
- Comments are immutable Markdown records with derived sanitized HTML. They cannot be edited or deleted.
- Only an Admin may accept a comment as an official suggestion. Acceptance is a separate immutable one-to-one record and never changes comment text.
- Repeating acceptance for the same scoped comment is idempotent: the existing acceptance is returned without a duplicate row or audit event.
- Authorized project readers may list accepted suggestions for a task. Results retain the source update, comment, author, accepting Admin, and server timestamps.
- Authorized project readers may read the current task assessment. Only Admins may list complete assessment history or append a new assessment.
- Assessments use `on_track`, `needs_attention`, `blocked`, or `complete`, require a Markdown remark, derive sanitized HTML, and append monotonically versioned immutable rows.
- Assessment writes require `expected_version`: zero creates the first assessment; later writes must match the current version. Concurrent stale writes return `409`.
- Review writes require authentication, CSRF, trusted project/task/update scope, an active project, and append audit in the same transaction.

## Surface map

- Migration `000006`: comments, accepted suggestions, assessments, constraints, indexes, and immutable-row triggers
- Progress module: scoped comment list/create, idempotent Admin acceptance, task-level suggestion list, Markdown rendering, audit
- Projects module: current/history assessment queries, Admin append policy, optimistic versioning, Markdown rendering, audit
- HTTP: nested routes, authentication, CSRF, method handling, errors, and response envelopes
- Contracts: complete OpenAPI request/response/error schemas and human-readable semantics
- Verification assets: service/HTTP/contract tests and a guarded disposable live review script
- Repository memory: architecture, domain, permissions, project state, development plan, changelog, and documentation index

## Routes

- `GET|POST /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/comments`
- `POST /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/comments/{comment_id}/accept`
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/accepted-suggestions`
- `GET|PUT /api/v1/projects/{project_id}/tasks/{task_id}/assessment`
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/assessments`

## Acceptance criteria

- Every identifier is resolved through current Admin/member project scope; inaccessible or mismatched identifiers return the same scoped `404`.
- Comment and assessment Markdown is bounded, stored as source, and returned with sanitized HTML.
- Comment creation, first suggestion acceptance, and assessment append each commit state plus audit atomically.
- Duplicate acceptance returns the existing result without duplicating state or audit.
- Members can read current assessment and accepted suggestions but cannot accept suggestions, inspect assessment history, or write assessments.
- Stale assessment versions return `409` and preserve the current assessment.
- OpenAPI and human contracts are sufficient for the separate frontend to implement the full review workflow.

## Verification policy

Tests and scripts were authored during implementation and executed only after backend feature completion, per operator direction. Focused/full/race/contract/loopback checks pass. No production database or live service was modified; the guarded database-live workflow remains pending.

## Implementation result

Migration `000006`, the progress/projects application boundaries, seven nested HTTP operations, authoritative OpenAPI schemas, focused service/route tests, `scripts/verify-live-review.ps1`, and the linked educational/contract/project-memory documentation are implemented. Automated verification passes; this status must not be promoted to fully Verified until the disposable live workflow passes.
