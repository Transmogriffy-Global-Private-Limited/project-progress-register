# Plan 0004 — Project access and geofence policy

Status: Implemented; automated verification passes, database-live lifecycle verification pending

## Objective

Deliver the complete backend project-access boundary as one larger vertical slice: project lifecycle, current Member access, historical membership records, versioned site geofence policy, authorization, audit, JSON/OpenAPI contracts, and verification assets.

## Decisions

- Admin is globally authorized for every project. A Member can read a project only through an active `project_members` row; identifiers alone grant nothing.
- Projects are never deleted in v1. Deactivation preserves history and remains readable to authorized users; later operational commands may require an active project.
- Project name, Markdown description, and active state use optimistic `version` checks.
- Membership removal closes a temporal row instead of deleting it. Re-adding creates a new row, preserving access history.
- Only enabled users with the Member role can be assigned. Admin access is global and is not represented as project membership.
- Each project has at most one current geofence. A change closes the current version and inserts the next immutable version in one transaction.
- Geofences use decimal latitude/longitude, radius metres, and maximum accepted reported accuracy metres. No map provider or spatial extension is required.
- Account-administration and project-access work remain local and uncommitted until the paused verification backlog resumes.

## Surface map

- Migration: `projects`, temporal `project_members`, and versioned `project_geofences`
- Domain: validation, Admin commands, scoped queries, optimistic conflicts, and audit semantics
- PostgreSQL: authorized query shapes, transactions, row locks, history, constraints, and indexes
- HTTP: cookie authentication, CSRF on writes, strict UUID/path parsing, status mapping, and route wiring
- API: project list/create/read/update, membership list/add/remove, and geofence replacement
- Verification assets: service/HTTP tests, OpenAPI route coverage, and a disposable live project-access verifier
- Documentation: architecture, domain, permissions, contracts, PostgreSQL, project state, plan, and changelog

## Routes

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{project_id}`
- `PATCH /api/v1/projects/{project_id}`
- `GET /api/v1/projects/{project_id}/members`
- `PUT /api/v1/projects/{project_id}/members/{user_id}`
- `DELETE /api/v1/projects/{project_id}/members/{user_id}`
- `PUT /api/v1/projects/{project_id}/geofence`

## Acceptance criteria

- An enabled Admin without forced password replacement can manage every project, membership, and geofence.
- An enabled Member without forced password replacement sees only projects with current membership and receives `404` for other project identifiers.
- Project updates reject stale versions without mutation.
- Membership changes preserve history, reject non-Member/disabled targets, and take effect immediately.
- Concurrent geofence changes serialize on the project, reject stale expected versions, and preserve every superseded policy.
- Latitude, longitude, radius, and accuracy bounds are validated in both application code and PostgreSQL constraints.
- All writes require the session-bound CSRF token and append secret-free audit events in the state-change transaction.
- OpenAPI and human contracts are sufficient for a separate frontend.
- Automated tests, contract checks, race detection, build, and loopback smoke checks pass. The guarded database-live script remains pending.

## Verification backlog

When resumed: formatter, migration discovery/application on an explicitly disposable database, focused domain/store/HTTP tests, authorization checks, OpenAPI validation and route coverage, build, vet, race detector, loopback live lifecycle verifier, residue scan, full suite, and Git checks.
