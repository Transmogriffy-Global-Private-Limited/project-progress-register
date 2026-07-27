# Architecture

## System boundary

Project Progress Register is one Go modular-monolith process behind Caddy and one PostgreSQL database. Caddy owns public TLS and proxies to a loopback-only application listener. Attachment bytes live in a configurable non-public local filesystem root; PostgreSQL owns attachment metadata and all durable business state.

There is no multi-tenancy, Redis, event broker, microservice, SPA framework, container runtime, or production Node.js process in v1.

## Current runtime topology

```text
Browser
  -> Caddy HTTPS on ppr.transev.site/backend/* (production only)
  -> Go HTTP server on 127.0.0.1:18090 with BASE_PATH=/backend
      -> existing minimal authentication/diagnostic shell
      -> identity application service
      -> project access application service
          -> stateless safe Markdown renderer
      -> progress application service
          -> private attachment storage
      -> PostgreSQL pool
```

The current foundation contains one `ppr` binary with explicit `serve`, `migrate up`, and `migrate status` commands.

Production runs that binary as the unprivileged `ppr` system user through `ppr.service`. Caddy preserves `/backend` when proxying; the application owns prefix-aware routing, redirects, compatibility-page links, attachment response links, Swagger addressing, and the `/backend/` session-cookie path. The hostname root and every unprefixed application route remain unavailable.

## Package ownership

- `cmd/ppr` — process entry point, command selection, dependency wiring, signals, and graceful shutdown.
- `internal/config` — environment parsing and validation, including the loopback invariant.
- `internal/database` — low-footprint pgx pool construction.
- `internal/migrations` — embedded forward-only SQL migrations, checksums, advisory locking, and schema-current checks.
- `internal/health` — bounded readiness orchestration.
- `internal/httpserver` — HTTP routing, transport behavior, security headers, logging, and error containment.
- `internal/identity` — account validation, Argon2id password policy, bootstrap, login throttling, opaque sessions, CSRF, complete paginated audit queries, audit orchestration, and PostgreSQL identity queries.
- `internal/projects` — project lifecycle, temporal membership, geofence versions, task ownership/responsibility and before/after revisions, append-only Admin assessments, authorized dashboard/timeline read models, scoped PostgreSQL queries, and audit orchestration.
- `internal/progress` — progress updates, immutable revisions and comments, accepted suggestions, upload-location policy, per-attachment verification, authorized downloads, and storage reconciliation.
- `internal/filestore` — private staged/final attachment bytes, detected-type allowlisting, SHA-256 hashing, and atomic finalization.
- `internal/safemarkdown` — Goldmark parsing and Bluemonday allowlist sanitization for derived HTML response fields.
- `internal/webui` — trusted templates and static assets.
- `api/openapi/v1` — authoritative version-one OpenAPI document and embedded schema bytes.

New domain packages are added only with the vertical slice that uses them. Transport handlers may parse and present data but must not own authorization or business decisions. Application services own orchestration; domain code owns invariants; PostgreSQL enforces durable constraints.

## Startup and shutdown

`ppr serve` performs this sequence:

```text
load and validate environment
-> create a lazy PostgreSQL pool
-> load embedded migration metadata
-> construct bounded readiness and PostgreSQL identity/project/progress repositories
-> construct identity policy, shared Markdown renderer, project/task policy, private filestore, and progress policy
-> start non-blocking attachment reconciliation
-> parse templates and register routes
-> bind the validated loopback address
-> serve until SIGINT or SIGTERM
-> gracefully drain requests within SHUTDOWN_TIMEOUT
-> close PostgreSQL pool
```

The server may remain alive while PostgreSQL is unavailable so liveness and diagnostics still work. Readiness remains `503` until PostgreSQL responds and the migration ledger is current.

## Health semantics

- `GET /api/v1/health/live` proves only that the HTTP process can respond. It never queries PostgreSQL.
- `GET /api/v1/health/ready` uses `READINESS_TIMEOUT`, pings PostgreSQL, then validates the checksummed migration ledger. It returns only `ready` or `not_ready`; internal connection or schema details remain in server logs.

## Persistence and migrations

PostgreSQL is authoritative. `ppr migrate up` holds a PostgreSQL advisory lock, creates `ppr_schema_migrations` when absent, checks already-applied checksums, and applies each pending SQL file in its own transaction. Edited or unknown applied migrations are rejected. Business migrations are forward-only and arrive in the same slice as their consuming behavior.

Runtime and migration database URLs can differ so production can give the runtime role fewer privileges. Migration commands never run automatically during HTTP startup.

## HTTP and contracts

The backend uses `net/http`; JSON APIs and OpenAPI are the product integration boundary. The existing minimal `html/template` authentication/diagnostic shell remains for compatibility but receives no new product features. `BASE_PATH` optionally mounts the complete transport beneath one canonical prefix without rewriting operation paths. Request middleware creates a random correlation ID and accepts `X-Forwarded-For` only from a loopback peer, matching the Caddy boundary.

`api/openapi/v1/openapi.yaml` is specification-first and authoritative for the implemented JSON API. The embedded source powers validation, the raw schema route, and Swagger UI. At route construction the server creates a defensive copy and resolves only the OpenAPI `basePath` server-variable default to the configured deployment prefix; operation paths and all schemas remain the committed bytes. `API_DOCS_ENABLED=false` prevents both documentation routes from being registered.

## Dependencies

- `github.com/jackc/pgx/v5` provides the native PostgreSQL driver and bounded connection pool.
- `github.com/getkin/kin-openapi` validates the committed OpenAPI document during tests; it does not sit in the request path.
- `github.com/swaggest/swgui/v5emb` provides embedded Swagger UI assets so documentation needs neither Node.js nor a public CDN.
- `golang.org/x/crypto` provides Argon2id password hashing.
- `github.com/yuin/goldmark` parses GitHub-flavored Markdown without a Node.js runtime.
- `github.com/microcosm-cc/bluemonday` applies an explicit UGC HTML allowlist after Markdown rendering.

Migration orchestration remains a small project-owned component because the required behavior is limited to ordered embedded SQL, checksums, advisory locking, transactional forward application, status, and readiness. It does not parse SQL or attempt automatic rollback.

## Security boundaries

- Listener validation rejects wildcard, LAN, public, malformed, and privileged addresses.
- Configuration is environment-only; secrets are never committed or logged.
- HTTP responses set baseline content, framing, referrer, and MIME-sniffing protections.
- Authentication uses random opaque tokens in host-only `HttpOnly`, `SameSite=Lax` cookies. Only token hashes are durable; every request rechecks expiry, revocation, and the user enabled flag.
- Authenticated writes require an HMAC-derived, session-bound CSRF token. Production cookies additionally set `Secure`.
- Five failed logins per normalized-identifier/IP pair in 15 minutes block that pair for 15 minutes. Client errors do not disclose account, enabled, or throttle state.
- Account administration is one identity-owned flow: Admin authorization, normalized account creation, generated temporary credential, role/enabled mutation, password reset/change, session revocation, and audit. PostgreSQL locks enabled Admin rows before a demotion/disable decision and rejects removal of the final enabled Admin.
- Accounts with `must_change_password=true` may recover the current session, log out, or replace the password, but receive `403` from the retained diagnostic home and from Admin operations. Replacement revokes every session and requires a fresh login.
- The projects module owns project lifecycle, temporal membership, and versioned geofence policy. Admin scope is global; Member list/detail queries contain the active-membership predicate in PostgreSQL, so an untrusted identifier cannot bypass scope.
- Dashboard and task-timeline projections remain ordinary PostgreSQL queries. They apply current trusted project scope, use durable domain/history tables, and introduce no cache, reporting database, queue, or duplicated source of truth.
- Project edits use project versions. Geofence replacement locks the project, compares the current geofence version, closes it, and inserts the next immutable policy in one transaction. PostgreSQL numeric constraints protect the accepted coordinate and distance ranges without PostGIS.
- Tasks live inside the project aggregate. PostgreSQL project locks keep membership stable during task operations; Admins may edit any task while Members require both current access and immutable creator ownership. Responsibility is display/work assignment only and never grants access or ownership.
- Markdown source is durable truth. Goldmark output passes through Bluemonday before the API returns read-only HTML fields; raw client HTML is never trusted or persisted as rendered truth.
- The progress module owns task diary entries, immutable revisions, upload-location snapshots, attachment verification labels, and authorized download decisions. It reads project/task/membership/geofence state through scoped PostgreSQL queries but never writes project-owned tables.
- The progress module also owns immutable update comments and their separate one-to-one accepted-suggestion records. The projects module owns append-only task assessments. Both use the same trusted project scope; Members may read current review state, while only Admins may accept suggestions, append assessments, or inspect full assessment history.
- The filestore module streams allowlisted bytes to a private staging root, calculates SHA-256, and atomically finalizes opaque keys. PostgreSQL attachment state remains authoritative; immediate-and-minute reconciliation recovers pending finalization without blocking process liveness.
- Future authorization is centralized in application services and reinforced by project-scoped queries and database constraints.
- Browser evidence remains explicitly non-cryptographic; see `EVIDENCE_AND_TRUST_MODEL.md`.

## Recovery model

The process is stateless apart from PostgreSQL and the private attachment root. Restart reconstructs configuration, pools, identity/project/task/progress policy, storage, the Markdown renderer, compatibility templates, routes, and migration expectations. PostgreSQL preserves accounts, hashed sessions, throttles, projects, access history, geofence versions, task/progress source content, revisions, comments, accepted suggestions, assessment versions, attachment state, and append-only audit events. Current assessment is derived from the highest durable task version, so no process-memory restoration is required. A background loop immediately reconciles pending attachment rows and retries each minute; database unavailability postpones reconciliation while liveness remains available. Expired or revoked sessions stop authenticating immediately; derived HTML is recreated from stored Markdown and retention cleanup is intentionally deferred.
