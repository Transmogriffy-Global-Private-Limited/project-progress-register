#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage:
  rehost-ppr [-t seconds] [--no-tail] [--no-reload] [--source-dir path]

What it does:
  1. tests and builds the current PPR source tree
  2. installs the new binary atomically while retaining one previous binary
  3. delegates the guarded stop/reload/delay/start cycle to rehost-service
  4. waits until the PPR database-readiness endpoint succeeds
  5. rolls back the binary if restart or readiness fails
  6. tails service logs unless --no-tail is passed

The source directory defaults to /root/project-progress-register and can also be
set through PPR_SOURCE_DIR. Database migrations are intentionally not applied.
USAGE
}

die() {
  echo "❌ $*" >&2
  exit 1
}

delay=70
tail_logs=1
reload_systemd=1
source_dir="${PPR_SOURCE_DIR:-/root/project-progress-register}"

while (( $# > 0 )); do
  case "$1" in
    -t)
      shift
      (( $# > 0 )) || die "Missing value after -t"
      [[ "$1" =~ ^[0-9]+$ ]] || die "Invalid delay: $1. Must be a non-negative integer."
      delay="$1"
      ;;
    --no-tail)
      tail_logs=0
      ;;
    --no-reload)
      reload_systemd=0
      ;;
    --source-dir)
      shift
      (( $# > 0 )) || die "Missing value after --source-dir"
      source_dir="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown argument: $1"
      ;;
  esac
  shift
done

command -v rehost-service >/dev/null 2>&1 || die "rehost-service is not installed"
command -v curl >/dev/null 2>&1 || die "curl is not installed"
command -v flock >/dev/null 2>&1 || die "flock is not installed"
command -v go >/dev/null 2>&1 || die "go is not installed"

source_dir="$(readlink -f -- "$source_dir")"
[[ -d "$source_dir" ]] || die "Source directory does not exist: $source_dir"
[[ -f "$source_dir/go.mod" ]] || die "go.mod not found in source directory: $source_dir"
grep -qx 'module github.com/Transmogriffy-Global-Private-Limited/project-progress-register' \
  "$source_dir/go.mod" || die "Source directory is not the PPR repository: $source_dir"

exec 9>/run/lock/rehost-ppr.lock
flock -n 9 || die "Another PPR rehost is already running"

sudo -v

target=/opt/ppr/bin/ppr
backup=/opt/ppr/bin/ppr.previous
backup_next=/opt/ppr/bin/ppr.previous.next
next=/opt/ppr/bin/ppr.next
failed=/opt/ppr/bin/ppr.failed
ready_url="http://127.0.0.1:18090/backend/api/v1/health/ready"
build_dir="$(mktemp -d /tmp/ppr-rehost.XXXXXX)"
candidate="$build_dir/ppr"
candidate_installed=0
next_prepared=0
deployment_verified=0

wait_until_ready() {
  local attempt

  for attempt in {1..30}; do
    if curl --fail --silent --show-error --max-time 2 "$ready_url" >/dev/null 2>&1; then
      return 0
    fi

    systemctl is-active --quiet ppr.service || return 1
    sleep 1
  done

  return 1
}

rollback_binary() {
  echo "↩️ Restoring the previous PPR binary..." >&2
  sudo systemctl stop ppr.service

  if [[ -e "$target" ]]; then
    sudo mv -Tf -- "$target" "$failed"
  elif [[ -e "$next" ]]; then
    sudo mv -Tf -- "$next" "$failed"
  fi

  if [[ ! -e "$backup" ]]; then
    echo "❌ Previous PPR binary is unavailable at $backup" >&2
    return 1
  fi

  sudo mv -Tf -- "$backup" "$target"
  sudo chown root:root "$target"
  sudo chmod 0755 "$target"
  sudo systemctl start ppr.service

  if wait_until_ready; then
    echo "✅ Previous PPR binary restored and ready." >&2
    return 0
  fi

  echo "❌ Previous PPR binary was restored but did not become ready." >&2
  sudo journalctl -u ppr.service -n 80 --no-pager >&2
  return 1
}

finish() {
  local status=$?
  trap - EXIT
  set +e

  rm -f -- "$candidate"
  rmdir -- "$build_dir" 2>/dev/null
  if (( next_prepared == 1 )); then
    sudo rm -f -- "$next" "$backup_next"
  fi

  if (( status != 0 && candidate_installed == 1 && deployment_verified == 0 )); then
    rollback_binary || true
  fi

  exit "$status"
}
trap finish EXIT

echo "🧪 Testing PPR source: $source_dir"
(
  cd "$source_dir"
  go test ./...
)

echo "🔨 Building static PPR binary..."
(
  cd "$source_dir"
  CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$candidate" ./cmd/ppr
)
[[ -s "$candidate" && -x "$candidate" ]] || die "Build did not produce an executable binary"

echo "📦 Installing new PPR binary and retaining $backup..."
sudo install -m 0755 -o root -g root "$candidate" "$next"
next_prepared=1
[[ -x "$target" ]] || die "Current production binary is unavailable: $target"
sudo install -m 0755 -o root -g root "$target" "$backup_next"
sudo mv -Tf -- "$backup_next" "$backup"
candidate_installed=1
sudo mv -Tf -- "$next" "$target"

rehost_args=( -t "$delay" --no-tail )
if (( reload_systemd == 0 )); then
  rehost_args+=( --no-reload )
fi

rehost-service "${rehost_args[@]}" ppr.service

if ! wait_until_ready; then
  echo "❌ New PPR binary did not become ready within 30 seconds." >&2
  sudo journalctl -u ppr.service -n 80 --no-pager >&2
  exit 1
fi

deployment_verified=1
echo "✅ New PPR binary is installed and ready: ${ready_url}"

if (( tail_logs == 1 )); then
  echo "📜 Tailing logs for ppr.service. Press Ctrl+C to stop tailing."
  sudo journalctl -u ppr.service -f
fi
