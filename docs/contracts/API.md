# HTTP API contract

The authoritative machine-readable source is `../../api/openapi/v1/openapi.yaml`. This document owns causal semantics that are awkward to express in schema and identifies behavior whose database-live verification remains pending. The self-contained browser handoff for a frontend team is `../integrations/FRONTEND_INTEGRATION.md`.

## Versioning and representation

Programmatic application routes use `/api/v1` and JSON UTF-8. `BASE_PATH` may prepend one deployment prefix to every route; production uses `/backend`, so the public API begins `/backend/api/v1`. Caddy preserves that prefix. Health, bootstrap, and login are unauthenticated; session recovery and logout use the `ppr_session` cookie. JSON request bodies require `Content-Type: application/json`, reject unknown fields, and are limited to 64 KiB.

## Identity trust and error model

Successful login sets a host-only `ppr_session` cookie with the configured base path plus trailing slash (or `/` without a prefix), `HttpOnly`, `SameSite=Lax`, absolute expiry, and `Secure` in production. The production cookie path is `/backend/`. The cookie contains 32 random bytes encoded for URLs; PostgreSQL stores only its SHA-256 digest. There is no JWT, bearer token, local-storage token, or public registration route.

Every current-session lookup checks the token digest, expiry, revocation, and enabled user state. Login and session recovery return a session-bound `csrf_token`. An authenticated JSON write sends it in `X-CSRF-Token`; HTML forms send `_csrf`. Logout without a current session is idempotently successful.

Errors use `{"error":{"code":"...","message":"..."}}`. Unknown users, disabled users, incorrect passwords, and throttled attempts all return `401 invalid_credentials`; clients must not infer the hidden cause. Server errors use a generic message and keep details in secret-free structured logs.

The account-administration operations below pass automated service, HTTP, contract, race, and build verification. Their disposable-database lifecycle verifier remains pending.

## Forced password replacement

Login and session recovery expose `user.must_change_password`. While true, Admin operations return `403 password_change_required`; a frontend must route the user to its credential-replacement workflow. `POST /api/v1/auth/password` requires the session cookie, `X-CSRF-Token`, and a valid new password. One transaction replaces the hash, clears forced-change state, increments the account version, revokes every user session, and appends `identity.password_changed`. Success is `200` with `password_changed` and `logged_out` true; the cookie is cleared and the user signs in again.

## Admin account operations

All `/api/v1/admin/users` operations require an enabled Admin with `must_change_password=false`. Denied attempts append `authorization.admin_users_denied`. Authenticated writes require `X-CSRF-Token`.

- `GET /api/v1/admin/users` returns the complete username-ordered internal account list. The small internal-user requirement does not yet justify pagination.
- `POST /api/v1/admin/users` accepts normalized `username`, `email`, and `role`. It generates and hashes a temporary password, inserts the account with `must_change_password=true`, and appends `identity.user_created` atomically. `201` returns the temporary password exactly once with `Cache-Control: no-store`; later reads cannot recover it.
- `PATCH /api/v1/admin/users/{user_id}` requires `role`, `enabled`, and `expected_version`. It locks enabled Admins, rejects stale versions, prevents removal of the final enabled Admin, updates the account/version, revokes all target sessions, and appends `identity.user_updated`. Conflicts return `409` with `conflict` or `last_admin`.
- `POST /api/v1/admin/users/{user_id}/password-reset` generates a new temporary password, sets forced-change state, increments version, revokes every target session, and appends `identity.password_reset`. `200` returns the credential once with `Cache-Control: no-store`.
- `GET /api/v1/admin/audit/identity` returns the newest 200 identity, authentication, and account-authorization audit records in reverse chronological order. Actor username is a presentation join; the immutable actor ID and audit row remain authoritative.

This repository does not implement the account-management frontend. A client must treat temporary credentials as single-display secrets, avoid placing them in URLs or persistent browser storage, and discard them after secure handoff.

## Project access

The project-access operations below pass automated service, HTTP, contract, race, and build verification. Their disposable-database lifecycle verifier remains pending. Every project response exposes authoritative `description_markdown` plus read-only `description_html` derived through the shared safe-Markdown renderer.

- `GET /api/v1/projects` returns all projects to Admins and only current memberships to Members. Results include the current geofence or `null`; inactive authorized projects remain visible for historical continuity.
- `POST /api/v1/projects` is Admin-only and accepts `name` plus `description_markdown`. It creates an active version-1 project without members or geofence and appends `project.created` transactionally.
- `GET /api/v1/projects/{project_id}` applies the same trusted scope in its PostgreSQL query. Unknown and inaccessible identifiers both return `404 not_found`.
- `PATCH /api/v1/projects/{project_id}` is Admin-only and requires the full mutable field set plus `expected_version`. It updates name, Markdown description, and active state; stale versions return `409 conflict`. Projects are never deleted in v1.
- `GET /api/v1/projects/{project_id}/members` is Admin-only and returns current Member accounts, including an explicit enabled flag.
- `PUT /api/v1/projects/{project_id}/members/{user_id}` is Admin-only, accepts no body, and creates current membership only for an enabled Member-role account. Duplicate current membership returns `409`; an invalid target returns `422 invalid_member`. Membership assignments persist across later role/enable changes; Admin scope supersedes them, disablement blocks login, and returning to enabled Member restores the retained assignments unless an Admin explicitly removed them.
- `DELETE /api/v1/projects/{project_id}/members/{user_id}` is Admin-only, accepts no body, closes current membership, preserves history, clears that user's current task responsibilities with task-version increments, and returns `204`. Subsequent Member reads lose access immediately; clients should refresh the task list.
- `PUT /api/v1/projects/{project_id}/geofence` is Admin-only and requires latitude, longitude, radius metres, maximum accepted reported accuracy metres, and `expected_version`. Use version zero for the first policy. The transaction locks the project, rejects stale versions, closes the previous policy, inserts the next immutable version, and appends `project.geofence_updated`.

All project writes require `X-CSRF-Token`. Admin denials append `authorization.project_admin_denied`; inaccessible Member detail reads append `authorization.project_denied`. Membership and geofence mutation audit details contain identifiers/versions, never request bodies or location observations.

## Task register and safe Markdown

Task operations pass automated service, HTTP, sanitizer, contract, race, and build verification; their disposable-database lifecycle verifier remains pending. They are nested below an untrusted `project_id`; repository queries and project locks establish current Admin/member scope before accessing a task. Unknown, mismatched, inaccessible, and non-owner mutation identifiers return the same `404 not_found`.

- V1 `GET/POST /api/v1/projects/{project_id}/tasks` and `GET/PATCH /api/v1/projects/{project_id}/tasks/{task_id}` retain the original singular `responsible_user_id` request and `responsible_member` response fields while existing integrations migrate. V1 reads return the deterministic first assignment when V2 has created multiple; V1 update then returns `409 task_v2_required` rather than flattening hidden assignments.
- V2 `GET/POST /api/v2/projects/{project_id}/tasks` and `GET/PATCH /api/v2/projects/{project_id}/tasks/{task_id}` expose the complete assignment set. Create accepts optional `responsible_user_ids` (omission or `null` means empty); update requires the complete non-null array and uses `[]` to remove all assignments. Responses always contain `responsible_members`, ordered by case-insensitive username and user ID.
- Both versions use the same authorization, active-project requirement, immutable creator, optional `YYYY-MM-DD` target date, Markdown fields, scoped `404`, and optimistic `expected_version` semantics. Admins may update any task; a Member must be its immutable creator and retain current project membership.

Every responsibility UUID must be unique and resolve to a current enabled Member in the same project or the whole command returns `422 invalid_responsible_member`/`validation_failed` without mutation. Assignment grants no access or editing rights. Task fields, set replacement, version increment, immutable revision, and `task.created`/`task.updated` audit occur in one transaction; denied scoped/ownership operations append `authorization.task_denied` without task content.

Markdown source fields are authoritative. Project `description_html` and task `goals_html`/`description_html` are read-only Goldmark-plus-Bluemonday projections. Clients must never inject source Markdown as HTML. See `../guides/SAFE_MARKDOWN.md`.

## Progress, geotags, revisions, and attachments

- `GET /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates` returns current entries oldest-first with evidence and attachment metadata. `revisions` is empty in list responses.
- `POST /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates` requires authentication, CSRF, an active authorized project, `Idempotency-Key`, and `multipart/form-data`. The `metadata` JSON part contains Markdown, optional location/unavailable reason, and an ordered attachment descriptor for every repeated `files` part.
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}` returns complete current content, immutable evidence, attachments, and all before/after revisions.
- `PATCH` on the same path replaces current Markdown using `expected_version`; Admins may edit any update, while Members must be immutable author and retain project access. Evidence and attachments cannot be edited.
- `GET .../attachments/{attachment_id}/content` streams only an available file after resolving every parent identifier through current project authorization. Videos use inline disposition and byte ranges; other media retains attachment disposition.

File bytes require `metadata.location` with finite latitude/longitude and positive accuracy. Outside-geofence, inaccurate, and no-geofence results are accepted and stored. Only an attachment with `media_kind=image|video`, browser-reported `source=camera`, and location status `verified` receives attachment `verification_status=verified`. Existing-file media and documents remain `non_verified` while retaining that geotag. See `../guides/PROGRESS_EVIDENCE.md`.

Each file is streamed, bounded, allowlisted by detected bytes, SHA-256 hashed, and stored through pending-to-available recovery. Responses expose a same-origin `content_path` that includes the configured `BASE_PATH`, never a storage key. With production `BASE_PATH=/backend`, download paths begin `/backend/api/v1/`. Pending recovery returns `503 attachment_pending`; irrecoverably missing bytes return `410 attachment_unavailable`. The client retries a pending create using the same idempotency key. Reusing a key with different metadata or file hashes returns `409`.

Successful content reads set detected MIME, `Accept-Ranges: bytes`, `private, no-store`, and `nosniff`. Videos use inline filename disposition for streaming/playback; images and documents use attachment disposition. Audit records progress creation/edit, attachment availability, download/stream intent, and scoped denial without storing Markdown, file bytes, geolocation, or filesystem paths in audit details.

## Comments, accepted suggestions, and assessments

- `GET .../progress-updates/{update_id}/comments` returns immutable comments oldest-first for an authorized project reader. Each item includes Markdown source, sanitized HTML, author/time, and either its separate acceptance record or `null`.
- `POST` on that comments path accepts `content_markdown` and returns `201`. Admins and current Members may comment only while the project is active. The backend requires non-whitespace content, at most 10,000 Unicode characters and 20,000 UTF-8 bytes.
- `POST .../comments/{comment_id}/accept` is Admin-only and requires CSRF. The first scoped acceptance inserts a separate immutable record plus `suggestion.accepted` audit and returns `201` with `created=true`. A retry returns the same record with `200` and `created=false`; it creates no duplicate audit row and never edits the source comment.
- `GET .../tasks/{task_id}/accepted-suggestions` returns accepted comments oldest by acceptance time with source update, author, accepting Admin, and both timestamps. Admins and current project Members may read it, including for an inactive but still authorized project.
- `GET .../tasks/{task_id}/assessment` returns `200` with the latest assessment or `{"assessment":null}`. Admins and current project Members may read it.
- `PUT` on that assessment path is Admin-only, requires CSRF, and appends an immutable assessment. The body contains `verdict`, nonblank `remark_markdown`, and `expected_version`. Supported verdicts are `on_track`, `needs_attention`, `blocked`, and `complete`; remarks share the comment size bounds and return sanitized HTML. Use version zero for the first row and the current version thereafter. A stale value returns `409 conflict`; successful appends return `201`.
- `GET .../tasks/{task_id}/assessments` is Admin-only and returns complete history newest-first. A Member receives `403 forbidden` even when allowed to see the current assessment.

Every nested identifier is resolved through the trusted project/task/update scope; inaccessible and mismatched identifiers return scoped `404 not_found`. Inactive projects remain readable but reject new comments, suggestion acceptance, and assessments with `409 project_inactive`. Comment creation, first acceptance, and assessment append each commit their audit event in the same PostgreSQL transaction. See `../guides/REVIEW_WORKFLOW.md`.

## Dashboard, task timeline, and complete audit

- `GET /api/v1/dashboard` returns `totals` and one summary per authorized project. Admins see all projects; Members see current memberships only. Facts include active state, task/update/accepted-suggestion counts, latest progress server time, and counts of each task's latest assessment verdict. It deliberately does not label anything “needs progress” while that product rule remains undefined.
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/timeline` returns the complete oldest-first domain chronology to every authorized task viewer through bounded keyset pages. `limit` defaults to 100 and is bounded to 200; pass an opaque `next_cursor` back as `cursor` until it is absent. Malformed values return `422`. Events cover task/progress before-and-after revisions, attachment creation/state/downloads, comments, accepted suggestions, and assessment versions. Each event has stable source identity, action, entity, actor, server time, and typed metadata. `attachment.added` preserves its creation-time `pending` state. Markdown metadata includes sanitized HTML; security audit IP/request/user-agent context is excluded. See `../guides/TASK_TIMELINE.md`.
- `GET /api/v1/admin/audit` is Admin-only and covers the complete append-only security/business audit stream. `limit` defaults to 100 and is bounded to 200. Optional exact filters are `action`, `outcome`, `actor_user_id`, and `target_type`. `cursor` is the opaque `next_cursor` from a prior page; malformed filters/cursors return `422`. Results order newest-first and equal timestamps continue by immutable event UUID.

The existing `GET /api/v1/admin/audit/identity` remains a compatibility view of the newest 200 identity/authentication/account-authorization events. It is not the complete audit contract.

## `POST /api/v1/setup/bootstrap`

Creates the first Admin only when `BOOTSTRAP_TOKEN` is configured and no user exists. The body contains `bootstrap_token`, normalized `username`, `email`, and `password`. The service validates the guarded secret, username/email, and password, hashes the password with Argon2id, then atomically inserts the Admin and `identity.bootstrap_succeeded` audit row under a transaction-level advisory lock. Concurrent calls can create at most one first user.

Success is `201` with `{"user": ...}`. Validation is `422`; wrong setup secret is `403`; absent configuration or an existing user is `404`. The bootstrap secret and password are never logged, stored, or returned.

## `POST /api/v1/auth/login`

Accepts `identifier` and `password`. The normalized identifier and trusted client IP address one durable throttle bucket. Five failures in a 15-minute window block the pair for 15 minutes. Unknown-user password work uses a dummy Argon2id verifier to reduce timing disclosure.

On success, one transaction creates a new hashed session, updates `last_login_at`, clears the throttle bucket, and appends `auth.login_succeeded`. The response is `200` with user, `csrf_token`, and `expires_at`, plus the cookie. Failure is generic `401 invalid_credentials`; the system records `auth.login_failed` or `auth.login_throttled` without plaintext identifiers or secrets.

## `GET /api/v1/auth/session`

Authenticates the cookie against current PostgreSQL state and returns user, a re-derived CSRF token, and expiry. This is the authoritative browser recovery path after reload or process restart. Missing, expired, revoked, or disabled-user sessions return `401 unauthenticated`.

## `POST /api/v1/auth/logout`

With a valid session, requires `X-CSRF-Token`, then atomically revokes the session and appends `auth.logout_succeeded`; the cookie is expired. Missing/invalid CSRF is `403 csrf_invalid`. Without a valid session, it still expires the cookie and returns `200 {"logged_out":true}` so retry is safe.

## `GET /api/v1/health/live`

Purpose: prove that the Go process and HTTP transport can respond.

Flow:

```text
HTTP request -> method check -> static process response
```

It does not query PostgreSQL. Success is `200` with `{"status":"ok"}`. Other methods return `405` and `Allow: GET`.

## `GET /api/v1/health/ready`

Purpose: determine whether the instance may receive stateful application traffic.

Flow:

```text
HTTP request
-> method check
-> bounded PostgreSQL ping
-> migration ledger existence/checksum/current-state verification
-> 200 ready or 503 not_ready
```

Success is `200` with `{"status":"ready"}`. Any dependency or schema failure is `503` with `{"status":"not_ready"}`. Failures are safe to retry. Other methods return `405` and `Allow: GET`.

## API documentation routes

When `API_DOCS_ENABLED=true`:

- `GET /api/openapi/v1/openapi.yaml` serves the embedded authoritative schema with only its server-variable default resolved to the configured `BASE_PATH`.
- `GET /api/docs/` serves an embedded Swagger UI configured to read that route.
- `GET /api/docs` redirects to the canonical trailing-slash viewer route.

When false, none of these routes are registered and they return `404`. The toggle does not affect ordinary application or health routes.

With `BASE_PATH=/backend`, the same paths are externally `/backend/api/openapi/v1/openapi.yaml` and `/backend/api/docs/`. The served OpenAPI `basePath` server variable defaults to `/backend`, so Swagger executes against the mounted API without duplicating operation paths in the committed contract.

## Common transport behavior

Responses receive MIME-sniffing, frame, referrer, and content-security headers. Each request receives a random `X-Request-ID`; logs and audits correlate on it. Client IP comes from the socket unless the direct peer is loopback, in which case the first valid `X-Forwarded-For` address is trusted for Caddy. Unexpected handler panics are contained and mapped to `500`; details remain in structured logs. Request logs contain request ID, method, path, status, and duration but no request body or credentials.
