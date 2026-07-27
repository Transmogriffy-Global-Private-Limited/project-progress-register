# Local development

## Supported environment

- Windows 11 without administrator access
- PowerShell 7+
- Go 1.25+
- Git for Windows
- A reachable PostgreSQL database

Docker, WSL, Bash, `make`, Node.js, and public network binding are not required.

The production target is PostgreSQL 18.4 on a small Ubuntu VPS behind Caddy. The foundation uses standard PostgreSQL behavior and does not depend on a version-specific extension.

## Configure the current PowerShell session

Copy values from `.env.example` into environment variables, replacing the example password and database with local credentials. The repository does not automatically parse `.env`.

```powershell
$env:APP_ENV = 'development'
$env:HTTP_ADDR = '127.0.0.1:8080'
$env:DATABASE_URL = 'postgres://<user>:<password>@127.0.0.1:5432/<database>?sslmode=disable'
$env:API_DOCS_ENABLED = 'true'
```

Do not paste real credentials into documentation, source files, screenshots, or chat.

## Migrations

Inspect without modification:

```powershell
.\scripts\migrate.ps1 -Command status
```

Apply the reviewed embedded migrations to the explicitly configured database:

```powershell
.\scripts\migrate.ps1 -Command up
```

`up` initializes `ppr_schema_migrations`, holds an advisory lock, verifies applied checksums, and applies pending migrations transactionally. It does not drop, truncate, or reverse schema. Never point migration commands at an unreviewed or unintended database.

## Run

```powershell
.\scripts\run-local.ps1
```

Then open `http://127.0.0.1:8080/`. When API docs are enabled, the viewer is at `http://127.0.0.1:8080/api/docs/` and the raw contract is at `http://127.0.0.1:8080/api/openapi/v1/openapi.yaml`.

The server refuses non-loopback and privileged-port configurations. Camera and geolocation browser APIs require a secure context; loopback is treated as secure by modern browsers, while production relies on Caddy HTTPS.

## Verification

```powershell
.\scripts\format.ps1
.\scripts\build.ps1
.\scripts\test.ps1
.\scripts\validate-openapi.ps1
.\scripts\smoke-foundation.ps1
.\scripts\verify-all.ps1
git diff --check
git status --short
```

`verify-all.ps1` checks formatting without changing files, validates module tidiness, runs `go vet`, all tests, and a complete build. The binary is written to ignored `.local/bin/ppr.exe`.

`smoke-foundation.ps1` starts two short-lived hidden processes on `127.0.0.1:18080` and `127.0.0.1:18081`, uses a deliberately unreachable PostgreSQL address, confirms readiness fails safely, checks the API-documentation toggle in both states, verifies the actual listener address, and terminates both test processes.

## Expected readiness states

- Liveness returns `200` whenever the process can serve HTTP.
- Readiness returns `503` if PostgreSQL is unavailable, migration initialization has not run, checksums differ, an unknown migration exists, or migrations are pending.
- After `migrate up` and successful PostgreSQL access, readiness returns `200`.

Internal dependency errors are logged by the server and are not returned to clients.
