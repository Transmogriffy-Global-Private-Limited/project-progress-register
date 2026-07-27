# Configuration reference

Configuration is read from process environment when `ppr` starts. The application does not silently load `.env`; `.env.example` is a non-secret template. Every change requires process restart.

| Variable | Type / accepted values | Default | Required | Sensitive | Consumer and behavior |
|---|---|---|---|---|---|
| `APP_NAME` | Non-empty string | `Project Progress Register` | No | No | HTTP page and API-documentation titles. Keeps branding replaceable. |
| `APP_ENV` | `development`, `test`, `production` | `development` | No | No | Runtime environment label. Production adds `Secure` to the session cookie. |
| `HTTP_ADDR` | Loopback `host:port`; port 1024–65535 | `127.0.0.1:8080` | No | No | HTTP listener. Wildcard, LAN, public, malformed, and privileged addresses are rejected. IPv6 uses `[::1]:port`. |
| `BASE_PATH` | Empty or canonical slash-prefixed URL path without trailing slash | Empty | No | No | Mounts every application route beneath one prefix. Production uses `/backend`; Caddy preserves this prefix. |
| `DATABASE_URL` | PostgreSQL connection URI | None | Yes | Yes | Runtime PostgreSQL pool and, by default, migrations. Never logged. |
| `MIGRATION_DATABASE_URL` | PostgreSQL connection URI | `DATABASE_URL` | No | Yes | Migration command only. Production may supply a more privileged migration role while the server uses a restricted runtime role. |
| `API_DOCS_ENABLED` | Exactly `true` or `false` | `false` | No | No | Registers or omits `/api/docs/`, `/api/docs`, and `/api/openapi/v1/openapi.yaml`. |
| `SESSION_CSRF_KEY` | Standard Base64 of exactly 32 bytes | None | Required for `serve`; not migrations | Yes | HMAC key for deriving session-bound CSRF tokens. Changing it invalidates outstanding CSRF tokens and requires restart, but does not revoke sessions. |
| `SESSION_TTL` | Go duration from `15m` through `168h` | `12h` | No | No | Absolute lifetime of each newly created session and cookie. |
| `BOOTSTRAP_TOKEN` | 24–256 non-whitespace-trimmed characters | None | No | Yes | Enables one-time first-Admin setup only while no users exist. Remove after bootstrap and restart. Never stored or logged. |
| `ATTACHMENT_STORAGE_DIR` | Non-empty filesystem path | `.local/attachments` | No | No | Private attachment byte root. Relative paths resolve from process working directory. Must not be publicly served. |
| `ATTACHMENT_MAX_FILE_BYTES` | Integer 1048576–1073741824 | `104857600` | No | No | Streaming per-file byte limit; multipart total is bounded from this and the count limit. |
| `ATTACHMENT_MAX_FILES_PER_UPDATE` | Integer 1–25 | `10` | No | No | Maximum repeated file parts and attachment descriptors in one progress creation. |
| `READINESS_TIMEOUT` | Go duration from `100ms` through `30s` | `2s` | No | No | Total bound for PostgreSQL ping plus migration-current verification. |
| `SHUTDOWN_TIMEOUT` | Go duration from `1s` through `1m` | `10s` | No | No | Graceful HTTP request-drain deadline after termination signal. |

## Security

Do not commit `.env`, database passwords, or production connection strings. Set them in the process environment or the deployment’s established secret mechanism. `.env`, `.env.*`, and local runtime data are ignored; `.env.example` is intentionally tracked.

The docs toggle defaults off because interactive execution and a complete schema expand the observable surface. Local development may explicitly enable it. Sensitive environments should keep it disabled unless access is intentionally permitted. `BASE_PATH` affects routes, redirects, template URLs, the session-cookie path, and Swagger addressing; changing it invalidates the old browser route and requires restart.
