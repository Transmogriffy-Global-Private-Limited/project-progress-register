#!/usr/bin/env bash
set -Eeuo pipefail

env_file=${1:-/etc/ppr/ppr.env}
backup_root=${2:-/var/backups/ppr}
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
libpq_exec="$script_dir/libpq-env-exec.py"

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "backup-ppr.sh must run as root so it can coordinate ppr.service and private storage" >&2
  exit 1
fi
for command_name in systemctl systemd-run python3 pg_dump psql tar sha256sum mktemp realpath; do
  command -v "$command_name" >/dev/null || { echo "missing required command: $command_name" >&2; exit 1; }
done
[[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; exit 1; }
[[ -f "$libpq_exec" ]] || { echo "libpq environment helper not found: $libpq_exec" >&2; exit 1; }
env_file=$(realpath -- "$env_file")

install -d -m 0700 -- "$backup_root"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
temporary=$(mktemp -d "$backup_root/.ppr-$stamp-XXXXXX")
destination="$backup_root/ppr-$stamp"
service_was_active=false

cleanup() {
  status=$?
  if [[ "$service_was_active" == true ]]; then
    systemctl start ppr.service || status=1
  fi
  if [[ $status -ne 0 ]]; then
    echo "backup failed; incomplete staging remains at $temporary" >&2
  fi
  exit "$status"
}
trap cleanup EXIT

if systemctl is-active --quiet ppr.service; then
  service_was_active=true
  systemctl stop ppr.service
fi

systemd-run --quiet --wait --pipe --collect \
  -p "EnvironmentFile=$env_file" \
  /bin/bash -c '
    set -Eeuo pipefail
    database_url=${MIGRATION_DATABASE_URL:-${DATABASE_URL:-}}
    [[ -n "$database_url" ]] || { echo "DATABASE_URL or MIGRATION_DATABASE_URL is required" >&2; exit 1; }
    [[ -n ${ATTACHMENT_STORAGE_DIR:-} && "$ATTACHMENT_STORAGE_DIR" = /* ]] || { echo "ATTACHMENT_STORAGE_DIR must be an absolute path" >&2; exit 1; }
    [[ -d "$ATTACHMENT_STORAGE_DIR" ]] || { echo "attachment directory not found: $ATTACHMENT_STORAGE_DIR" >&2; exit 1; }
    python3 "$3" pg_dump --format=custom --file="$1"
    python3 "$3" psql -X -v ON_ERROR_STOP=1 -Atc "SELECT lpad(max(version)::text, 6, '\''0'\'') FROM public.ppr_schema_migrations" > "$4"
    tar --create --gzip --file="$2" --directory="$ATTACHMENT_STORAGE_DIR" .
  ' _ "$temporary/database.dump" "$temporary/attachments.tar.gz" "$libpq_exec" "$temporary/schema-contract.txt"
schema_contract=$(<"$temporary/schema-contract.txt")
[[ "$schema_contract" =~ ^[0-9]{6}$ ]] || { echo "backup failed to resolve the applied schema contract" >&2; exit 1; }
unlink -- "$temporary/schema-contract.txt"
(
  cd "$temporary"
  sha256sum database.dump attachments.tar.gz > manifest.sha256
)
printf 'created_at_utc=%s\nschema_contract=%s\n' "$stamp" "$schema_contract" > "$temporary/manifest.txt"
chmod -R go-rwx "$temporary"
mv -- "$temporary" "$destination"
temporary="$destination"

echo "coordinated PPR backup created at $destination"
