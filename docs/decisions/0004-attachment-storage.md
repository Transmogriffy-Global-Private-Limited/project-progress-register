# ADR 0004 — Local filesystem attachment storage

Status: Accepted; implemented, automated-verified, migrated, and deployed; guarded database-live lifecycle/restore verification pending

Date: 2026-07-27

## Context

V1 runs on one small VPS and needs camera images plus uploaded images, documents, and videos. PostgreSQL should own searchable durable metadata, but storing large file bytes in business tables would increase database and backup burden and obstruct later storage replacement.

## Decision

Store attachment bytes under a configurable private local filesystem root and metadata in PostgreSQL. Application services depend on a narrow storage boundary introduced with the upload slice. Use opaque storage identifiers, staging, streamed size/MIME/hash validation, atomic same-volume rename, explicit metadata state, and restart reconciliation.

All downloads pass backend project authorization and are streamed through the application with attachment disposition. No upload path is executable, publicly browsable, or derived directly from the original filename.

## Consequences

- V1 remains operationally simple and low-memory.
- Database and file backups must be coordinated and documented.
- Filesystem/database partial failure is explicit and recoverable rather than hidden.
- A later object-storage implementation can replace the byte backend without changing project authorization or metadata semantics.
