#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 || ${3:-} != "--confirm-empty-target" ]]; then
  echo "usage: restore-ppr.sh <backup-directory> <environment-file> --confirm-empty-target" >&2
  exit 2
fi
backup_dir=$1
env_file=$2
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
libpq_exec="$script_dir/libpq-env-exec.py"

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "restore-ppr.sh must run as root" >&2
  exit 1
fi
for command_name in systemctl systemd-run python3 psql pg_restore tar sha256sum realpath; do
  command -v "$command_name" >/dev/null || { echo "missing required command: $command_name" >&2; exit 1; }
done
[[ -d "$backup_dir" && -f "$backup_dir/database.dump" && -f "$backup_dir/attachments.tar.gz" && -f "$backup_dir/manifest.sha256" ]] || { echo "backup package is incomplete" >&2; exit 1; }
[[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; exit 1; }
[[ -f "$libpq_exec" ]] || { echo "libpq environment helper not found: $libpq_exec" >&2; exit 1; }
backup_dir=$(realpath -- "$backup_dir")
env_file=$(realpath -- "$env_file")
if systemctl is-active --quiet ppr.service; then
  echo "ppr.service must be stopped before restore" >&2
  exit 1
fi

(
  cd "$backup_dir"
  sha256sum --check manifest.sha256
)

systemd-run --quiet --wait --pipe --collect \
  -p "EnvironmentFile=$env_file" \
  /bin/bash -c '
    set -Eeuo pipefail
    database_url=${MIGRATION_DATABASE_URL:-${DATABASE_URL:-}}
    [[ -n "$database_url" ]] || { echo "DATABASE_URL or MIGRATION_DATABASE_URL is required" >&2; exit 1; }
    [[ -n ${ATTACHMENT_STORAGE_DIR:-} && "$ATTACHMENT_STORAGE_DIR" = /* ]] || { echo "ATTACHMENT_STORAGE_DIR must be an absolute path" >&2; exit 1; }

    public_objects=$(python3 "$3" psql -X -v ON_ERROR_STOP=1 -Atc "SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('\''r'\'','\''p'\'','\''v'\'','\''m'\'','\''S'\'','\''f'\'')")
    [[ "$public_objects" == "0" ]] || { echo "restore target database is not empty; found $public_objects public objects" >&2; exit 1; }
    if [[ -d "$ATTACHMENT_STORAGE_DIR" ]] && find "$ATTACHMENT_STORAGE_DIR" -mindepth 1 -print -quit | grep -q .; then
      echo "attachment restore target is not empty: $ATTACHMENT_STORAGE_DIR" >&2
      exit 1
    fi
    install -d -m 0750 -- "$ATTACHMENT_STORAGE_DIR"

    pg_restore --no-owner --no-privileges --file=- "$1" |
      python3 "$3" psql -X -v ON_ERROR_STOP=1
    tar --extract --gzip --file="$2" --directory="$ATTACHMENT_STORAGE_DIR"
  ' _ "$backup_dir/database.dump" "$backup_dir/attachments.tar.gz" "$libpq_exec"

echo "PPR restore completed into the confirmed empty targets; run migration status and readiness checks before starting ppr.service"
