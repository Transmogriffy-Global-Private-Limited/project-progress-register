# Domain model

Status: Approved design; domain tables are not implemented in the foundation slice.

## Aggregate relationships

```text
Project
  -> current members
  -> current geofence
  -> Tasks
      -> chronological Progress Updates
          -> verified location evidence
          -> camera captures
          -> existing-file uploads
          -> revisions
          -> comments
              -> optional accepted-suggestion action
      -> current Admin Assessment
      -> immutable assessment history
```

## Identity

`users` owns normalized username/email, password hash, global role, enabled state, and timestamps. User identity is never reused. Disabling an account ends active access but preserves authorship and audit history.

`sessions` owns only a hash of a random opaque token, user, creation/expiry times, and revocation data. A raw session token exists only in the secure cookie.

The system must always retain at least one enabled Admin.

## Projects and membership

`projects` owns name, Markdown description, active state, immutable creator, and timestamps. `project_members` represents current Member access; uniqueness prevents duplicate membership.

`project_geofences` records the current site policy and retains enough history or snapshot data for later evidence to show exactly which centre, radius, and accuracy threshold were evaluated. Changing a geofence never rewrites evidence already submitted.

## Tasks

`tasks` belongs to one project and owns name, Markdown goals and description, immutable creator, optional responsible member, optional target date, version, and timestamps. Responsible membership is mutable and never defines edit ownership.

Task history will preserve materially edited content where needed for audit and concurrency behavior. No general task deletion is required in v1.

## Progress updates and revisions

`progress_updates` belongs to one task and owns current Markdown content, immutable author, authoritative server submission time, version, and timestamps.

`progress_update_revisions` appends one immutable entry for every edit with editor, edited time, previous content, new content, and resulting version. Updates are never silently overwritten.

`update_locations` stores browser-reported coordinates, accuracy, optional browser timestamp, server receipt time, geofence snapshot, computed distance, verification result, and reason. Server computation is authoritative for the result; browser measurements remain operational evidence.

## Attachments

`update_attachments` links an update to opaque storage identity and records original filename, server-detected MIME, size, SHA-256, uploader, server receipt time, source, storage state, and trust classification.

`attachment_metadata` stores extracted EXIF timestamp, GPS, and device/camera details separately as untrusted metadata. Attachment bytes never become publicly addressable durable truth.

## Comments and accepted suggestions

`update_comments` stores immutable comment authorship and content. `accepted_suggestions` is a separate one-to-one acceptance action referencing the original comment, accepting Admin, and timestamp. Acceptance never mutates comment text.

## Assessments

`task_assessments` is append-only. The latest row is the current assessment and every older row remains history. Verdict values begin with `on_track`, `needs_attention`, `blocked`, and `complete`.

## Audit

`audit_events` is append-only and records actor when known, action, target type and identity, server timestamp, outcome, request correlation, and carefully selected context. It never stores passwords, session tokens, reset tokens, attachment bytes, or unrestricted request bodies.

## Common persistence rules

- Application-generated UUIDs avoid database extensions and expose no sequential business information.
- Durable timestamps use PostgreSQL `timestamptz` and UTC semantics.
- Foreign keys, unique constraints, checks, and indexes enforce durable invariants.
- Creator identities are immutable even after membership or account state changes.
- Transactions encompass each business state transition and its audit record.
- Current access is evaluated at request time; historical authorship is retained without granting access.
