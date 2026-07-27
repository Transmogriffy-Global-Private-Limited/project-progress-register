# Architecture

## System boundary

Project Progress Register is one Go modular-monolith process behind Caddy and one PostgreSQL database. Caddy owns public TLS and proxies to a loopback-only application listener. Attachment bytes will live in a configurable non-public local filesystem root; PostgreSQL owns attachment metadata and all durable business state.

There is no multi-tenancy, Redis, event broker, microservice, SPA framework, container runtime, or production Node.js process in v1.

## Current runtime topology

```text
Browser
  -> Caddy HTTPS (production only)
  -> Go HTTP server on loopback
      -> server-rendered templates and static assets
      -> application/use-case services (future vertical slices)
      -> PostgreSQL pool
      -> local attachment storage (future evidence slice)
```

The current foundation contains one `ppr` binary with explicit `serve`, `migrate up`, and `migrate status` commands.

## Package ownership

- `cmd/ppr` — process entry point, command selection, dependency wiring, signals, and graceful shutdown.
- `internal/config` — environment parsing and validation, including the loopback invariant.
- `internal/database` — low-footprint pgx pool construction.
- `internal/migrations` — embedded forward-only SQL migrations, checksums, advisory locking, and schema-current checks.
- `internal/health` — bounded readiness orchestration.
- `internal/httpserver` — HTTP routing, transport behavior, security headers, logging, and error containment.
- `internal/webui` — trusted templates and static assets.
- `api/openapi/v1` — authoritative version-one OpenAPI document and embedded schema bytes.

Future domain packages will be added only with the vertical slice that uses them. Transport handlers may parse and present data but must not own authorization or business decisions. Application services will own orchestration; domain code will own invariants; PostgreSQL will enforce durable constraints.

## Startup and shutdown

`ppr serve` performs this sequence:

```text
load and validate environment
-> create a lazy PostgreSQL pool
-> load embedded migration metadata
-> construct bounded readiness
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

The application uses `net/http` and `html/template`. HTMX and small vanilla JavaScript may be added only when a workflow needs them. HTML form handlers and JSON endpoints will call the same application services rather than duplicate policy.

`api/openapi/v1/openapi.yaml` is specification-first and authoritative for the implemented JSON API. The same embedded bytes power validation, the raw schema route, and Swagger UI. `API_DOCS_ENABLED=false` prevents both documentation routes from being registered.

## Foundation dependencies

- `github.com/jackc/pgx/v5` provides the native PostgreSQL driver and bounded connection pool.
- `github.com/getkin/kin-openapi` validates the committed OpenAPI document during tests; it does not sit in the request path.
- `github.com/swaggest/swgui/v5emb` provides embedded Swagger UI assets so documentation needs neither Node.js nor a public CDN.

Migration orchestration remains a small project-owned component because the required behavior is limited to ordered embedded SQL, checksums, advisory locking, transactional forward application, status, and readiness. It does not parse SQL or attempt automatic rollback.

## Security boundaries

- Listener validation rejects wildcard, LAN, public, malformed, and privileged addresses.
- Configuration is environment-only; secrets are never committed or logged.
- HTTP responses set baseline content, framing, referrer, and MIME-sniffing protections.
- Future authentication uses server-side opaque sessions and CSRF protection.
- Future authorization is centralized in application services and reinforced by project-scoped queries and database constraints.
- Browser evidence remains explicitly non-cryptographic; see `EVIDENCE_AND_TRUST_MODEL.md`.

## Recovery model

The process is stateless apart from PostgreSQL and the future attachment root. Restart reconstructs configuration, pools, templates, routes, and migration expectations. PostgreSQL preserves business state. The attachment slice will introduce staged writes, explicit metadata state, and reconciliation so database/filesystem partial failure is recoverable.
