# AI changelog

## 2026-07-27 — Foundation slice

Status: Implemented; live PostgreSQL success-path verification pending

Changed:

- Established repository-specific agent instructions and authoritative documentation navigation.
- Added the approved product, architecture, domain, permission, evidence/trust, configuration, development-plan, project-state, detailed-plan, and ADR documentation.
- Implemented the Go modular-monolith foundation, strict loopback configuration, server-rendered shell, health/readiness, PostgreSQL migration framework, OpenAPI source, embedded Swagger UI toggle, and PowerShell workflow.
- Recorded the decision to defer blockchain while preserving ordinary integrity hashes and a future external-checkpoint path.

Why:

- Provide a small, explicit, verifiable base before identity and business-domain work.
- Keep documentation, contracts, runtime behavior, and verification synchronized from the first slice.

Verification:

- `gofmt -w ./cmd ./internal ./api`
- `go mod tidy`
- `go test ./...` — passed
- `.\scripts\validate-openapi.ps1` — passed
- `.\scripts\build.ps1` — passed
- `.\scripts\verify-all.ps1` — PowerShell syntax, format, module tidiness, vet, tests, and build passed
- `.\scripts\smoke-foundation.ps1` — passed in docs-enabled and docs-disabled states on `127.0.0.1`
- `go test -race ./...` — passed
- Residue scan and `git diff --check` — passed before final staging

Remaining before Verified:

- Run `migrate status`, `migrate up`, and a `200 ready` check against an explicitly approved safe development/test database.

Compatibility and migration impact:

- New repository; no prior application compatibility surface.
- The migration command creates only `ppr_schema_migrations` in this slice; there are no business migrations yet.
