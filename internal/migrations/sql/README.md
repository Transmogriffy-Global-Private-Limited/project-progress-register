# Embedded SQL migrations

Add forward-only migrations here using `NNNNNN_lowercase_name.sql` filenames.

Running `ppr migrate up` initializes the checksummed `ppr_schema_migrations` ledger and applies pending files in order. Business migrations arrive only with their consuming backend slice; applied files are immutable and corrections require a new forward migration. Current migrations cover identity, account administration, project access/geofences, tasks, progress/attachment evidence, immutable review, before/after task history, and plural task responsibilities.
