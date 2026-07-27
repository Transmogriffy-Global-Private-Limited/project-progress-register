# Embedded SQL migrations

Add forward-only migrations here using `NNNNNN_lowercase_name.sql` filenames.

The foundation deliberately contains no unused domain tables. Running `ppr migrate up` initializes the checksummed `ppr_schema_migrations` ledger; the first business migration will arrive with the identity and audit vertical slice.
