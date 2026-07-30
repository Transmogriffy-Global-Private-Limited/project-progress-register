# Read this and build the frontend

Status: Complete browser-integration handoff for the deployed backend

This is the one human-readable document a frontend engineer needs to build the complete Project Progress Register frontend. It covers the product model, browser transport, authentication, permissions, every request and response shape, all 43 HTTP operations, uploads, geolocation, concurrency, pagination, errors, recovery, and recommended screen data flows.

The machine-readable contract remains OpenAPI, but no other backend document is required to understand the integration described here.

## 1. Backend coordinates and hard boundaries

Production origin and prefix:

```text
https://ppr.transev.site/backend
```

Production JSON API root:

```text
https://ppr.transev.site/backend/api/v1
```

Production Swagger and raw schema are currently enabled:

```text
https://ppr.transev.site/backend/api/docs/
https://ppr.transev.site/backend/api/openapi/v1/openapi.yaml
```

Use root-relative URLs such as `/backend/api/v1/auth/session` in the deployed frontend. Do not hardcode the hostname into application logic. Local deployments may use an empty prefix, so centralize `API_BASE_PATH` and build routes as `${API_BASE_PATH}/api/v1/...`.

The backend intentionally has:

- no JWT or bearer token;
- no public registration;
- no WebSocket, SSE, event bus, or push contract;
- no CORS middleware;
- no multi-tenancy or tenant selector;
- no delete operation for projects, tasks, progress, comments, suggestions, or assessments;
- no Kanban/status/sprint/backlog model;
- no backend-defined “needs progress” rule;
- no public file URL or object-storage URL.

Deploy the frontend on the same origin or place the API behind the frontend's same-origin reverse proxy. A different browser origin is unsupported because there is no CORS contract. Every request that can use authentication must use `credentials: "include"`.

## 2. Product model and screen map

There are two global roles:

- `admin`: manages accounts and every project; may perform every Member action.
- `member`: sees only projects with current membership and may mutate only within the rules below.

The durable hierarchy is:

```text
Project
  -> current Members
  -> current Geofence
  -> Tasks
      -> Progress Updates
          -> immutable revisions
          -> Attachments
          -> immutable Comments
              -> optional separate Admin acceptance
      -> accepted Suggestions
      -> current Admin Assessment
      -> immutable Assessment history
      -> complete domain Timeline
```

Recommended frontend routes:

```text
/login
/change-password
/
/projects/:projectId
/projects/:projectId/tasks/:taskId
/projects/:projectId/tasks/:taskId/progress/new
/admin/users
/admin/audit
```

Recommended screen ownership:

- Login: identifier/password and generic credential errors.
- Forced password change: available immediately after login when `must_change_password=true`.
- Home: factual dashboard totals and authorized project summaries.
- Project: project header/geofence and task register; Admin-only project edit, membership, and geofence controls.
- Task: task details, current assessment, accepted suggestions, chronological progress, comments, attachments, and optional complete timeline.
- Add progress: Markdown, browser location, camera capture and/or existing files, upload progress, and retry state.
- Admin users: account list, create, role/enable update, password reset, one-time credential display.
- Admin audit: filtered, cursor-paginated complete audit stream.

## 3. Browser transport contract

### 3.1 Cookies and credentials

Successful login sets the opaque `ppr_session` cookie. JavaScript cannot and must not read it because it is `HttpOnly`. In production it is host-only, `Secure`, `SameSite=Lax`, and scoped to `/backend/`. PostgreSQL stores only a SHA-256 digest of the token.

Always send browser credentials:

```ts
fetch(url, { credentials: "include" })
```

Never copy session state into local storage, session storage, URLs, analytics, or logs.

### 3.2 CSRF

Login and session recovery return `csrf_token`. Hold it in memory. Every authenticated state-changing request sends:

```http
X-CSRF-Token: <current session csrf_token>
```

The token is bound to the current session. Recover a new token with `GET /auth/session` after reload. A missing or invalid value returns `403 csrf_invalid`.

Logout is special: it requires CSRF only while the submitted cookie identifies a valid session. Retrying logout after the session is gone still succeeds.

### 3.3 JSON requests

For JSON bodies:

```http
Content-Type: application/json
```

The backend rejects unknown fields, malformed JSON, multiple JSON values, and bodies over 64 KiB. Use UTF-8 JSON. Do not send derived HTML fields in mutation bodies.

### 3.4 Errors and request correlation

JSON errors have exactly this envelope:

```ts
interface APIErrorEnvelope {
  error: {
    code: string;
    message: string;
  };
}
```

Every response includes `X-Request-ID`. Capture it in client diagnostics and support reports, but do not display it as ordinary UI content.

The wrong HTTP method may return plain-text `405 Method Not Allowed`; normal application code should never depend on that response body.

### 3.5 Recommended API client

```ts
const API_BASE_PATH = "/backend"; // "" for an unprefixed local backend
let csrfToken: string | null = null;

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public requestId: string | null,
  ) {
    super(message);
  }
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit & { json?: unknown } = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  let body = init.body;

  if ("json" in init) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(init.json);
  }
  const method = (init.method ?? "GET").toUpperCase();
  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(`${API_BASE_PATH}${path}`, {
    ...init,
    body,
    headers,
    credentials: "include",
  });
  const requestId = response.headers.get("X-Request-ID");

  if (!response.ok) {
    const payload = await response.json().catch(() => null) as APIErrorEnvelope | null;
    throw new APIError(
      response.status,
      payload?.error.code ?? "http_error",
      payload?.error.message ?? `HTTP ${response.status}`,
      requestId,
    );
  }
  if (response.status === 204) return undefined as T;
  return await response.json() as T;
}
```

Do not use this JSON helper for multipart creation or attachment bytes; dedicated examples appear below.

## 4. Authentication state machine

At application startup:

```text
GET /auth/session
-> 200: store user, csrf_token, expires_at
   -> must_change_password=true: show only change-password/logout UI
   -> otherwise: enter the application
-> 401: clear in-memory auth and show login
```

After login:

```text
POST /auth/login
-> 200: cookie set; store returned user/csrf/expiry
-> user.must_change_password=true: route to /change-password
-> otherwise: route to the intended authenticated page
```

After password change, account update affecting the current user, password reset, disablement, expiry, or revocation, an existing cookie may stop authenticating. Treat `401 unauthenticated` from any protected endpoint as global logout: clear in-memory user/CSRF/cache and route to login. Do not blindly retry the same request.

Treat `403 password_change_required` as a forced transition to the password-change screen. Treat ordinary `403 forbidden` as a permission result, not a missing resource.

## 5. Authorization and UI capability matrix

The backend is always authoritative. The frontend should hide or disable impossible controls for clarity, but must still handle denial.

| Capability | Admin | Member |
|---|---:|---:|
| View dashboard | All projects | Current project memberships |
| Create/edit/deactivate project | Yes | No |
| Manage project Members/geofence | Yes | No |
| Read inactive authorized project | Yes | Yes |
| Create task in active project | Yes | Yes |
| Edit task | Any | Only immutable creator |
| Add progress in active project | Yes | Yes |
| Edit progress | Any | Only immutable author |
| View all progress/revisions/files in authorized project | Yes | Yes |
| Add comment in active project | Yes | Yes |
| Accept suggestion | Yes | No |
| View accepted suggestions/current assessment | Yes | Yes |
| Set assessment/read assessment history | Yes | No |
| View complete task timeline | Yes | Yes |
| Manage users/read audit | Yes | No |

Important distinctions:

- Responsible Members are display/work assignments only. Responsibility grants neither access nor edit ownership.
- Membership controls Member access. Removing membership revokes future reads immediately.
- Inactive projects remain readable but reject task/progress/review mutations with `409 project_inactive`.
- Unknown, mismatched, and inaccessible nested IDs intentionally collapse to `404 not_found`.
- Admin access is global and is not represented as a project membership row.

## 6. TypeScript data contract

All timestamps are RFC 3339 date-time strings with server/UTC semantics. IDs are UUID strings unless explicitly described otherwise. A date-only target is `YYYY-MM-DD`; do not convert it through a UTC `Date` and accidentally shift the calendar day.

```ts
type UUID = string;
type DateTime = string;
type CalendarDate = string;
type Role = "admin" | "member";
type Verdict = "on_track" | "needs_attention" | "blocked" | "complete";

interface User {
  id: UUID;
  username: string;
  email: string;
  role: Role;
  enabled: boolean;
  must_change_password: boolean;
  created_at: DateTime;
  updated_at: DateTime;
  version: number;
}

interface SessionResponse {
  user: User;
  csrf_token: string;
  expires_at: DateTime;
}

interface Geofence {
  id: UUID;
  version: number;
  latitude: number;
  longitude: number;
  radius_metres: number;
  max_accuracy_metres: number;
  valid_from: DateTime;
}

interface Project {
  id: UUID;
  name: string;
  description_markdown: string;
  description_html: string;
  active: boolean;
  created_by: UUID;
  created_at: DateTime;
  updated_at: DateTime;
  version: number;
  geofence: Geofence | null;
}

interface ProjectMember {
  user_id: UUID;
  username: string;
  email: string;
  enabled: boolean;
  added_at: DateTime;
}

interface Actor {
  user_id: UUID;
  username: string;
}

interface ResponsibleMember {
  user_id: UUID;
  username: string;
  enabled: boolean;
}

interface Task {
  id: UUID;
  project_id: UUID;
  name: string;
  goals_markdown: string;
  goals_html: string;
  description_markdown: string;
  description_html: string;
  created_by: Actor;
  responsible_members: ResponsibleMember[];
  target_date: CalendarDate | null;
  created_at: DateTime;
  updated_at: DateTime;
  version: number;
}

// Compatibility shape returned only by the retained V1 task endpoints.
interface LegacyTaskV1 extends Omit<Task, "responsible_members"> {
  responsible_member: ResponsibleMember | null;
}

interface ReportedLocation {
  latitude: number;
  longitude: number;
  accuracy_metres: number;
  browser_observed_at?: DateTime | null;
}

interface EvidenceGeofence {
  id: UUID;
  version: number;
  latitude: number;
  longitude: number;
  radius_metres: number;
  max_accuracy_metres: number;
}

type LocationStatus =
  | "verified"
  | "unverified_outside"
  | "unverified_accuracy"
  | "unverified_no_geofence"
  | "unverified_unavailable"
  | "not_supplied";

type LocationUnavailableReason =
  | "permission_denied"
  | "timeout"
  | "unavailable"
  | "not_supported";

interface ProgressEvidence {
  location_status: LocationStatus;
  location_reason: string;
  reported_location: ReportedLocation | null;
  location_unavailable_reason: LocationUnavailableReason | null;
  geofence: EvidenceGeofence | null;
  computed_distance_metres: number | null;
}

type MediaKind = "image" | "document" | "video";
type AttachmentSource = "camera" | "upload";
type VerificationStatus = "verified" | "non_verified";
type StorageState = "pending" | "available" | "failed";

interface ProgressAttachment {
  id: UUID;
  original_name: string;
  reported_mime: string;
  detected_mime: string;
  media_kind: MediaKind;
  source: AttachmentSource;
  source_trust: "browser_reported";
  verification_status: VerificationStatus;
  verification_reason: string;
  size_bytes: number;
  sha256: string;
  browser_last_modified_at: DateTime | null;
  embedded_metadata: Record<string, unknown>;
  embedded_metadata_trust: "untrusted";
  storage_state: StorageState;
  failure_reason: string;
  content_path: string;
  created_at: DateTime;
  available_at: DateTime | null;
}

interface ProgressRevision {
  id: UUID;
  from_version: number;
  to_version: number;
  previous_content_markdown: string;
  previous_content_html: string;
  new_content_markdown: string;
  new_content_html: string;
  edited_by: Actor;
  edited_at: DateTime;
}

interface ProgressUpdate {
  id: UUID;
  project_id: UUID;
  task_id: UUID;
  content_markdown: string;
  content_html: string;
  created_by: Actor;
  evidence: ProgressEvidence;
  attachments: ProgressAttachment[];
  revisions: ProgressRevision[]; // empty in list; complete in detail/mutation
  created_at: DateTime;
  updated_at: DateTime;
  version: number;
}

interface SuggestionAcceptance {
  id: UUID;
  accepted_by: Actor;
  accepted_at: DateTime;
}

interface Comment {
  id: UUID;
  progress_update_id: UUID;
  content_markdown: string;
  content_html: string;
  created_by: Actor;
  created_at: DateTime;
  accepted_suggestion: SuggestionAcceptance | null;
}

interface AcceptedSuggestion {
  id: UUID;
  comment_id: UUID;
  progress_update_id: UUID;
  task_id: UUID;
  content_markdown: string;
  content_html: string;
  comment_author: Actor;
  commented_at: DateTime;
  accepted_by: Actor;
  accepted_at: DateTime;
}

interface Assessment {
  id: UUID;
  task_id: UUID;
  version: number;
  verdict: Verdict;
  remark_markdown: string;
  remark_html: string;
  assessed_by: Actor;
  created_at: DateTime;
}

interface AssessmentCounts {
  on_track: number;
  needs_attention: number;
  blocked: number;
  complete: number;
}

interface DashboardProject {
  id: UUID;
  name: string;
  active: boolean;
  task_count: number;
  progress_update_count: number;
  accepted_suggestion_count: number;
  latest_progress_at: DateTime | null;
  current_assessments: AssessmentCounts;
}

interface Dashboard {
  totals: {
    project_count: number;
    active_project_count: number;
    inactive_project_count: number;
    task_count: number;
    progress_update_count: number;
    accepted_suggestion_count: number;
    current_assessments: AssessmentCounts;
  };
  projects: DashboardProject[];
}

type TimelineAction =
  | "task.created" | "task.updated"
  | "progress.created" | "progress.updated"
  | "attachment.added" | "attachment.available"
  | "attachment.failed" | "attachment.downloaded"
  | "comment.created" | "suggestion.accepted"
  | "assessment.created";

interface TimelineEvent {
  id: string; // stable source-derived ID, not necessarily a UUID
  action: TimelineAction;
  entity_type: string;
  entity_id: UUID;
  actor: Actor;
  occurred_at: DateTime;
  metadata: Record<string, unknown>;
}

interface AuditRecord {
  id: UUID;
  actor_user_id?: UUID;
  actor_username?: string;
  action: string;
  target_type: string;
  target_id?: UUID;
  outcome: "succeeded" | "failed" | "denied";
  occurred_at: DateTime;
  request_id: string;
  client_ip: string;
  user_agent: string;
  details: Record<string, unknown>;
}
```

Response envelopes are intentionally simple:

```ts
type UserResponse = { user: User };
type UsersResponse = { users: User[] };
type CredentialResponse = { user: User; temporary_password: string };
type ProjectResponse = { project: Project };
type ProjectsResponse = { projects: Project[] };
type MemberResponse = { member: ProjectMember };
type MembersResponse = { members: ProjectMember[] };
type GeofenceResponse = { geofence: Geofence };
type TaskResponse = { task: Task };
type TasksResponse = { tasks: Task[] };
type ProgressResponse = { progress_update: ProgressUpdate };
type ProgressListResponse = { progress_updates: ProgressUpdate[] };
type CommentResponse = { comment: Comment };
type CommentsResponse = { comments: Comment[] };
type AcceptedSuggestionsResponse = { accepted_suggestions: AcceptedSuggestion[] };
type AcceptanceMutationResponse = { accepted_suggestion: AcceptedSuggestion; created: boolean };
type CurrentAssessmentResponse = { assessment: Assessment | null };
type AssessmentResponse = { assessment: Assessment };
type AssessmentsResponse = { assessments: Assessment[] };
type TimelinePage = { timeline: TimelineEvent[]; next_cursor?: string };
type AuditPage = { audit_events: AuditRecord[]; next_cursor?: string };
```

## 7. Markdown and HTML rendering

Markdown source is the editable durable truth. The backend derives sanitized HTML:

- `description_markdown` -> `description_html`
- `goals_markdown` -> `goals_html`
- `content_markdown` -> `content_html`
- `remark_markdown` -> `remark_html`
- timeline keys ending `_markdown` -> sibling `_html`

For display, render only the backend `*_html` projection through the framework's explicit HTML mechanism. Never inject `*_markdown` as HTML. For editing, initialize controls from the Markdown source. Never submit an HTML projection back to the backend.

Limits:

- project description: 20,000 UTF-8 bytes;
- task goals: 20,000 UTF-8 bytes;
- task description/progress content: 50,000 UTF-8 bytes;
- comment/assessment remark: 10,000 Unicode characters and 20,000 UTF-8 bytes.

Client-side limits improve UX; backend validation is final.

## 8. Endpoint catalogue

Every `{..._id}` path value is a UUID. Every protected request includes the session cookie through `credentials: "include"`. Every protected mutation also includes `X-CSRF-Token` unless explicitly stated otherwise.

### 8.1 Setup and authentication

#### POST `/api/v1/setup/bootstrap` — `bootstrapAdmin`

Unauthenticated, one-time operational setup. Ordinary product FE should normally omit this screen because production has already bootstrapped and returns `404 bootstrap_unavailable`.

```ts
type BootstrapRequest = {
  bootstrap_token: string; // 24-256 characters
  username: string;        // ^[a-z0-9][a-z0-9._-]{2,31}$
  email: string;           // valid email, <=254 chars
  password: string;        // >=12 Unicode chars, <=128 UTF-8 bytes
};
```

Returns `201 { user }`. Also handle `400 invalid_request`, `403 bootstrap_denied`, `404 bootstrap_unavailable`, `415 unsupported_media_type`, and `422 validation_failed`. Concurrent attempts can create at most one Admin.

#### POST `/api/v1/auth/login` — `login`

```ts
type LoginRequest = { identifier: string; password: string };
```

Returns `200 SessionResponse` and sets the cookie. Username or email may be used as identifier. Unknown account, disabled account, wrong password, and throttling all deliberately return the same `401 invalid_credentials`. Five failures for one normalized-identifier/client-IP pair in 15 minutes block that pair for 15 minutes. Also handle `400`, `415`, and `422` as input/transport errors.

#### GET `/api/v1/auth/session` — `getCurrentSession`

Returns `200 SessionResponse`; use it on every full-page load. Returns `401 unauthenticated` for a missing, expired, revoked, or disabled-user session.

#### POST `/api/v1/auth/logout` — `logout`

No body. Send CSRF when authenticated. Returns `200 { logged_out: true }` and clears the cookie. Safe to repeat after the session is absent. An authenticated call with bad CSRF returns `403 csrf_invalid`.

#### POST `/api/v1/auth/password` — `changeOwnPassword`

Available to any authenticated user, including forced-change users.

```ts
type ChangePasswordRequest = { password: string }; // same 12-char/128-byte rule
```

Returns `200 { password_changed: true, logged_out: true }`, revokes every user session, and clears the cookie. Route immediately to login. Handle `400`, `401`, `403 csrf_invalid`, `415`, and `422`.

### 8.2 Admin account administration

All operations require `user.role === "admin"` and `must_change_password === false`.

#### GET `/api/v1/admin/users` — `listUsers`

Returns `200 { users: User[] }`, complete and username-ordered. There is no pagination.

#### POST `/api/v1/admin/users` — `createUser`

```ts
type CreateUserRequest = {
  username: string;
  email: string;
  role: "admin" | "member";
};
```

Returns `201 CredentialResponse` with `Cache-Control: no-store`. Display/copy `temporary_password` exactly once, do not persist it, and securely hand it to the user. The new account is enabled and forced to change password. `409 conflict` covers duplicate account identity. Also handle `400`, `401`, `403`, `415`, and `422`.

#### PATCH `/api/v1/admin/users/{user_id}` — `updateUser`

Send the complete mutable account state:

```ts
type UpdateUserRequest = {
  role: "admin" | "member";
  enabled: boolean;
  expected_version: number;
};
```

Returns `200 { user }`, increments version, and revokes all target sessions. `409 conflict` means stale state; reload. `409 last_admin` means the mutation would disable/demote the final enabled Admin. Returns `404` for an unknown target.

#### POST `/api/v1/admin/users/{user_id}/password-reset` — `resetUserPassword`

No body. Returns `200 CredentialResponse` with `Cache-Control: no-store`, increments the user version, revokes all target sessions, and forces replacement after next login. Treat the temporary password with the same one-display rule as account creation.

#### GET `/api/v1/admin/audit/identity` — `listIdentityAudit`

Returns `200 { audit_events: AuditRecord[] }`: newest 200 identity/authentication/account-authorization events, newest first. This compatibility view has no filters or pagination. Prefer the complete audit operation for the Admin audit screen.

#### GET `/api/v1/admin/audit` — `listCompleteAudit`

Query parameters:

```ts
interface AuditQuery {
  limit?: number; // default 100; 1-200
  cursor?: string;
  action?: string; // exact action, <=100
  outcome?: "succeeded" | "failed" | "denied";
  actor_user_id?: UUID;
  target_type?: string; // exact target type, <=50
}
```

Returns newest-first `AuditPage`. To continue, repeat the exact filters and pass `next_cursor` unchanged. Stop when it is absent. A new filter search starts without a cursor. Treat the cursor as opaque; never decode or construct it. Invalid filters/cursors return `422 validation_failed`.

### 8.3 Dashboard

#### GET `/api/v1/dashboard` — `getDashboard`

Returns `200 Dashboard`. Admin totals cover all projects; Member totals cover current memberships only. Inactive authorized projects remain included. `latest_progress_at` can be `null`.

The values are facts only. Do not label a project “needs progress” unless product ownership later defines that separate rule. Current assessment totals count only the latest assessment for each task.

### 8.4 Projects, membership, and geofence

#### GET `/api/v1/projects` — `listProjects`

Returns `200 { projects: Project[] }`, name-ordered and already authorization-filtered. Admin sees all; Member sees current memberships. `geofence` may be `null`.

#### POST `/api/v1/projects` — `createProject`

Admin-only.

```ts
type CreateProjectRequest = {
  name: string;                 // trimmed nonblank, <=120
  description_markdown: string; // <=20,000 UTF-8 bytes
};
```

Returns `201 { project }`. The project begins active, at version 1, with no Members and no geofence.

#### GET `/api/v1/projects/{project_id}` — `getProject`

Returns `200 { project }`. Unknown and inaccessible IDs both return `404 not_found`.

#### PATCH `/api/v1/projects/{project_id}` — `updateProject`

Admin-only; send all mutable fields:

```ts
type UpdateProjectRequest = {
  name: string;
  description_markdown: string;
  active: boolean;
  expected_version: number;
};
```

Returns `200 { project }`. `409 conflict` means reload current project/version. Deactivation preserves all history and read access; there is no delete.

#### GET `/api/v1/projects/{project_id}/members` — `listProjectMembers`

Admin-only. Returns `200 { members: ProjectMember[] }`, username-ordered. Disabled assigned accounts remain visible with `enabled: false`.

#### PUT `/api/v1/projects/{project_id}/members/{user_id}` — `addProjectMember`

Admin-only, no body. Target must currently be an enabled `member` account. Returns `201 { member }`. `409 conflict` means current membership already exists. `422 invalid_member` means the account is disabled or is not a Member.

#### DELETE `/api/v1/projects/{project_id}/members/{user_id}` — `removeProjectMember`

Admin-only, no body. Returns `204` with no body. Access ends immediately. The user is removed from every task assignment set; other assignees remain and each affected task version increments once, so refresh project tasks after success.

#### PUT `/api/v1/projects/{project_id}/geofence` — `replaceProjectGeofence`

Admin-only.

```ts
type ReplaceGeofenceRequest = {
  latitude: number;            // -90..90
  longitude: number;           // -180..180
  radius_metres: number;       // 1..100000
  max_accuracy_metres: number; // 0.1..10000
  expected_version: number;    // 0 if geofence is null, else geofence.version
};
```

Returns `200 { geofence }`. Replacement creates a new immutable policy version; old progress evidence keeps its original snapshot. `409 conflict` means refetch the project and retry from the new current version.

### 8.5 Tasks

Use V2 for all new frontend task work. V1 remains only so the existing singular-assignment integration can migrate without an immediate contract break.

For a copy-paste-oriented migration walkthrough focused on plural, editable task assignments, see [FRONTEND_TASK_RESPONSIBILITIES_V2.md](FRONTEND_TASK_RESPONSIBILITIES_V2.md).

#### GET `/api/v2/projects/{project_id}/tasks` — `listProjectTasksV2`

Returns `200 { tasks: Task[] }`, name-ordered. Inactive projects remain readable.

#### POST `/api/v2/projects/{project_id}/tasks` — `createTaskV2`

Admin or current Member; active project required.

```ts
type CreateTaskRequest = {
  name: string;                       // nonblank, <=160
  goals_markdown: string;             // <=20,000 UTF-8 bytes
  description_markdown: string;       // <=50,000 UTF-8 bytes
  responsible_user_ids?: UUID[] | null; // unique; omit, null, or [] for none
  target_date?: CalendarDate | null;   // omit or null for none
};
```

Returns `201 { task }`. The authenticated user becomes immutable creator. Every supplied ID must be a current enabled project Member. Duplicate IDs return `422 validation_failed`; an ineligible ID returns `422 invalid_responsible_member`.

#### GET `/api/v2/projects/{project_id}/tasks/{task_id}` — `getProjectTaskV2`

Returns `200 { task }`. Unknown, wrong-parent, and inaccessible identifiers all return `404`.

#### PATCH `/api/v2/projects/{project_id}/tasks/{task_id}` — `updateProjectTaskV2`

Admin or immutable Member creator; active project required. Send every mutable field and explicit nulls:

```ts
type UpdateTaskRequest = {
  name: string;
  goals_markdown: string;
  description_markdown: string;
  responsible_user_ids: UUID[]; // complete desired set; [] removes all
  target_date: CalendarDate | null;
  expected_version: number;
};
```

Returns `200 { task }` and increments version. `409 conflict` means refetch before allowing another save. A non-owner Member receives scoped `404`, not `403`.

Assignment replacement is atomic with the other task fields, immutable revision, version increment, and audit event. `responsible_members` is always the complete array, ordered by case-insensitive username then user ID. Assignment never grants access or edit ownership.

V1 compatibility operations remain documented and callable:

- GET `/api/v1/projects/{project_id}/tasks` — `listProjectTasks`
- POST `/api/v1/projects/{project_id}/tasks` — `createTask`
- GET `/api/v1/projects/{project_id}/tasks/{task_id}` — `getProjectTask`
- PATCH `/api/v1/projects/{project_id}/tasks/{task_id}` — `updateProjectTask`

They retain `responsible_user_id?: UUID | null` on create, required nullable `responsible_user_id` on update, and `responsible_member: ResponsibleMember | null` in `LegacyTaskV1`. If V2 has assigned multiple Members, V1 reads expose only the deterministic first compatibility member and V1 update returns `409 task_v2_required`; it never silently removes the hidden assignments. Do not build new screens against V1.

### 8.6 Progress, geolocation, and attachments

For a copy-paste-oriented direct camera video integration walkthrough, see [FRONTEND_DIRECT_CAMERA_VIDEO.md](FRONTEND_DIRECT_CAMERA_VIDEO.md).

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates` — `listTaskProgressUpdates`

Returns `200 { progress_updates: ProgressUpdate[] }`, oldest first. Every item includes current content, evidence, and attachments, but `revisions` is intentionally `[]`. Fetch detail only when revision history is needed.

#### POST `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates` — `createTaskProgressUpdate`

Admin or current Member; active project required. This is `multipart/form-data`, not JSON. Send both `Idempotency-Key` and CSRF. Do not manually set `Content-Type`; the browser must add the multipart boundary.

Metadata contract:

```ts
interface CreateProgressMetadata {
  content_markdown: string; // nonblank, <=50,000 UTF-8 bytes
  location?: ReportedLocation | null;
  location_unavailable_reason?: LocationUnavailableReason | null;
  attachments: Array<{
    source: "camera" | "upload";
    browser_last_modified_at?: DateTime | null;
  }>;
}
```

`attachments.length` must exactly equal the number of repeated `files` parts and preserve the same order.

```ts
async function createProgress(
  projectId: UUID,
  taskId: UUID,
  metadata: CreateProgressMetadata,
  files: File[],
  idempotencyKey: string,
): Promise<ProgressUpdate> {
  const form = new FormData();
  form.append("metadata", JSON.stringify(metadata));
  for (const file of files) form.append("files", file, file.name);

  const response = await fetch(
    `${API_BASE_PATH}/api/v1/projects/${projectId}/tasks/${taskId}/progress-updates`,
    {
      method: "POST",
      credentials: "include",
      headers: {
        "X-CSRF-Token": csrfToken!,
        "Idempotency-Key": idempotencyKey,
      },
      body: form,
    },
  );
  // Parse success/error with the same envelope rules as apiFetch.
  return (await response.json() as ProgressResponse).progress_update;
}
```

The metadata part must be a normal text field. Do not append a `Blob` or `File`: browsers add a filename to those parts, the backend correctly classifies them as file parts, and only the repeated `files` field may contain files.

Generate one unpredictable 16-128 character idempotency key per logical submit, for example `crypto.randomUUID()`. Keep it with the pending draft until success or an intentional discard. A network error or `503 attachment_pending` retries the byte-identical metadata and files with the same key. A changed draft gets a new key. Reusing a key with changed metadata/file hashes returns `409 conflict`.

Location rules:

- Files present: `location` is mandatory and `location_unavailable_reason` must be null/absent. If geolocation cannot be obtained, do not send files; let the user retry location or submit text-only.
- Text only: location may be supplied; otherwise use one unavailable reason or omit both for `not_supplied`.
- Browser coordinates must be finite; accuracy must be positive; browser time may be null.
- Outside, inaccurate, and no-geofence coordinates do not reject files. They become non-verified evidence.
- Only an image or video produced by the distinct in-app camera flow should send `source: "camera"`. Gallery/existing images and videos plus all documents send `source: "upload"`.
- Even camera source is browser-reported, not hardware attestation.
- Display the returned `location_status`, `location_reason`, `verification_status`, and `verification_reason`; never calculate or infer the official label client-side.

Suggested browser capture inputs:

```html
<!-- Separate camera flow -->
<input type="file" accept="image/*" capture="environment">

<!-- Separate direct video-camera flow -->
<input type="file" accept="video/*" capture="environment">

<!-- Existing files -->
<input type="file" multiple>
```

Allowed server-detected formats:

- images: JPEG, PNG, GIF, WebP;
- documents: PDF, TXT, Markdown, CSV, DOC/DOCX, XLS/XLSX, PPT/PPTX, ODT/ODS/ODP;
- videos: MP4, WebM, MOV, AVI.

For a direct camera video, append the returned `File` in the same ordered `files` list and append the matching `{ source: "camera" }` descriptor at the same index. A qualifying direct camera photo or video is `verified` only when the backend also returns `location_status: "verified"`; otherwise it is accepted as `non_verified`. Never mark a gallery-selected video as camera source.

The configured defaults are 10 files and 100 MiB per file; hard backend ranges are 1-25 files and 1 MiB-1 GiB per file. There is currently no public runtime-configuration endpoint. Use conservative client limits and always handle `413 request_too_large` and `422 validation_failed`.

Returns `201 { progress_update }` both for initial creation and an identical successful retry. Other important outcomes are `400 invalid_request`, `409 conflict`, `413 request_too_large`, `415 unsupported_media_type`, `422 validation_failed`, and `503 attachment_pending`.

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}` — `getTaskProgressUpdate`

Returns `200 { progress_update }` with complete revision history oldest by version. Use after opening history or after a mutation when authoritative current state is needed.

#### PATCH `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}` — `updateTaskProgress`

Admin or immutable Member author; active project required.

```ts
type UpdateProgressRequest = {
  content_markdown: string;
  expected_version: number;
};
```

Returns `200 { progress_update }` with an appended before/after revision. Evidence and attachments cannot be changed. `409 conflict` means reload detail.

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/attachments/{attachment_id}/content` — `downloadProgressAttachment`

The attachment response already supplies `content_path`; prefer it instead of rebuilding this nested path. It includes `/backend` in production.

For images/documents and explicit saves, download with authenticated `fetch`, turn the successful response into a Blob, create a temporary object URL, click a temporary `<a download>`, then revoke the URL. Their response uses server-detected MIME, `Content-Disposition: attachment`, `Cache-Control: private, no-store`, and `nosniff`.

For video playback, use the authorized same-origin `content_path` directly as the `<video controls preload="metadata">` source. The browser includes the same-origin session cookie and uses range requests for seeking; the backend responds with detected video MIME, `Content-Disposition: inline`, `Accept-Ranges: bytes`, and `206 Partial Content` when a range is requested. Do not fetch the entire video into a Blob before playback. A `401`, `404`, `410`, or `503` from the media element should offer session recovery/refetch/retry rather than exposing storage details.

State handling:

- `storage_state=available`: enable download.
- `storage_state=pending`: disable or show retrying; content returns `503 attachment_pending`.
- `storage_state=failed`: disable and show generic failure; content returns `410 attachment_unavailable`.
- `404`: resource is absent or no longer authorized.

Never render uploaded bytes inline as trusted HTML and never use original filename or `reported_mime` as a security decision.

### 8.7 Comments, suggestions, and assessments

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/comments` — `listProgressComments`

Returns `200 { comments: Comment[] }`, oldest first. `accepted_suggestion` is either the separate acceptance badge or `null`.

#### POST `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/comments` — `createProgressComment`

Admin or current Member; active project required.

```ts
type CreateCommentRequest = { content_markdown: string };
```

Returns `201 { comment }`. Comments are immutable: there is no edit/delete endpoint.

#### POST `/api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/comments/{comment_id}/accept` — `acceptProgressCommentSuggestion`

Admin-only, no body, active project required, idempotent. First success is `201 { accepted_suggestion, created: true }`; a retry is `200` with the same record and `created: false`. Treat both as success and replace local comment/suggestion state. No duplicate audit record is created.

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/accepted-suggestions` — `listTaskAcceptedSuggestions`

Returns `200 { accepted_suggestions: AcceptedSuggestion[] }`, oldest by acceptance time. Available to Admin and authorized Member, including inactive projects.

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/assessment` — `getCurrentTaskAssessment`

Returns `200 { assessment: Assessment | null }`. `null` is the ordinary state before the first assessment; do not treat it as `404`.

#### PUT `/api/v1/projects/{project_id}/tasks/{task_id}/assessment` — `setCurrentTaskAssessment`

Admin-only, active project required.

```ts
type SetAssessmentRequest = {
  verdict: "on_track" | "needs_attention" | "blocked" | "complete";
  remark_markdown: string;
  expected_version: number; // 0 when current assessment is null
};
```

Returns `201 { assessment }` and appends a new immutable version. For later changes use the latest assessment's `version`. On `409 conflict`, refetch current assessment before another save.

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/assessments` — `listTaskAssessmentHistory`

Admin-only. Returns `200 { assessments: Assessment[] }`, newest version first. Members must use only the current-assessment endpoint and receive `403` here.

### 8.8 Task timeline

#### GET `/api/v1/projects/{project_id}/tasks/{task_id}/timeline` — `getTaskTimeline`

Admin or authorized Member. Query:

```ts
type TimelineQuery = { limit?: number; cursor?: string }; // default 100, max 200
```

Returns `TimelinePage`, oldest first. Pass `next_cursor` unchanged to continue and stop when absent. Treat it as opaque. To refresh the entire chronology, start again without a cursor. A malformed value returns `422`.

Timeline metadata by action:

| Action | Metadata |
|---|---|
| `task.created` | `version`, `name`, `goals_markdown/html`, `description_markdown/html`, complete `responsible_user_ids`, singular compatibility `responsible_user_id`, `target_date` |
| `task.updated` | `from_version`, `to_version`, `change_reason`, `before`, `after`; both snapshots contain name, Markdown/HTML, complete responsible ID arrays, singular compatibility ID, target date |
| `progress.created` | `version`, `content_markdown/html`, `location_status`, `location_reason`, `reported_location` |
| `progress.updated` | `from_version`, `to_version`, previous/new Markdown and HTML |
| `attachment.added` | progress ID, name, detected MIME, media kind, source, verification, size, SHA-256, historical `storage_state="pending"`, empty failure reason |
| `attachment.available/failed/downloaded` | progress ID, lifecycle storage state, failure reason |
| `comment.created` | progress ID and content Markdown/HTML |
| `suggestion.accepted` | comment ID, progress ID, content Markdown/HTML |
| `assessment.created` | version, verdict, remark Markdown/HTML |

This is the user-facing domain chronology. It intentionally omits client IP, user agent, request ID, login activity, and unrelated security audit context.

### 8.9 Health

#### GET `/api/v1/health/live` — `getLiveness`

Unauthenticated. Returns `200 { status: "ok" }` when the process can serve HTTP. It does not prove PostgreSQL or schema readiness. Product UI normally does not need to poll it.

#### GET `/api/v1/health/ready` — `getReadiness`

Unauthenticated. Returns `200 { status: "ready" }` when PostgreSQL is reachable and all migration checks pass; otherwise `503 { status: "not_ready" }`. Use for an explicit service-unavailable page or operational diagnostics, not as a substitute for session recovery.

## 9. Error handling policy

Handle by `error.code`, with status as the transport class:

| Status | Code | Frontend behavior |
|---:|---|---|
| 400 | `invalid_request` | Request construction bug or malformed multipart/JSON; preserve form and show a safe error. |
| 401 | `invalid_credentials` | Login-only generic failure; never claim an account exists or is disabled/throttled. |
| 401 | `unauthenticated` | Clear auth/query state and route to login. |
| 403 | `csrf_invalid` | Recover session once for a fresh CSRF token; do not auto-replay non-idempotent commands without user-safe retry semantics. |
| 403 | `password_change_required` | Route to forced password change. |
| 403 | `forbidden` | Hide/disable the denied action after refreshing user/resource state. |
| 403 | `bootstrap_denied` | Setup secret rejected; operational setup only. |
| 404 | `not_found` | Resource absent, wrong parent, or outside authorized scope; do not distinguish. |
| 404 | `bootstrap_unavailable` | Setup is disabled/already complete. |
| 409 | `conflict` | Reload current resource; for idempotency misuse, create a new logical submission/key only after user review. |
| 409 | `last_admin` | Explain that one enabled Admin must remain. |
| 409 | `project_inactive` | Keep page readable; disable mutation controls and refresh project. |
| 409 | `task_v2_required` | A legacy V1 edit encountered multiple assignments; reload and edit through the V2 task contract. |
| 410 | `attachment_unavailable` | Attachment metadata remains but bytes cannot be downloaded. |
| 413 | `request_too_large` | Keep draft; ask user to reduce files/size. |
| 415 | `unsupported_media_type` | Fix request content type or reject unsupported selected file. |
| 422 | `validation_failed` | Show backend message near the relevant field when safely mappable. |
| 422 | `invalid_member` | Refresh accounts/members; target is not an enabled Member. |
| 422 | `invalid_responsible_member` | Refresh project Members and responsibility selection. |
| 500 | `internal_error` | Preserve safe draft state; show retry/support UI with request ID. |
| 503 | `attachment_pending` | Retry identical upload with the same idempotency key or wait for reconciliation. |

Only retry GET requests automatically. Explicitly idempotent safe mutation retries are logout, suggestion acceptance, and an identical progress-create request with the same key and files. For other mutations, require user intent or authoritative refetch before retry.

## 10. Screen-level fetch and invalidation recipes

### App shell

1. Recover session.
2. Route forced-password users before loading business data.
3. Fetch dashboard after authentication.
4. On any protected `401`, clear all cached protected data.

### Home

1. `GET /dashboard`.
2. Navigate using returned project IDs.
3. Display factual counts only; do not invent backend status.

### Project

Fetch in parallel:

- `GET /projects/{project_id}`
- `GET /api/v2/projects/{project_id}/tasks`
- Admin only: `GET /projects/{project_id}/members`

After project update/geofence change, replace the project from the mutation response. After membership removal, refetch members and tasks because responsibility may have been cleared/versioned.

### Task

Fetch in parallel:

- V2 task detail (`GET /api/v2/projects/{project_id}/tasks/{task_id}`);
- progress list;
- accepted suggestions;
- current assessment;
- timeline first page when the chronology panel is visible;
- Admin-only assessment history when its panel is visible.

Comments are per progress update; load them when an update card or discussion section expands, or in bounded parallel requests for the currently visible entries. There is no combined comments endpoint.

After progress creation/edit, comment creation, acceptance, or assessment append, replace returned entities and refetch the affected aggregate views: progress/comments, accepted suggestions/current assessment, and timeline as applicable.

### Admin users

Load the complete account list. Keep each row's `version` with edit state. After create/update/reset, merge the returned user. Show a one-time credential modal that cannot be reopened from backend data.

### Admin audit and task timeline

Keep pages in order and deduplicate by immutable `id`. Never reuse a cursor with different filters. If a long-lived page needs authoritative freshness, discard old pages and restart from the first page rather than trying to splice new events into an old cursor session.

## 11. Optimistic concurrency

The following writes require the latest server version:

- account update -> `user.version`;
- project update -> `project.version`;
- geofence replace -> `project.geofence?.version ?? 0`;
- task update -> `task.version`;
- progress update -> `progress_update.version`;
- assessment append -> `assessment?.version ?? 0`.

On `409 conflict`, do not overwrite silently. Refetch, show the updated state, preserve the user's unsaved text locally where safe, and let the user deliberately reconcile.

## 12. Evidence labels the UI must preserve

Evidence is operational, not cryptographic. Never use wording such as “tamper-proof,” “proof of presence,” “verified by GPS,” or “court-grade.”

Truth levels:

- server authoritative: receipt/edit times, actor, SHA-256, detected MIME, selected geofence snapshot, computed distance, result;
- browser reported: coordinates, accuracy, browser time, and camera-flow assertion;
- untrusted: embedded metadata/EXIF and original filename/MIME;
- verified attachment: direct camera-source image or video plus server-verified location;
- non-verified attachment: every uploaded existing image/document/video and any direct camera image/video whose location does not pass.

Every file still has the update's reported upload geotag even when non-verified. Display per-file backend results; do not apply the update's location label to every file as if it were attachment verification.

## 13. Ordering, realtime, and recovery

There is no realtime transport. PostgreSQL is authoritative and normal recovery is refetching REST resources:

- projects/tasks: server-defined name ordering;
- progress/comments/accepted suggestions: oldest first;
- assessment history/audit: newest first;
- timeline: oldest first across cursor pages.

Polling is optional product behavior, not a backend requirement. Use conservative visibility/focus refreshes rather than aggressive intervals. After reconnect or process restart, recover the session and refetch current resources; no client event replay is required.

## 14. Frontend completion checklist

- [ ] Central API base path supports `/backend` and an empty local prefix.
- [ ] Every protected fetch uses `credentials: "include"`.
- [ ] CSRF lives only in memory and is attached to authenticated mutations.
- [ ] Startup always recovers the server session.
- [ ] Forced-password users can access only password change/logout.
- [ ] Role and ownership controls match the capability matrix.
- [ ] All JSON mutations send only accepted fields and explicit nullable fields where required.
- [ ] Markdown is edited as source and displayed from backend sanitized HTML.
- [ ] Optimistic versions are retained and `409` triggers reconciliation.
- [ ] Camera and existing-upload inputs remain separate.
- [ ] Every file submission obtains current browser coordinates before upload.
- [ ] Multipart metadata descriptors exactly match repeated files order.
- [ ] Progress retry retains the same key, metadata, and file bytes.
- [ ] Attachment downloads use `content_path`, cookies, and binary handling.
- [ ] Backend evidence/verification labels are displayed without stronger claims.
- [ ] Audit and timeline cursors are opaque and filters remain stable across pages.
- [ ] Protected `401` clears all private client state.
- [ ] One-time credentials are never persisted or recoverably displayed later.
- [ ] No UI invents unsupported project/task statuses, deletion, or “needs progress.”
- [ ] New task screens use V2 and retain the complete `responsible_user_ids` replacement set.
- [ ] Every one of the 43 operation IDs in this guide has an integration call or an intentional operator-only exclusion.
