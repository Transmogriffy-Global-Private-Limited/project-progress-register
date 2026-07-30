# Plan 0006 — Progress updates, revisions, evidence, and attachments

Status: Implemented; automated and base-path correction verification pass, database-live verification pending

## Objective

Deliver the central backend diary workflow as one coherent slice: chronological task progress, immutable edit history, optional location for text-only updates, mandatory upload-location evidence for every file, mixed image/document/video attachments, private recoverable byte storage, authorized downloads, audit, REST/OpenAPI contracts, verification assets, and authoritative documentation.

## Approved decisions

- A current project Member may add progress to any task they can access. Admins have the same project-wide access. Inactive projects remain readable but reject progress creation and editing.
- Members may edit only progress updates they created; Admins may edit any update. Each edit appends an immutable before/after revision with editor and server time.
- Location never gates a text-only progress submission, and geofence verification never gates a submission. File upload does require a valid browser-reported latitude, longitude, and positive accuracy so every stored file has a geotag. Every update receives one explicit evidence result: `verified`, `unverified_outside`, `unverified_accuracy`, `unverified_no_geofence`, `unverified_unavailable`, or `not_supplied`.
- Valid reported coordinates, accuracy, and browser-observed time are preserved even when the result is unverified. The current geofence and computed distance are snapshotted when available. Server receipt time is authoritative; browser and embedded metadata remain untrusted/reported evidence. The location result and each attachment's verification result are separate facts.
- Update creation uses `multipart/form-data`: one JSON `metadata` part plus zero through ten repeated `files` parts. Every attachment inherits the update's required upload-location snapshot. Camera captures and existing-file uploads are both accepted when coordinates exist, including outside-geofence, inaccurate, or no-geofence results; absent/unavailable location permits only a text update.
- Verification is per attachment. A direct image or video reported as an in-Chrome camera capture may be `verified` only when its upload location passes server-side accuracy and geofence evaluation. Gallery/existing-file images/videos and every document are always `non_verified` with an explicit reason while retaining the same geotag.
- V1 accepts common images, office/text/PDF documents, and browser-friendly videos. Each file is limited to 100 MiB and is streamed; request bodies are not buffered wholesale.
- Attachment bytes use configurable private local storage. PostgreSQL owns metadata and state. The application stages bytes, hashes and detects type, commits `pending` metadata, atomically renames bytes, marks them `available`, and reconciles interrupted pending rows at startup.
- Original filenames are display metadata only. Opaque random storage keys determine filesystem paths. Content reads require current project authorization; videos use inline byte-range streaming while other media uses attachment disposition, all with `nosniff`.
- Attachments are immutable in this slice. There is no attachment deletion, transcoding, thumbnailing, antivirus service, EXIF parsing, object storage, or dashboard freshness rule.

## Surface map

- Configuration: storage root and bounded file/count limits
- Migration: progress updates, immutable revisions, evidence snapshots, attachment metadata/state, constraints, and indexes
- Progress module: validation, Haversine evaluation, ownership, revisions, attachment orchestration, reconciliation, and audit
- Filesystem module: streaming staging, SHA-256, type allowlist, atomic finalize, authorized open, and orphan cleanup boundary
- HTTP: nested list/create/detail/update and attachment-content download routes
- Runtime: storage construction and startup reconciliation before accepting requests
- Contracts: multipart schemas, evidence semantics, revision history, attachment metadata, error responses, and download behavior
- Verification: evidence/storage/domain/HTTP tests, route drift coverage, guarded disposable live script, residue scan, and full suite
- Documentation: product, trust model, architecture, domain, permissions, configuration, API, storage integration, state, plan, changelog, and ADRs

## Routes

- `GET /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates`
- `POST /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates`
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}`
- `PATCH /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}`
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/attachments/{attachment_id}/content`

## Acceptance criteria

- Authorized task readers receive chronological current updates; detail includes immutable revisions and attachment metadata.
- Creation accepts written Markdown without location. Allowed image/document/video files require reported coordinates but do not require verified geofence status.
- Any valid supplied geotag is stored, evaluated server-side, and returned with its explicit trust/result reason. Failed geofence verification never silently becomes verified and never blocks camera or existing-file bytes; missing coordinates block file bytes but not a text-only update. Only qualifying direct Chrome-camera photos/videos receive attachment status `verified`; everything else is `non_verified` with its source/type/location reason.
- Inaccessible, mismatched, and non-owner mutation identifiers return the same scoped `404`.
- Member edits preserve immutable revisions; optimistic conflicts return `409` without mutation.
- Files are bounded, streamed, hashed, server-classified, privately stored, and never addressed by original filename.
- Interrupted pending file finalization is recoverable and does not expose partial bytes as available.
- Downloads require current project access, return a deployment-prefix-aware same-origin path, and audit successful delivery intent without exposing storage paths.
- OpenAPI and human contracts are sufficient for the separate frontend to build the complete workflow.

## Verification

Automated verification completed on 2026-07-27. Focused progress/filestore/HTTP tests, OpenAPI validation and route coverage, the full repository verifier, race detection, and both loopback API-documentation smoke modes pass. Migration application and `verify-live-progress.ps1` remain pending because they require an explicitly disposable, fully migrated database with zero users.

The later correction that moves `content_path` construction to the prefix-aware HTTP boundary passes focused HTTP coverage, the full repository verifier, and the prefixed loopback smoke state.

- Focused evidence classification and boundary tests
- Streaming storage/type/size/finalize/reconcile tests using temporary directories
- Service ownership, revision, location, and failure-path tests
- HTTP multipart, CSRF, scoped path, download header, and error tests
- OpenAPI validation and registered-route coverage
- Guarded zero-user disposable database lifecycle script
- Formatter, vet, build, race detector, loopback smoke checks, residue scan, full suite, and Git checks

## Remaining operational decisions

- Production retention and coordinated database/filesystem backup policy belongs to operational hardening.
- Malware scanning can be added behind the storage boundary when an actual scanner/operations requirement exists; V1 uses a strict allowlist and forced download semantics.
