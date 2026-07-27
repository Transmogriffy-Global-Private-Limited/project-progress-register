# Permissions

Status: Approved model; enforcement arrives with each vertical feature.

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
| View update revision history | Yes | Own current update history only unless later approved otherwise |
| View/download project attachment | Yes | Yes, with current project access and project policy |
| View untrusted EXIF separately | Yes | No by default |
| View full audit trail | Yes | No |

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

## Open decision

Whether Members may inspect revision history for updates created by other project members remains a non-blocking product decision. The conservative default exposes current content to all project members and full revision history to Admins plus the update author.
