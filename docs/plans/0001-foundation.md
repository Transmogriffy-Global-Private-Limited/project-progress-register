# Plan 0001 — Application foundation

Status: Implemented; live PostgreSQL success-path verification pending

## Surface map

- Repository-specific `AGENTS.md` and documentation index
- Product, architecture, domain, permissions, evidence, configuration, plan, state, changelog, and ADRs
- Go module and one `ppr` command
- Configuration and loopback listener validation
- PostgreSQL pool, migration ledger, and readiness boundary
- Server-rendered home and embedded static assets
- Versioned health routes, OpenAPI source, and embedded Swagger UI
- PowerShell format/build/test/run/migrate/contract/full-verification scripts
- Unit, transport, migration-loader, readiness, and contract tests

## Acceptance criteria

- `ppr serve` accepts only configured loopback addresses on unprivileged ports and shuts down gracefully.
- Home and liveness work independently of PostgreSQL readiness.
- Readiness succeeds only when PostgreSQL responds and embedded migration state is current.
- `ppr migrate up` initializes a checksummed, advisory-locked, forward-only ledger without creating unused business tables.
- `API_DOCS_ENABLED=true` serves the raw authoritative schema and embedded viewer; false makes all docs routes return `404`.
- OpenAPI validates and covers every registered versioned JSON route.
- The repository can be built and verified natively from PowerShell without elevation, Docker, WSL, Node, or public binding.
- Documentation distinguishes implemented foundation behavior from approved future behavior.

## Implementation

### A — Database foundation

Use pgx with an eight-connection pool. Implement embedded SQL discovery, immutable checksums, an advisory lock, a ledger, transactional forward application, read-only status, and schema-current readiness. Do not introduce unused domain tables.

### B — Application and transport

Create one binary with `serve`, `migrate up`, and `migrate status`. Use `net/http`, `html/template`, explicit dependency wiring, bounded timeouts, structured logs, baseline security headers, panic containment, and graceful shutdown.

### C — Contracts

Author OpenAPI 3.1 at `api/openapi/v1/openapi.yaml`. Serve the exact embedded bytes and an embedded Swagger UI only when the environment toggle is true.

### D — Focused verification

Test configuration defaults and unsafe addresses, readiness ordering/failure, migration discovery and checksums, HTTP status/payload/method behavior, docs enabled/disabled behavior, and OpenAPI validation/route coverage.

### E — Documentation and memory

Create the authoritative documentation system and ADRs. Record KISS choices, non-goals, trust limitations, known unverified database behavior, and the next vertical slice.

## Verification commands

```powershell
.\scripts\format.ps1
.\scripts\validate-openapi.ps1
.\scripts\test.ps1
.\scripts\build.ps1
.\scripts\smoke-foundation.ps1
.\scripts\verify-all.ps1
git diff --check
git status --short
```

Optional live PostgreSQL smoke verification requires an explicitly selected safe development database:

```powershell
.\scripts\migrate.ps1 -Command status
.\scripts\migrate.ps1 -Command up
.\scripts\run-local.ps1
```

No database-modifying check runs implicitly.

## Progress

- Repository and branch inspection: complete.
- Main-to-feature synchronization: complete.
- Foundation implementation and focused unit tests: complete.
- Documentation and ADRs: complete.
- PowerShell syntax, format, module tidiness, vet, tests, OpenAPI, build, race detector, loopback smoke, docs-toggle smoke, and residue scan: passed.
- Live PostgreSQL migration/readiness success path: not run because no explicit safe connection or credentials are configured.
