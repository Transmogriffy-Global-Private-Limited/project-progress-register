# Frontend integration — multiple task responsibilities V2

Status: Deployed and available in production

This is the focused frontend migration guide for assigning zero or more responsible Members to a task. It supplements the complete [`FRONTEND_INTEGRATION.md`](FRONTEND_INTEGRATION.md) handoff; the authoritative machine-readable contract remains [`api/openapi/v1/openapi.yaml`](../../api/openapi/v1/openapi.yaml).

## 1. What changed

The original V1 task contract represents at most one responsible Member:

```ts
responsible_user_id: UUID | null;
responsible_member: ResponsibleMember | null;
```

The additive V2 task contract represents the complete assignment set:

```ts
responsible_user_ids: UUID[];
responsible_members: ResponsibleMember[];
```

Only task list, create, detail, and update are versioned. Authentication, projects, memberships, progress updates, comments, assessments, timeline, dashboard, and audit endpoints remain under `/api/v1`.

Use V2 for every new task screen. V1 remains callable only to keep an existing singular-assignment integration working during migration.

## 2. Deployed routes

| Operation ID | Method | Path | Purpose |
| --- | --- | --- | --- |
| `listProjectTasksV2` | GET | `/api/v2/projects/{project_id}/tasks` | List tasks with every responsible Member |
| `createTaskV2` | POST | `/api/v2/projects/{project_id}/tasks` | Create a task and its initial assignment set |
| `getProjectTaskV2` | GET | `/api/v2/projects/{project_id}/tasks/{task_id}` | Read one task with every responsible Member |
| `updateProjectTaskV2` | PATCH | `/api/v2/projects/{project_id}/tasks/{task_id}` | Atomically replace all mutable fields and assignments |

Production uses the `/backend` deployment prefix:

```text
https://ppr.transev.site/backend/api/v2/...
```

Frontend code should still construct root-relative URLs from one configurable prefix:

```ts
export const API_BASE_PATH = "/backend"; // use "" against an unprefixed local backend
```

The deployed contract is `https://ppr.transev.site/backend/api/openapi/v1/openapi.yaml`.

## 3. Authentication and transport

V2 uses the existing opaque session cookie. It does not introduce JWT authentication.

Every protected request must include `credentials: "include"`. Create and update additionally require `Content-Type: application/json` and the current session-bound `X-CSRF-Token` returned by login or session recovery.

Keep CSRF only in memory. Recover it after reload through `GET /api/v1/auth/session`. Never place session or CSRF values in browser storage, URLs, logs, or analytics.

## 4. TypeScript contracts

```ts
type UUID = string;
type DateTime = string;
type CalendarDate = string; // YYYY-MM-DD

interface TaskActor {
  user_id: UUID;
  username: string;
}

interface ResponsibleMember {
  user_id: UUID;
  username: string;
  enabled: boolean;
}

interface TaskV2 {
  id: UUID;
  project_id: UUID;
  name: string;
  goals_markdown: string;
  goals_html: string;
  description_markdown: string;
  description_html: string;
  created_by: TaskActor;
  responsible_members: ResponsibleMember[];
  target_date: CalendarDate | null;
  created_at: DateTime;
  updated_at: DateTime;
  version: number;
}

interface CreateTaskV2Request {
  name: string;
  goals_markdown: string;
  description_markdown: string;
  responsible_user_ids?: UUID[] | null;
  target_date?: CalendarDate | null;
}

interface UpdateTaskV2Request {
  name: string;
  goals_markdown: string;
  description_markdown: string;
  responsible_user_ids: UUID[];
  target_date: CalendarDate | null;
  expected_version: number;
}

type TaskV2Response = { task: TaskV2 };
type TasksV2Response = { tasks: TaskV2[] };
```

`responsible_members` is always an array, never optional or nullable. The backend orders it by case-insensitive username and then user ID.

## 5. Assignment and authorization rules

- A task may have zero, one, or many responsible Members.
- Submitted IDs must be unique current enabled Members of the same project.
- Responsibility is work assignment only; it grants neither project access nor task-edit permission.
- The immutable task creator remains `created_by`.
- Admins may edit any task.
- A Member may edit only a task they created while retaining current project membership.
- The project must be active for create/update but remains readable when inactive.
- Membership removal automatically removes only that user from affected assignments and versions those tasks.

Never derive authorization from `responsible_members`.

## 6. Loading assignment candidates

Admins can load current project memberships from `GET /api/v1/projects/{project_id}/members`:

```ts
interface ProjectMember {
  user_id: UUID;
  username: string;
  email: string;
  enabled: boolean;
  added_at: DateTime;
}

type ProjectMembersResponse = { members: ProjectMember[] };
```

For an assignment picker, retain only `enabled === true`. The backend revalidates every ID during save, so still handle state changing after this list loads.

### Current Member-role limitation

The membership-list endpoint is Admin-only. A non-Admin Member creator cannot currently discover every eligible assignment candidate from a public backend query.

Safe FE behavior today:

- Admin editor: show the complete multi-select from the membership endpoint.
- Member creator: create unassigned unless candidate IDs come from another trusted product flow.
- Member editing an existing task: initialize from `task.responsible_members`; preserving or removing those visible IDs is safe.
- Never expose an Admin-fetched directory in a Member session, invent IDs, or infer them from usernames.

A future Member-readable candidate endpoint is needed before Members can independently add arbitrary new assignees through a complete picker.

## 7. Create

Create accepts omitted, `null`, or `[]` as unassigned:

```ts
async function createTaskV2(
  projectId: UUID,
  input: CreateTaskV2Request,
): Promise<TaskV2> {
  const response = await apiFetch<TaskV2Response>(
    `/api/v2/projects/${projectId}/tasks`,
    { method: "POST", json: input },
  );
  return response.task;
}
```

Example:

```json
{
  "name": "Foundation inspection",
  "goals_markdown": "Confirm completed foundation work.",
  "description_markdown": "Upload evidence in the progress timeline.",
  "responsible_user_ids": [
    "4bb1f319-5706-4e87-bbfc-d9f9f5d4e32e",
    "8d3ac3a3-aea7-40ea-8fd6-3de66846c096"
  ],
  "target_date": "2026-08-15"
}
```

Success returns `201 { task: TaskV2 }` at version `1`. Replace optimistic state with the response; never synthesize `responsible_members` from selected IDs.

## 8. List and detail

```ts
async function listTasksV2(projectId: UUID): Promise<TaskV2[]> {
  return (await apiFetch<TasksV2Response>(
    `/api/v2/projects/${projectId}/tasks`,
  )).tasks;
}

async function getTaskV2(projectId: UUID, taskId: UUID): Promise<TaskV2> {
  return (await apiFetch<TaskV2Response>(
    `/api/v2/projects/${projectId}/tasks/${taskId}`,
  )).task;
}
```

The list is name-ordered. Refetch authoritative state after conflicts, membership changes, and account-state changes.

## 9. Update

V2 update is complete replacement, not a partial JSON merge. Send every mutable field, the full desired assignment array, explicit nullable date, and the displayed version.

```ts
function updateInputFromTask(
  task: TaskV2,
  selectedIds: UUID[],
): UpdateTaskV2Request {
  return {
    name: task.name,
    goals_markdown: task.goals_markdown,
    description_markdown: task.description_markdown,
    responsible_user_ids: [...new Set(selectedIds)],
    target_date: task.target_date,
    expected_version: task.version,
  };
}

async function updateTaskV2(
  projectId: UUID,
  taskId: UUID,
  input: UpdateTaskV2Request,
): Promise<TaskV2> {
  return (await apiFetch<TaskV2Response>(
    `/api/v2/projects/${projectId}/tasks/${taskId}`,
    { method: "PATCH", json: input },
  )).task;
}
```

Use `[]` to remove all assignments. Never send `null` or omit `responsible_user_ids` on update.

One successful update atomically validates access/version/assignments, replaces all mutable fields and assignments, increments version once, appends one immutable revision, records audit, and returns the authoritative task.

Do not implement add/remove as concurrent PATCH calls. Modify one local `Set<UUID>` and submit the final array once.

## 10. Conflict recovery

`409 conflict` means another command changed the task version.

1. Preserve the unsaved draft in memory.
2. Fetch current V2 task detail.
3. Show that server state changed.
4. Let the user deliberately reapply or discard their draft.
5. Submit using the new version and complete assignment set.

Never silently overwrite or automatically replay this PATCH.

## 11. Errors

| Status | Code | FE behavior |
| --- | --- | --- |
| 400 | `invalid_request` | Fix malformed JSON/unexpected fields; retain draft. |
| 401 | `unauthenticated` | Clear auth/query state and route to login. |
| 403 | `csrf_invalid` | Recover session token and require a safe user retry. |
| 403 | `forbidden` | Refresh user/project capabilities. |
| 404 | `not_found` | Treat absent, wrong-parent, inaccessible, and non-owner IDs identically. |
| 409 | `conflict` | Refetch V2 detail and reconcile. |
| 409 | `project_inactive` | Keep readable; disable mutations. |
| 409 | `task_v2_required` | V1-only: switch edit flow to V2 and refetch. |
| 415 | `unsupported_media_type` | Send JSON content type. |
| 422 | `validation_failed` | Show the backend message near the safe field. |
| 422 | `invalid_responsible_member` | Refresh candidates/task; at least one ID is ineligible. |
| 500 | `internal_error` | Preserve draft and show retry/support with request ID. |

An invalid responsible Member rejects the whole command; no partial assignment change commits.

## 12. V1 compatibility and rollout

V1 retains singular `responsible_user_id` and `responsible_member`. When V2 assigns multiple Members, V1 reads expose only one deterministic compatibility member and V1 update returns `409 task_v2_required` instead of discarding hidden assignments.

Recommended rollout:

1. Add V2 types without changing unrelated V1 types.
2. Move task list/detail queries to V2.
3. Initialize selection from all `responsible_members[].user_id` values.
4. Move task create/update to V2.
5. Add conflict and invalid-candidate recovery.
6. Remove task-screen reads of singular `responsible_member`.
7. Keep unrelated `/api/v1` URLs unchanged.
8. Temporarily retain `task_v2_required` recovery until every deployed editor uses V2.

Never globally replace `/api/v1` with `/api/v2`; only these four task operations are versioned.

## 13. Display guidance

- Show zero assignments as “Unassigned”.
- Key selection by `user_id`, never username.
- Render backend usernames from `responsible_members`.
- Preserve backend ordering for ordinary read-only display.
- Show disabled historical assignees distinctly when returned.
- Disable mutations on inactive projects.
- Determine Member edit controls from creator identity plus current project access, never assignment.
- Edit `*_markdown` and display only backend-sanitized `*_html`.

## 14. Timeline

The canonical endpoint remains `GET /api/v1/projects/{project_id}/tasks/{task_id}/timeline`.

`task.created` metadata contains complete `responsible_user_ids`; `task.updated` before/after snapshots contain complete arrays. Singular `responsible_user_id` remains only for compatibility. Render historical assignment arrays as immutable facts rather than replacing them with current task state.

## 15. FE acceptance checklist

- [ ] Only four task operations use `/api/v2`.
- [ ] Protected requests include credentials; mutations include CSRF.
- [ ] `responsible_members` is a required array.
- [ ] Create accepts omitted, `null`, or `[]` as unassigned.
- [ ] Update always sends a unique complete array; `[]` clears it.
- [ ] Success responses replace cached task state.
- [ ] Conflicts refetch and require deliberate reconciliation.
- [ ] Invalid candidates refresh assignment data.
- [ ] V1 `task_v2_required` switches safely to V2.
- [ ] Responsibility never becomes authorization/ownership.
- [ ] Admin candidate selection uses enabled current memberships.
- [ ] Member UI respects the candidate-discovery limitation.
- [ ] Timeline renders plural assignment arrays.
- [ ] Unrelated V1 endpoints remain V1.

## 16. Suggested integration tests

1. Render zero, one, and multiple responsible Members.
2. Create with omitted, null, empty, one-ID, and multi-ID assignments.
3. Reject duplicate UI selection and handle backend validation.
4. Replace two assignments with one and confirm one version increment.
5. Clear assignments with `[]`.
6. Preserve draft and reconcile after a simulated conflict.
7. Recover from a stale candidate returning `invalid_responsible_member`.
8. Confirm responsible non-creators receive no edit controls.
9. Confirm Member creators preserve visible IDs when editing only text/date.
10. Keep inactive projects readable and non-editable.
11. Recover a V1 `task_v2_required` without assignment loss.
12. Render timeline before/after assignment arrays.
