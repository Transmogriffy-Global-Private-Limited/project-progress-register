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
$env:SESSION_CSRF_KEY = '<standard-base64-encoded-32-random-bytes>'
$env:BOOTSTRAP_TOKEN = '<one-time-secret-at-least-24-characters>'
$env:ATTACHMENT_STORAGE_DIR = '.local/attachments'
$env:ATTACHMENT_MAX_FILE_BYTES = '104857600'
$env:ATTACHMENT_MAX_FILES_PER_UPDATE = '10'
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

On the first run only, open `/setup` and provide the configured bootstrap token. After the Admin is created, remove `BOOTSTRAP_TOKEN` from the environment and restart. The application has no public registration route.

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

`verify-live-identity.ps1 -EnvFile .env.local` is intentionally manual. It requires a migrated database with zero users and creates one temporary Admin, session, throttle row, and authentication audit history while checking the complete identity flow. Use it only when those writes and the resulting reset requirement are explicitly approved. It generates bootstrap, password, and CSRF values in memory and never prints them.

`verify-live-account-admin.ps1 -EnvFile .env.local` is the corresponding destructive disposable-database verifier for the complete account lifecycle. It requires zero users and checks forced password replacement, Admin denial, reset/session revocation, promotion, safe demotion, and the final-enabled-Admin guard. It remains intentionally unexecuted against production; do not bypass its disposable-target and zero-user guard.

`verify-live-project-access.ps1 -EnvFile .env.local` requires the same fully migrated zero-user disposable database. It creates temporary identities and verifies hidden identifiers before membership, immediate visibility/revocation, two geofence versions, stale-version conflict, persisted history, and project audit. It is authored but must not run until verification resumes and the target data is confirmed disposable.

`verify-live-task-register.ps1 -EnvFile .env.local` creates a disposable Admin and two Members, then verifies scoped task creation/read/update, sanitized Markdown, responsibility without ownership, creator-only Member edits, stale versions, inactive-project rejection, persistence, and audit. It has the same zero-user and explicit-authorization requirement and remains unexecuted.

`verify-live-progress.ps1 -EnvFile .env.local` extends the disposable zero-user flow through a project, geofence, task, direct camera photo/video, uploaded document, shared geotag, per-file verified/non-verified classification, immutable revision, authorized download/range stream, storage state, and audit. It writes bytes under a unique ignored `.local/progress-live-*` root for inspection. It must not run unless the target is confirmed disposable, fully migrated, and contains zero users.

`verify-live-review.ps1 -EnvFile .env.local` extends the same guard through immutable task history, comments, Member acceptance denial, first/idempotent Admin acceptance, task-level accepted suggestions, two optimistic assessment versions, dashboard aggregates, complete authorized task timeline, full-audit authorization/pagination, and durable counts. It is authored but must not run unless testing is explicitly resumed and the fully migrated target contains zero users and is explicitly disposable.

## Expected readiness states

- Liveness returns `200` whenever the process can serve HTTP.
- Readiness returns `503` if PostgreSQL is unavailable, migration initialization has not run, checksums differ, an unknown migration exists, or migrations are pending.
- After `migrate up` and successful PostgreSQL access, readiness returns `200`.

Internal dependency errors are logged by the server and are not returned to clients.
