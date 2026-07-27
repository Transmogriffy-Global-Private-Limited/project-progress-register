#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 || ${3:-} != "--confirm-empty-target" ]]; then
  echo "usage: restore-ppr.sh <backup-directory> <environment-file> --confirm-empty-target" >&2
  exit 2
fi
backup_dir=$1
env_file=$2

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "restore-ppr.sh must run as root" >&2
  exit 1
fi
for command_name in systemctl psql pg_restore tar sha256sum; do
  command -v "$command_name" >/dev/null || { echo "missing required command: $command_name" >&2; exit 1; }
done
[[ -d "$backup_dir" && -f "$backup_dir/database.dump" && -f "$backup_dir/attachments.tar.gz" && -f "$backup_dir/manifest.sha256" ]] || { echo "backup package is incomplete" >&2; exit 1; }
[[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; exit 1; }
if systemctl is-active --quiet ppr.service; then
  echo "ppr.service must be stopped before restore" >&2
  exit 1
fi

(
  cd "$backup_dir"
  sha256sum --check manifest.sha256
)

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

database_url=${MIGRATION_DATABASE_URL:-${DATABASE_URL:-}}
[[ -n "$database_url" ]] || { echo "DATABASE_URL or MIGRATION_DATABASE_URL is required" >&2; exit 1; }
[[ -n ${ATTACHMENT_STORAGE_DIR:-} && "$ATTACHMENT_STORAGE_DIR" = /* ]] || { echo "ATTACHMENT_STORAGE_DIR must be an absolute path" >&2; exit 1; }

public_objects=$(psql "$database_url" -X -v ON_ERROR_STOP=1 -Atc "SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f')")
[[ "$public_objects" == "0" ]] || { echo "restore target database is not empty; found $public_objects public objects" >&2; exit 1; }
if [[ -d "$ATTACHMENT_STORAGE_DIR" ]] && find "$ATTACHMENT_STORAGE_DIR" -mindepth 1 -print -quit | grep -q .; then
  echo "attachment restore target is not empty: $ATTACHMENT_STORAGE_DIR" >&2
  exit 1
fi
install -d -m 0750 -- "$ATTACHMENT_STORAGE_DIR"

pg_restore --exit-on-error --no-owner --no-privileges --dbname="$database_url" "$backup_dir/database.dump"
tar --extract --gzip --file="$backup_dir/attachments.tar.gz" --directory="$ATTACHMENT_STORAGE_DIR"

echo "PPR restore completed into the confirmed empty targets; run migration status and readiness checks before starting ppr.service"
