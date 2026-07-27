# Repository Agent Instructions

The global `AGENTS.md` remains binding. These rules register the authoritative project-specific surfaces for Project Progress Register.

## Permanent invariants

- Keep the product a simple progress register: Project, Task, Progress Update, Comment, Suggestion, and Admin Assessment.
- Use a Go modular monolith, PostgreSQL, server-rendered HTML, focused HTMX, and minimal vanilla JavaScript.
- Do not introduce Node.js production runtime, containers, Redis, microservices, or public listener bindings without explicit approval.
- Bind local and production application listeners to loopback; Caddy owns external TLS termination.
- PostgreSQL is the durable source of truth. Browser location and camera evidence are operational evidence, not cryptographic proof.
- Enforce authentication, authorization, project membership, creator ownership, and Admin-only actions in backend application services.
- Keep the implementation and repository memory synchronized in the same coherent slice.

## Required project memory

Before planning or changing code, read:

- `docs/README.md`
- `docs/PRODUCT_REQUIREMENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/DOMAIN_MODEL.md`
- `docs/PERMISSIONS.md`
- `docs/EVIDENCE_AND_TRUST_MODEL.md`
- `docs/DEVELOPMENT_PLAN.md`
- `docs/PROJECT_STATE.md`
- `docs/AI_CHANGELOG.md`
- relevant plans under `docs/plans/`
- relevant ADRs under `docs/decisions/`

Approved plans are binding. Material architectural or scope deviations require updated plans and explicit human approval.

## Required documentation surfaces

- Documentation index: `docs/README.md`
- Educational and operational guides: `docs/guides/` and `docs/LOCAL_DEVELOPMENT.md`
- Integration documentation: `docs/integrations/`
- Human-readable contracts: `docs/contracts/`
- Authoritative OpenAPI source: `api/openapi/v1/openapi.yaml`
- Detailed plans: `docs/plans/`
- Architecture decisions: `docs/decisions/`

The OpenAPI source is specification-first and committed. Do not create an independently maintained generated schema.

## Canonical commands

Run from PowerShell 7+ at the repository root:

```powershell
.\scripts\format.ps1
.\scripts\build.ps1
.\scripts\test.ps1
.\scripts\validate-openapi.ps1
.\scripts\smoke-foundation.ps1
.\scripts\verify-all.ps1
```

`scripts/verify-live-identity.ps1` is a manually authorized integration verifier. It requires an already migrated database with zero users, creates one temporary Admin and security/audit rows, and must never be included in routine verification or run against a database whose data must be preserved.

Use `.\scripts\run-local.ps1` for loopback-only local serving and `.\scripts\migrate.ps1` for migration status or application.

Every meaningful slice ends with focused verification, a residue scan, full verification, `git diff --check`, and `git status --short`.
