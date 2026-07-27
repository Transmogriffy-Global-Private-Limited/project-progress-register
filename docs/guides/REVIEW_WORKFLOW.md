# Review workflow

## Purpose

Review remains attached to the chronological project record. A comment records discussion on one progress update, an accepted suggestion records an Admin decision without rewriting that comment, and an assessment records the Admin's current task verdict while retaining every prior verdict.

## Causal flow

```text
authorized project reader
-> comment on an active task update
-> immutable Markdown source plus sanitized HTML projection
-> optional separate Admin acceptance
-> task-level accepted-suggestion query

Admin assessment command
-> active project and scoped task
-> expected current version
-> append next immutable assessment plus audit
-> current query returns highest version
-> Admin history returns every version newest-first
```

PostgreSQL is authoritative. Comments, acceptances, and assessments do not depend on sockets, caches, or process memory and therefore survive restart unchanged.

## Frontend integration

An authorized project view may fetch comments, accepted suggestions, and current assessment independently. Render only the backend's `content_html` or `remark_html` projection as HTML; keep the Markdown field for display/editing context and never render raw Markdown as trusted HTML.

Suggestion acceptance is safe to retry. Treat both `201 created=true` and `200 created=false` as success and replace local state with the returned acceptance. Assessment editing must retain the last read `version` and submit it as `expected_version`; after `409 conflict`, reload the current assessment before offering another write.

The current assessment endpoint deliberately returns `200` with `assessment: null` before the first assessment. Members can display current assessment and accepted suggestions but must not be shown Admin-only acceptance, assessment-write, or history controls as if they were authorized. Backend authorization remains decisive.

## Failure and recovery

- Inaccessible or mismatched project/task/update/comment identifiers return the same scoped `404`.
- Inactive projects remain readable but reject review writes with `409 project_inactive`.
- Missing authentication returns `401`; missing or invalid CSRF on a write returns `403`.
- Member acceptance, assessment writes, and history reads return `403` and record the configured denied audit event.
- Stale assessment versions return `409` without changing history.
- Transaction rollback prevents a comment, first acceptance, or assessment from committing without its audit record.

The guarded `scripts/verify-live-review.ps1` exercises these boundaries against a fully migrated database containing zero users. It intentionally creates data and must run only on an explicitly disposable target after testing resumes.
