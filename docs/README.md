# Documentation index

This index is the entry point to the authoritative Project Progress Register documentation. A document marked as planned describes approved direction; `PROJECT_STATE.md` alone owns claims about what is currently implemented.

## Product and concepts

- `PRODUCT_REQUIREMENTS.md` — approved product purpose, scope, workflows, and non-goals.
- `DOMAIN_MODEL.md` — entity ownership, relationships, invariants, and lifecycle.
- `PERMISSIONS.md` — Admin and Member authorization matrix and enforcement rules.
- `EVIDENCE_AND_TRUST_MODEL.md` — camera, geolocation, metadata, hashing, and tamper-evidence boundaries.
- `guides/SAFE_MARKDOWN.md` — Markdown source-of-truth, sanitization, frontend contract, limits, and recovery.
- `guides/PROGRESS_EVIDENCE.md` — upload geotags, per-file verification, multipart retries, and recovery.

## Architecture and operations

- `ARCHITECTURE.md` — modular-monolith boundaries, runtime topology, startup, persistence, and request flows.
- `CONFIGURATION.md` — complete environment-variable reference.
- `LOCAL_DEVELOPMENT.md` — native Windows and PowerShell workflow.
- `contracts/API.md` — human-readable semantics for the implemented HTTP API.
- `integrations/POSTGRESQL.md` — PostgreSQL ownership, migration, readiness, and failure behavior.
- `integrations/ATTACHMENT_STORAGE.md` — private byte storage, allowlists, state transitions, reconciliation, and authorized downloads.

## Planning and repository memory

- `DEVELOPMENT_PLAN.md` — canonical living plan and feature registry.
- `plans/0001-foundation.md` — detailed foundation implementation plan.
- `plans/0002-trusted-identity.md` — verified identity, session, CSRF, throttling, and audit plan.
- `plans/0003-account-administration.md` — account-lifecycle implementation plan awaiting verification.
- `plans/0004-project-access.md` — active project, membership, and versioned-geofence implementation plan.
- `plans/0005-task-register.md` — active task ownership, responsibility, date, and safe-Markdown implementation plan.
- `plans/0006-progress-updates-and-attachments.md` — active progress, evidence, revision, attachment-storage, and download plan.
- `PROJECT_STATE.md` — implemented and verified reality.
- `AI_CHANGELOG.md` — chronological agent-assisted change record.

## Architecture decisions

- `decisions/0001-modular-monolith.md`
- `decisions/0002-authentication-and-authorization.md`
- `decisions/0003-verified-evidence-and-geofence.md`
- `decisions/0004-attachment-storage.md`
- `decisions/0005-server-rendered-ui.md`
- `decisions/0006-openapi-and-api-documentation.md`
- `decisions/0007-defer-blockchain.md`
- `decisions/0008-backend-only-repository.md`
- `decisions/0009-derived-sanitized-markdown.md`

The authoritative machine-readable API contract is `../api/openapi/v1/openapi.yaml`. It is validated by `scripts/validate-openapi.ps1` and the full verification suite.
