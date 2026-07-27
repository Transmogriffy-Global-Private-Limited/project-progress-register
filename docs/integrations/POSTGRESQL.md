# PostgreSQL integration

## Purpose and ownership

PostgreSQL is the authoritative durable store for application state, security state, metadata, revisions, assessments, and audit history. The Go application owns business orchestration; PostgreSQL enforces transactions, relationships, uniqueness, and checks.

The runtime creates a pool with at most eight connections to suit the 2 GB VPS target. Migration `000001_identity_and_audit.sql` creates `users`, `sessions`, `login_throttles`, and `audit_events` with constraints and indexes. Random UUID defaults use PostgreSQL's built-in `gen_random_uuid()`.

## Authentication and configuration

`DATABASE_URL` configures the runtime pool. `MIGRATION_DATABASE_URL` optionally separates schema-owner privileges from the runtime role; it falls back to the runtime URL for simple local development. Both are secrets and must not be logged or committed.

Future production setup should grant the runtime role only the table and sequence operations actually required. The migration role owns DDL. Exact grants will be documented with the first domain migration.

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

Migration `000002_account_administration.sql` adds forced-password-change state; the existing enabled/role index supports the Admin lock query. Account creation inserts user plus audit. Role/enabled mutation locks all enabled Admin rows, checks optimistic version and the final-Admin invariant, updates the account, revokes sessions, and audits. Reset/change operations update the Argon2id hash, change forced-password state, revoke sessions, and audit in one transaction. Automated verification passes; migrations `000002` through `000004` and their database-live lifecycle scripts remain intentionally unexecuted against retained data.

Migration `000003_project_access.sql` adds projects, temporal project membership, and immutable geofence versions. Partial unique indexes enforce one current membership per Member/project and one current geofence per project. Project mutations and their audit events share transactions; geofence replacement serializes on the project row and preserves every superseded policy. This migration is also authored but intentionally unexecuted during the testing pause.

Migration `000004_task_register.sql` adds creator-owned tasks with Markdown source, optional responsible user/date, optimistic version, byte-size constraints, and project/creator/responsibility indexes. Task writes lock the authorized active project so concurrent membership removal cannot cross the command boundary; task state and audit commit together. The migration is authored but unexecuted during the pause.

## Local constraints and verification

The application connects outbound to PostgreSQL and does not alter the database listener. Local PostgreSQL must remain loopback-only. Migration commands modify the database named by the environment, so inspect the URL and status before applying.

Unit tests validate migration ordering, naming, checksums, and error cases without touching a database. On 2026-07-27, the human explicitly approved the initially empty remote `pprdb` PostgreSQL 18.4 target as disposable; migration and complete live identity verification passed. `scripts/verify-live-identity.ps1` preserves this repeatable flow but refuses any database with an existing user and must be run only with explicit authorization because it creates temporary identity and audit data.
