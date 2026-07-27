# Private attachment storage integration

## Purpose and ownership

`internal/filestore` owns attachment bytes. `internal/progress` owns attachment metadata, evidence, authorization, and state transitions. PostgreSQL is authoritative for whether a file is `pending`, `available`, or `failed`; the filesystem is authoritative only for the opaque bytes named by a storage key.

## Configuration

- `ATTACHMENT_STORAGE_DIR` — private root; default `.local/attachments`
- `ATTACHMENT_MAX_FILE_BYTES` — 1 MiB through 1 GiB; default 100 MiB
- `ATTACHMENT_MAX_FILES_PER_UPDATE` — 1 through 25; default 10

Relative roots resolve from the process working directory. The root must not be served by Caddy or any static file handler. The application creates root, `.staging`, and sharded `data` directories with owner-only permissions where the platform supports them.

## Accepted formats

- Images: JPEG, PNG, GIF, WebP
- Documents: PDF, plain text (`.txt`/`.md`), CSV, DOC/DOCX, XLS/XLSX, PPT/PPTX, ODT/ODS/ODP
- Videos: MP4, WebM, QuickTime MOV, AVI

Detection uses file bytes. Office ZIP/OLE containers additionally require a matching allowlisted extension. Original filenames and reported MIME values are untrusted display metadata and never choose a storage path.

## Durable write sequence

```text
authenticate and authorize the active project/task scope
-> require valid browser upload location for every file
-> stream file to .staging/<random-key>.part
-> enforce configured byte limit while streaming
-> detect allowlisted type and calculate SHA-256
-> commit progress plus pending attachment metadata in PostgreSQL
-> atomically rename within the same storage volume to data/<prefix>/<key>
-> mark attachment available and append audit
```

If the database transaction fails, newly staged files are discarded. After the database owns a pending row, staged bytes are retained for reconciliation. If the final file already exists, finalization is idempotent.

## Startup and recovery

The runtime starts a reconciliation loop immediately and repeats every minute. Each pending row is finalized from staging, or recognized as already final, then marked available. If neither staged nor final bytes exist, the row becomes failed with a generic reason. Transient filesystem errors leave the row pending and stop that reconciliation pass so a later pass can retry. Database unavailability likewise postpones reconciliation without disabling liveness.

Staging files not referenced by a pending row are retained for 24 hours so in-flight requests are never disturbed, then removed as crash orphans during reconciliation.

## Downloads and security

Downloads use the nested authorized API `content_path`. The HTTP transport constructs this same-origin path from the authorized resource identifiers and prepends the configured `BASE_PATH`; production values therefore begin `/backend/api/v1/`. The backend rechecks current project access, refuses non-available state, opens only a validated opaque key, records download intent in audit, and sends `Content-Disposition: attachment`, detected MIME, `Cache-Control: private, no-store`, and `X-Content-Type-Options: nosniff`.

No endpoint exposes the storage root or key. V1 does not execute, render inline, transcode, thumbnail, extract archives, or run a malware-scanning service.

## Backup and restore

Database and filesystem backups represent one coordinated recovery point. `scripts/backup-ppr.sh` performs a maintenance stop, captures a custom PostgreSQL dump plus attachment archive, hashes both, and restores prior service state. `scripts/restore-ppr.sh` verifies hashes and refuses non-empty targets. Losing the filesystem cannot be repaired from PostgreSQL hashes alone. See `../guides/BACKUP_AND_RESTORE.md`; off-host scheduling, retention, and the first restore drill remain operator actions rather than hidden application behavior.
