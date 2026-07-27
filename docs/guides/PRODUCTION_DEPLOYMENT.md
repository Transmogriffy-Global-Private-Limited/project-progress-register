# Production deployment

## Supported topology

The production process runs as the unprivileged `ppr` system user. The Go server listens only on `127.0.0.1:18090`; Caddy owns public HTTPS for `ppr.transev.site` and forwards only `/backend/*`. The application receives the prefix unchanged because `BASE_PATH=/backend` makes routing, redirects, cookies, assets, and optional API documentation prefix-aware.

```text
Browser
-> https://ppr.transev.site/backend/*
-> Caddy
-> http://127.0.0.1:18090/backend/*
-> ppr.service
-> configured PostgreSQL database and /var/lib/ppr/attachments
```

The hostname root is intentionally `404`. `/backend` redirects permanently to `/backend/`. Caddy must not strip the prefix.

## Files and ownership

| Path | Owner and mode | Purpose |
|---|---|---|
| `/opt/ppr/bin/ppr` | `root:root`, `0755` | Verified production binary. |
| `/etc/ppr/ppr.env` | `root:ppr`, `0640` | Production environment and secrets. Never commit or print it. |
| `/var/lib/ppr/attachments` | `ppr:ppr`, `0700` | Private attachment bytes. Never serve this directory through Caddy. |
| `/etc/systemd/system/ppr.service` | `root:root`, `0644` | Installed copy of `deploy/systemd/ppr.service`. |
| `/etc/caddy/Caddyfile` | existing Caddy ownership | Contains the reviewed block from `deploy/caddy/ppr.Caddyfile`. |

Database dumps and Caddy backups are operational artifacts outside the repository. Repository `.env*`, `.local/`, binaries, attachment bytes, dumps, and logs must remain ignored or external.

## Production environment

Create the production environment from the reviewed `.env.example`, preserving only the explicitly selected database URLs. Set at least:

```dotenv
APP_ENV=production
HTTP_ADDR=127.0.0.1:18090
BASE_PATH=/backend
API_DOCS_ENABLED=false
ATTACHMENT_STORAGE_DIR=/var/lib/ppr/attachments
```

Generate `SESSION_CSRF_KEY` from 32 cryptographically random bytes encoded with standard Base64. Generate `BOOTSTRAP_TOKEN` independently with at least 24 characters. `DATABASE_URL`, `MIGRATION_DATABASE_URL`, `SESSION_CSRF_KEY`, and `BOOTSTRAP_TOKEN` are secrets.

The service reads configuration only at process start. Any environment change requires restarting `ppr.service`.

## Database reset and migrations

Before a destructive reset, resolve and record the exact target from the reviewed URL, inspect its schemas and active connections, create a timestamped custom-format `pg_dump`, and verify the dump archive. Never infer that a similarly named database is disposable.

Run migrations explicitly; HTTP startup never migrates:

```bash
systemd-run --quiet --wait --pipe --collect --uid=ppr --gid=ppr \
  -p WorkingDirectory=/var/lib/ppr -p EnvironmentFile=/etc/ppr/ppr.env \
  /opt/ppr/bin/ppr migrate status
systemd-run --quiet --wait --pipe --collect --uid=ppr --gid=ppr \
  -p WorkingDirectory=/var/lib/ppr -p EnvironmentFile=/etc/ppr/ppr.env \
  /opt/ppr/bin/ppr migrate up
```

The transient service reads the protected environment file without expanding its secrets into shell arguments. A successful current schema reports five applied and zero pending migrations.

## Activation and verification

Validate the full Caddyfile before reload:

```bash
caddy validate --config /etc/caddy/Caddyfile
systemctl daemon-reload
systemctl enable --now ppr.service
systemctl reload caddy
```

Verify the application listener and local health before relying on DNS:

```bash
systemctl status ppr.service --no-pager
ss -ltnp | grep '127.0.0.1:18090'
curl --fail --silent --show-error http://127.0.0.1:18090/backend/api/v1/health/live
curl --fail --silent --show-error http://127.0.0.1:18090/backend/api/v1/health/ready
curl --fail --silent --show-error --resolve ppr.transev.site:443:127.0.0.1 \
  https://ppr.transev.site/backend/api/v1/health/ready
```

Production API documentation defaults to disabled: in that state, `/backend/api/docs/` and `/backend/api/openapi/v1/openapi.yaml` must return `404`. When an operator explicitly sets `API_DOCS_ENABLED=true` and restarts PPR, both routes must return `200`. Unprefixed application routes must return `404` in either state. See `PROJECT_STATE.md` for the currently selected exposure.

## Rehost handler

The installed `/usr/local/bin/rehost-ppr-service` is sourced from `scripts/rehost-ppr.sh`. `~/.bash_aliases` exposes the thin interactive handler `rehost-ppr`. It delegates the guarded service cycle to the host's shared `rehost-service`, then waits up to 30 seconds for database readiness before reporting success.

```bash
# Default 70-second delay, then follow logs.
rehost-ppr

# Immediate verified cycle without following logs.
rehost-ppr -t 0 --no-tail
```

`--no-reload` skips `systemctl daemon-reload`. The handler does not build, migrate, alter Caddy, or change environment files; prepare those surfaces separately before rehosting.

## First Admin and bootstrap removal

While the database has no users and `BOOTSTRAP_TOKEN` is configured, the compatibility setup page is `https://ppr.transev.site/backend/setup`. After creating the first Admin:

1. Remove the `BOOTSTRAP_TOKEN` line from `/etc/ppr/ppr.env`.
2. Restart only `ppr.service`.
3. Confirm readiness remains `200` and setup returns `404`.

The token is never stored in PostgreSQL. Losing it before bootstrap requires generating a replacement and restarting the service; leaving it configured after bootstrap does not reopen setup, but unnecessary secret retention is prohibited.

## Rollback and recovery

Before Caddy edits, keep a timestamped copy of the active Caddyfile. If proxy activation fails, restore that exact copy, validate it, and reload Caddy. Application rollback replaces `/opt/ppr/bin/ppr` with the previously retained binary and restarts only `ppr.service`.

Database migrations are forward-only. Restoring the pre-reset dump replaces the reset database and therefore requires an explicit outage, exact-target validation, and coordinated attachment restoration. PostgreSQL and `/var/lib/ppr/attachments` must be backed up as one logical recovery point once real uploads begin.

## Diagnostics

```bash
journalctl -u ppr.service -n 100 --no-pager
journalctl -u caddy -n 100 --no-pager
systemctl show ppr.service -p User -p Group -p ExecStart -p EnvironmentFiles
curl --silent --show-error -o /dev/null -w '%{http_code}\n' \
  http://127.0.0.1:18090/backend/api/v1/health/ready
```

Liveness proves only the HTTP process. Readiness additionally proves PostgreSQL connectivity and exact migration state.
