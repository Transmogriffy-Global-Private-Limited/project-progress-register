# Permissions

Status: Identity enforcement is live-verified. Account/project/task/progress/attachment/review/reporting enforcement passes automated verification; later database-live lifecycle verification remains pending.

Authorization is enforced in backend application services and project-scoped persistence queries. Templates may hide unavailable actions for usability but are never an authorization boundary.

| Capability | Admin | Member |
|---|---|---|
| Manage users and roles | Yes | No |
| View all projects | Yes | No |
| View member project | Yes | Yes, only current membership |
| Create/edit projects | Yes | No |
| Manage project membership/geofence | Yes | No |
| Create task | Yes | Yes, in current member project |
| Edit task | Any task | Own task only, with current project access |
| Add progress update | Yes | Yes, in current member project |
| Edit progress update | Any update | Own update only, with current project access |
| Comment on update | Yes | Yes, with current project access |
| Accept comment as suggestion | Yes | No |
| Set task assessment | Yes | No |
| View current assessment/suggestions | Yes | Yes, with current project access |
| View assessment history | Yes | No |
| View update revision history | Yes | Yes, with current project access |
| View/download project attachment | Yes | Yes, with current project access and project policy |
| View untrusted EXIF separately | Yes | No by default |
| View full audit trail | Yes | No |
| View complete task timeline | Yes | Yes, with current project access |

## Enforcement invariants

- Admin is a global application role because the approved product explicitly grants Admin access to all projects.
- Member project access requires a current `project_members` relationship for every read and command.
- Client-supplied project, task, update, comment, or attachment identifiers are always resolved through an authorized scope.
- “Own task” and “own update” use immutable creator/author identity, never responsible member or another mutable field.
- Removing project membership revokes future project access immediately but does not erase historical authorship.
- Disabling a user revokes active sessions and blocks new authentication.
- Only an Admin may change roles, membership, geofences, suggestions, or assessments.
- The final enabled Admin cannot be disabled or demoted.
- Every privileged state change and material denied attempt specified by the audit policy records an audit event without secrets.
- Only an enabled Admin whose temporary password has been replaced may list, create, role-change, enable/disable, or reset users.
- Every authenticated user may replace their own password; doing so revokes all of their sessions.
- Project list/detail queries derive Member scope from current membership in PostgreSQL and return `404` for inaccessible identifiers.
- Only enabled Member-role accounts can receive project membership; Admin access is global and is never represented by membership rows.
- Inactive projects remain readable to authorized users for history but are not deleted or silently hidden.
- Admins or current project Members may create tasks only in active projects; creator identity is the authenticated actor and cannot change.
- Admins may edit any task. Members may edit only their own task while they retain current project membership.
- Every responsible assignment requires a current enabled project Member but grants neither project access nor edit ownership. V2 replaces the complete assignment set atomically; membership removal removes that user while retaining other assignees.
- Admins and current project Members may add progress to active-project tasks. Members edit only updates they authored; Admins edit any. Every edit appends an immutable revision.
- Current project access permits reading all task progress, revisions, attachment metadata, and available bytes. Authorship controls mutation, not read visibility.
- Every file requires reported upload coordinates. Geofence failure never grants verification and never denies accepted bytes; only a camera-source image with a server-verified location is labelled verified.
- Attachment identifiers are resolved through project, task, and progress scope. Original names and opaque storage keys never grant access.
- Comments are immutable and readable within current project scope. Admins and current Members may add them only to active projects.
- Suggestion acceptance is a separate immutable Admin action. Repeating it returns the one existing acceptance and does not duplicate state or audit.
- Current assessments are visible within current project scope. Only Admins append assessments or read full history; append uses the current durable version to reject stale writers.
- The complete task timeline uses the same current project scope as task reads. It exposes domain chronology and metadata, not security-only audit request context.
