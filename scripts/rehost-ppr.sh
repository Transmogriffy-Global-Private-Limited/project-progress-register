#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage:
  rehost-ppr [-t seconds] [--no-tail] [--no-reload]

What it does:
  1. delegates the guarded stop/reload/delay/start cycle to rehost-service
  2. waits until the PPR database-readiness endpoint succeeds
  3. tails service logs unless --no-tail is passed
USAGE
}

die() {
  echo "❌ $*" >&2
  exit 1
}

delay=70
tail_logs=1
reload_systemd=1

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

rehost_args=( -t "$delay" --no-tail )
if (( reload_systemd == 0 )); then
  rehost_args+=( --no-reload )
fi

rehost-service "${rehost_args[@]}" ppr.service

ready_url="http://127.0.0.1:18090/backend/api/v1/health/ready"
for attempt in {1..30}; do
  if curl --fail --silent --show-error --max-time 2 "$ready_url" >/dev/null 2>&1; then
    echo "✅ PPR is ready: ${ready_url}"
    if (( tail_logs == 1 )); then
      echo "📜 Tailing logs for ppr.service. Press Ctrl+C to stop tailing."
      sudo journalctl -u ppr.service -f
    fi
    exit 0
  fi

  if ! systemctl is-active --quiet ppr.service; then
    echo "❌ ppr.service stopped before becoming ready." >&2
    sudo journalctl -u ppr.service -n 80 --no-pager >&2
    exit 1
  fi
  sleep 1
done

echo "❌ ppr.service did not become ready within 30 seconds." >&2
sudo journalctl -u ppr.service -n 80 --no-pager >&2
exit 1
