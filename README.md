# Project Progress Register

Project Progress Register is an internal, mobile-first web application for replacing paper and spreadsheet project-progress logs with a simple chronological register.

The product vocabulary is intentionally small: Project, Task, Progress Update, Comment, Suggestion, and Admin Assessment. The implementation preserves stronger authorization, revision, evidence, attachment, and audit guarantees beneath that simple interface.

## Current state

The implemented foundation slice provides:

- a Go modular-monolith application shell;
- loopback-only HTTP configuration;
- a server-rendered home page;
- liveness and PostgreSQL/schema readiness endpoints;
- checksummed, forward-only PostgreSQL migration tooling;
- an authoritative OpenAPI contract and environment-controlled embedded Swagger UI;
- PowerShell-native local development and verification commands.

Accounts, projects, tasks, updates, attachments, comments, suggestions, assessments, and audit records are planned but are not yet implemented. See `docs/PROJECT_STATE.md` for the exact current boundary.

## Start here

- Documentation index: `docs/README.md`
- Product requirements: `docs/PRODUCT_REQUIREMENTS.md`
- Architecture: `docs/ARCHITECTURE.md`
- Development plan: `docs/DEVELOPMENT_PLAN.md`
- Local setup: `docs/LOCAL_DEVELOPMENT.md`
- API contract source: `api/openapi/v1/openapi.yaml`

## Quick verification

From PowerShell 7+:

```powershell
.\scripts\verify-all.ps1
git diff --check
git status --short
```

No Docker, WSL, Node.js runtime, administrator access, or public listener is required.
