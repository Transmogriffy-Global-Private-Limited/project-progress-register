# PostgreSQL integration

## Purpose and ownership

PostgreSQL is the authoritative durable store for application state, security state, metadata, revisions, assessments, and audit history. The Go application owns business orchestration; PostgreSQL enforces transactions, relationships, uniqueness, and checks.

The runtime creates a pool with at most eight connections to suit the 2 GB VPS target. Migration `000001_identity_and_audit.sql` creates `users`, `sessions`, `login_throttles`, and `audit_events` with constraints and indexes. Random UUID defaults use PostgreSQL's built-in `gen_random_uuid()`.

## Authentication and configuration

`DATABASE_URL` configures the runtime pool. `MIGRATION_DATABASE_URL` optionally separates schema-owner privileges from the runtime role; it falls back to the runtime URL for simple local development. Both are secrets and must not be logged or committed.

Production uses separate responsibilities. The protected migration URL connects as the existing schema owner for explicit DDL. The generated `ppr_runtime` login receives database connect, schema usage, table `SELECT`/`INSERT`/`UPDATE`/`DELETE`, and sequence usage/select grants, including matching default privileges for later migrations. It has no superuser, database-creation, or role-creation capability.

## Migration flow

```text
ppr migrate up
-> connect using migration URL
-> acquire fixed PostgreSQL advisory lock
-> create ppr_schema_migrations when absent
-> compare embedded versions and SHA-256 checksums to applied rows
-> reject unknown or edited history
-> apply each pending SQL migration in its own transaction
-> record version, name, checksum, and server timestamp
-> release lock
```

Migrations are named `NNNNNN_lowercase_name.sql` and embedded in the binary. Already-applied migrations are immutable; corrections use a new forward migration. Automatic migration during server startup is intentionally prohibited.

## Readiness and failure

The server constructs its pool lazily and can serve liveness while PostgreSQL is offline. Readiness performs a bounded ping followed by migration-current verification. Connection loss, missing ledger, checksum drift, unknown applied versions, or pending migrations produce `503 not_ready` without leaking details.

Migration transactions prevent partially applied SQL within one file. The advisory lock prevents two application migrators from applying concurrently. Operators retry after correcting connectivity, credentials, or schema state; destructive database recovery is never automatic.

Identity transitions keep each durable fact and its audit event together: bootstrap inserts user plus audit; successful login inserts session, updates last-login state, clears its throttle bucket, and audits; failure updates throttle plus audit; logout revokes plus audits. A transaction rollback prevents half-recorded identity state.

Migration `000002_account_administration.sql` adds forced-password-change state; the existing enabled/role index supports the Admin lock query. Account creation inserts user plus audit. Role/enabled mutation locks all enabled Admin rows, checks optimistic version and the final-Admin invariant, updates the account, revokes sessions, and audits. Reset/change operations update the Argon2id hash, change forced-password state, revoke sessions, and audit in one transaction.

Migration `000003_project_access.sql` adds projects, temporal project membership, and immutable geofence versions. Partial unique indexes enforce one current membership per Member/project and one current geofence per project. Project mutations and their audit events share transactions; geofence replacement serializes on the project row and preserves every superseded policy.

Migration `000004_task_register.sql` adds creator-owned tasks with Markdown source, optional responsible user/date, optimistic version, byte-size constraints, and project/creator/responsibility indexes. Task writes lock the authorized active project so concurrent membership removal cannot cross the command boundary; task state and audit commit together.

Migration `000005_progress_updates_and_attachments.sql` adds current progress entries, immutable before/after revisions, upload-location/geofence snapshots, idempotency hashes, and attachment metadata/state. Progress creation locks the authorized task/project boundary, confirms the evaluated geofence is still current, commits pending metadata plus progress audit, then filesystem finalization is marked available in a second audited transaction. Revision insertion, current-content update, version increment, and audit share one transaction.

Migration `000006_review_workflow.sql` adds immutable update comments, one-to-one accepted suggestions, and append-only versioned task assessments. Update/delete rejection triggers protect history. Suggestion acceptance locks the scoped comment and treats its unique acceptance as an idempotent result. Assessment append locks the scoped task, compares `expected_version` with the current maximum, inserts the next version, and commits its audit event in the same transaction.

Migration `000007_task_timeline.sql` adds immutable complete task before/after revisions. Task update locks current state, validates responsibility, updates the task, appends the revision, and records audit in one transaction. The authorized task timeline unions those revisions with other durable domain/history tables; it is not a second store.

Migration `000008_multiple_task_responsibilities.sql` copies every existing singular task assignment into `task_responsibilities`, converts historical revision responsibility fields into UUID arrays, restores the append-only revision trigger, and removes the obsolete singular columns/index. V2 task updates validate and replace the complete set transactionally. V1 uses the same rows through a singular compatibility adapter and refuses to flatten tasks that already have multiple assignments.

Migration `000009_camera_video_capture.sql` replaces the camera-image-only constraint with a camera image/video constraint and expands the verified-row shape to direct camera images/videos. It does not rewrite existing attachment rows or weaken the source/media/verification enums; documents still cannot claim camera source and uploaded media still cannot be verified by application policy.

On 2026-07-27, the operator-selected `pprdb` target was backed up, dropped, recreated, and initially migrated from zero through `000005`; users and every business table were confirmed empty afterward. Reviewed migrations `000006` through `000009` were later applied from validated pre-migration backups, and production currently reports nine applied and zero pending migrations. Migration `000008` copied the existing singular responsibility into the authoritative join table before removing the old columns; migration `000009` expanded the camera/verified constraint shape without rewriting attachment rows. Guarded lifecycle verifiers populate their target database and require an explicitly disposable environment.

## Local constraints and verification

The application connects outbound to PostgreSQL and does not alter the database listener. Local PostgreSQL must remain loopback-only. Migration commands modify the database named by the environment, so inspect the URL and status before applying.

Unit tests validate migration ordering, naming, checksums, and error cases without touching a database. On 2026-07-27, the human explicitly approved the initially empty remote `pprdb` PostgreSQL 18.4 target as disposable; migration and complete live identity verification passed. `scripts/verify-live-identity.ps1` preserves this repeatable flow but refuses any database with an existing user and must be run only with explicit authorization because it creates temporary identity and audit data.
