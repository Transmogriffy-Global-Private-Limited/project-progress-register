# Documentation index

This index is the entry point to the authoritative Project Progress Register documentation. A document marked as planned describes approved direction; `PROJECT_STATE.md` alone owns claims about what is currently implemented.

## Product and concepts

- `PRODUCT_REQUIREMENTS.md` — approved product purpose, scope, workflows, and non-goals.
- `DOMAIN_MODEL.md` — entity ownership, relationships, invariants, and lifecycle.
- `PERMISSIONS.md` — Admin and Member authorization matrix and enforcement rules.
- `EVIDENCE_AND_TRUST_MODEL.md` — camera, geolocation, metadata, hashing, and tamper-evidence boundaries.
- `guides/SAFE_MARKDOWN.md` — Markdown source-of-truth, sanitization, frontend contract, limits, and recovery.
- `guides/PROGRESS_EVIDENCE.md` — upload geotags, per-file verification, multipart retries, and recovery.
- `guides/REVIEW_WORKFLOW.md` — comments, accepted suggestions, assessments, authorization, conflicts, and frontend recovery.
- `guides/TASK_TIMELINE.md` — authorized audit-style task chronology, metadata, reconstruction, and trust boundary.
- `integrations/FRONTEND_INTEGRATION.md` — self-contained “read this and build the FE” guide covering every browser workflow, endpoint, payload, response, permission, error, retry, and rendering rule.

## Architecture and operations

- `ARCHITECTURE.md` — modular-monolith boundaries, runtime topology, startup, persistence, and request flows.
- `CONFIGURATION.md` — complete environment-variable reference.
- `LOCAL_DEVELOPMENT.md` — native Windows and PowerShell workflow.
- `guides/PRODUCTION_DEPLOYMENT.md` — Ubuntu systemd/Caddy hosting, database reset safeguards, bootstrap removal, verification, and rollback.
- `guides/BACKUP_AND_RESTORE.md` — coordinated maintenance backup, empty-target restore, validation, recovery, and scheduling boundary.
- `contracts/API.md` — human-readable semantics for the implemented HTTP API.
- `integrations/POSTGRESQL.md` — PostgreSQL ownership, migration, readiness, and failure behavior.
- `integrations/ATTACHMENT_STORAGE.md` — private byte storage, allowlists, state transitions, reconciliation, and authorized downloads.

## Planning and repository memory

- `DEVELOPMENT_PLAN.md` — canonical living plan and feature registry.
- `plans/0001-foundation.md` — detailed foundation implementation plan.
- `plans/0002-trusted-identity.md` — verified identity, session, CSRF, throttling, and audit plan.
- `plans/0003-account-administration.md` — implemented account-lifecycle plan with guarded database-live verification pending.
- `plans/0004-project-access.md` — implemented project, membership, and versioned-geofence plan with guarded database-live verification pending.
- `plans/0005-task-register.md` — implemented task ownership, responsibility, date, and safe-Markdown plan with guarded database-live verification pending.
- `plans/0006-progress-updates-and-attachments.md` — implemented progress, evidence, revision, attachment-storage, and download plan with guarded database-live verification pending.
- `plans/0007-production-hosting.md` — `/backend` base path, production database reset, systemd/Caddy hosting, and public verification.
- `plans/0008-review-comments-suggestions-assessments.md` — implemented immutable comments, accepted suggestions, and Admin assessment workflow awaiting deferred verification.
- `plans/0009-reporting-audit-and-recovery.md` — implemented/deployed dashboard, task timeline, complete Admin audit, and guarded backup/restore plan; restore drill pending.
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

The authoritative machine-readable API contract is `../api/openapi/v1/openapi.yaml`. It is validated by `scripts/validate-openapi.ps1` and the full verification suite. `scripts/verify-fe-integration-docs.ps1` also fails when an OpenAPI operation or path is absent from the complete frontend handoff.
