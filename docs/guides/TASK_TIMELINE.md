# Task timeline

## Purpose and access

`GET /api/v1/projects/{project_id}/tasks/{task_id}/timeline` gives every Admin or current project Member who can view the task an oldest-first page of the complete “what happened” chronology. `limit` defaults to 100 and accepts 1–200. When `next_cursor` is present, pass it unchanged as `cursor` on the next request and continue until `next_cursor` is absent. Malformed limits or cursors return `422`; inaccessible and mismatched identifiers return scoped `404`. Deactivation does not hide history from an otherwise authorized viewer.

This is a domain chronology, not the Admin security audit. It intentionally excludes client IP, user agent, request ID, login activity, and unrelated project activity.

## Event coverage

The stream uses stable source-derived IDs and includes:

- `task.created` and every `task.updated` before/after revision;
- `progress.created` and every `progress.updated` before/after revision;
- `attachment.added`, `attachment.available`, `attachment.failed`, and `attachment.downloaded`;
- `comment.created` and `suggestion.accepted`;
- every `assessment.created` version.

Each event contains `action`, `entity_type`, `entity_id`, actor identity, server time, and action-specific `metadata`. Migration `000007` makes task edits as reconstructable as progress edits; migration `000008` extends those snapshots with complete `responsible_user_ids` arrays. The singular `responsible_user_id` remains as a deterministic compatibility value in task event metadata, while new clients must use the plural array. `attachment.added` records the creation-time `pending` state; later availability or failure is represented by its own event rather than rewriting the earlier fact.

Metadata fields ending in `_markdown` receive a sibling `_html` value rendered through the backend allowlist sanitizer. Frontends may render only the HTML projection as HTML. Attachment metadata includes evidence classification, type, size, SHA-256, and storage state without exposing storage keys or filesystem paths.

## Ordering, restart, and recovery

Events order by server timestamp and then stable event ID. The opaque cursor contains that ordering position, and keyset continuation begins strictly after it. PostgreSQL domain/history tables are authoritative; no cache or process-memory stream is involved. Reload and restart simply re-run the authorized query. A client that abandons an old pagination session may start again without a cursor; no realtime replay or reconciliation protocol is required.

The current production database was empty before these workflow migrations. For any future database imported from a version predating `000007`, task edits made before that migration cannot gain unavailable before/after content retroactively; all later edits are complete.
