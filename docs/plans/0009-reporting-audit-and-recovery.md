# Plan 0009 — Reporting, full audit, and backup recovery

Status: Implemented and automated-verified; database-live and restore-drill verification pending

## Objective

Complete the backend handoff with neutral frontend-ready home summaries, paginated Admin access to the complete append-only audit stream, and guarded coordinated PostgreSQL/attachment backup and restore operations.

## Approved behavior

- `GET /api/v1/dashboard` returns only facts: authorized project summaries, task/update/suggestion counts, current assessment-verdict counts, and latest progress time. It does not invent the unresolved “needs progress update” business rule.
- Admins see every project; Members see only current memberships. Inactive authorized projects remain included and explicitly labelled.
- `GET /api/v1/admin/audit` is Admin-only and returns every audit action category with bounded keyset pagination plus optional exact action, outcome, actor, and target-type filters.
- The existing identity-only audit endpoint remains compatible.
- Every Admin or current project Member may read a complete oldest-first task timeline through bounded opaque keyset pages. It combines task before/after revisions, progress/revisions, attachments and state/access events, comments, accepted suggestions, and assessment versions with event-specific metadata.
- Timeline output excludes audit client IP, user agent, request ID, and other security-only request context. Markdown metadata includes a sibling sanitized HTML projection.
- Audit and timeline cursors are opaque, stable across equal timestamps, and rejected as validation errors when malformed.
- Backup captures PostgreSQL plus the private attachment root during one maintenance stop, hashes both artifacts, and restarts the service even after failure.
- Restore requires an explicit confirmation flag, a stopped service, an empty database, and an empty attachment destination. It verifies manifest hashes before writing and never drops or resets a database.

## Surface map

- Migration `000007`: append-only before/after task revisions for complete future reconstruction
- Projects module: authorized aggregate dashboard and task-timeline read models
- Identity module: validated filter/cursor contract and complete audit query
- HTTP/OpenAPI: dashboard and full-audit routes, parameters, envelopes, errors, and compatibility
- Operations: guarded production-native backup/restore scripts and recovery guide
- Verification assets: focused service/route tests plus script syntax/static contract coverage, authored now and executed at backend completion
- Repository memory: architecture, API, PostgreSQL/storage integration, state, plan, and changelog

## Acceptance criteria

- Dashboard counts derive from one PostgreSQL statement over trusted project scope and never leak inaccessible project identifiers.
- Assessment counts use only each task's latest durable assessment.
- Timeline reads use the same current Admin/member project scope as the task, return at most 200 rows oldest-first, and retain enough immutable metadata to recreate each state transition. Attachment creation metadata remains `pending` even after later lifecycle events.
- Audit pages return at most 200 rows, newest first; `next_cursor` continues strictly after the last returned `(occurred_at,id)` pair.
- Invalid filters/cursors return `422`; Members receive `403` for full audit.
- Backup and restore keep database credentials out of command arguments and output, expose no listeners, and never silently overwrite existing state.
- Contracts give the separate frontend and operators enough information to integrate without code archaeology.

## Verification policy

Tests and operational scripts were authored during implementation and executed only after the backend feature set was complete. Focused/full/race/contract/loopback and script-syntax checks pass. No production service, database, attachment root, or hosted contract was modified.

## Implementation result

The neutral dashboard, complete paginated audit, bounded authorized task timeline, migration `000007`, OpenAPI/human contracts, focused tests, extended live verifier, coordinated backup/restore scripts, and operator documentation are implemented. Review follow-up added stable timeline pagination, creation-time attachment facts, credential-safe database tool invocation, and executable script modes. Automated verification passes; database-live migrations/lifecycle and a disposable restore drill remain required for full verification.
