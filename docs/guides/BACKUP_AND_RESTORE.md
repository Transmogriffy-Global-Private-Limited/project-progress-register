# Backup and restore

## Recovery unit

One valid recovery point contains both:

- a PostgreSQL custom-format dump, which owns schema, attachment metadata, histories, and audit; and
- a compressed archive of `ATTACHMENT_STORAGE_DIR`, which owns the opaque bytes referenced by that metadata.

`scripts/backup-ppr.sh` stops only `ppr.service` while capturing both artifacts, hashes them into `manifest.sha256`, then restarts the service if it was initially active. The maintenance stop prevents new metadata or file finalization from crossing the recovery point. Caddy and PostgreSQL remain running.

## Create a backup

Run only with explicit production-maintenance authorization:

```bash
sudo ./scripts/backup-ppr.sh /etc/ppr/ppr.env /var/backups/ppr
```

The timestamped output directory is owner-only and contains `database.dump`, `attachments.tar.gz`, `manifest.sha256`, and a non-secret manifest. The database URL remains in the protected process environment and is not passed in the command argument list. Copy completed packages to the approved encrypted off-host target. Do not commit them or keep the only copy on the application VPS.

If capture fails, the script reports its incomplete staging path and still attempts to restore the service's prior active state. Operators inspect readiness and logs before removing any incomplete artifact.

## Restore into empty targets

Restore never drops, truncates, or cleans a database. Prepare a separate empty PostgreSQL database and an absent or empty private attachment directory, configure a reviewed restore environment file, and stop `ppr.service`. Then run:

```bash
sudo ./scripts/restore-ppr.sh /var/backups/ppr/ppr-YYYYMMDDTHHMMSSZ /etc/ppr/ppr-restore.env --confirm-empty-target
```

The script verifies both SHA-256 values, refuses a running service, refuses any public database object, refuses a non-empty attachment directory, restores without source ownership/privileges, then extracts bytes. It does not start the service.

Afterward:

1. Run `ppr migrate status` with the reviewed restore environment.
2. Inspect attachment counts and a sample SHA-256 against PostgreSQL metadata.
3. Apply only reviewed forward migrations if the restored schema is older than the binary.
4. Start `ppr.service`, then verify loopback readiness and one authorized attachment download.
5. Record the restore drill date, package, duration, and evidence outside the repository without credentials.

## Retention and scheduling boundary

The repository provides the coordinated primitive, not an environment-specific retention promise. Before activation, the operator must choose backup frequency, encrypted off-host destination, retention window, alerting, and a systemd timer or equivalent. A backup is not operationally proven until an empty-target restore drill passes.
