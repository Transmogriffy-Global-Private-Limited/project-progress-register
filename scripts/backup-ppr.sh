#!/usr/bin/env bash
set -Eeuo pipefail

env_file=${1:-/etc/ppr/ppr.env}
backup_root=${2:-/var/backups/ppr}

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "backup-ppr.sh must run as root so it can coordinate ppr.service and private storage" >&2
  exit 1
fi
for command_name in systemctl pg_dump tar sha256sum mktemp; do
  command -v "$command_name" >/dev/null || { echo "missing required command: $command_name" >&2; exit 1; }
done
[[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; exit 1; }

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

database_url=${MIGRATION_DATABASE_URL:-${DATABASE_URL:-}}
[[ -n "$database_url" ]] || { echo "DATABASE_URL or MIGRATION_DATABASE_URL is required" >&2; exit 1; }
[[ -n ${ATTACHMENT_STORAGE_DIR:-} && "$ATTACHMENT_STORAGE_DIR" = /* ]] || { echo "ATTACHMENT_STORAGE_DIR must be an absolute path" >&2; exit 1; }
[[ -d "$ATTACHMENT_STORAGE_DIR" ]] || { echo "attachment directory not found: $ATTACHMENT_STORAGE_DIR" >&2; exit 1; }

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

PGDATABASE="$database_url" pg_dump --format=custom --file="$temporary/database.dump"
tar --create --gzip --file="$temporary/attachments.tar.gz" --directory="$ATTACHMENT_STORAGE_DIR" .
(
  cd "$temporary"
  sha256sum database.dump attachments.tar.gz > manifest.sha256
)
printf 'created_at_utc=%s\nschema_contract=000007\n' "$stamp" > "$temporary/manifest.txt"
chmod -R go-rwx "$temporary"
mv -- "$temporary" "$destination"
temporary="$destination"

echo "coordinated PPR backup created at $destination"
