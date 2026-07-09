#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

REMOTE_HOST="${SLAN_REMOTE_RECOVERY_HOST:-47.245.40.231}"
REMOTE_SSH_PORT="${SLAN_REMOTE_RECOVERY_SSH_PORT:-22}"
REMOTE_HTTP_URL="${SLAN_REMOTE_RECOVERY_HTTP_URL:-http://47.245.40.231:28080/downloads/clients/install.sh}"
RECOVER_SCRIPT="${SLAN_REMOTE_RECOVERY_SCRIPT:-$ROOT_DIR/scripts/tests/linux/linux_remote_recover_publish_check.sh}"
SLEEP_SECONDS="${SLAN_REMOTE_RECOVERY_SLEEP_SECONDS:-15}"
MAX_ATTEMPTS="${SLAN_REMOTE_RECOVERY_MAX_ATTEMPTS:-0}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

probe_ssh() {
  if command -v nc >/dev/null 2>&1; then
    nc -z -w 3 "$REMOTE_HOST" "$REMOTE_SSH_PORT" >/dev/null 2>&1
    return
  fi
  ssh -o BatchMode=yes -o ConnectTimeout=3 -p "$REMOTE_SSH_PORT" "root@${REMOTE_HOST}" 'exit 0' >/dev/null 2>&1
}

probe_http() {
  curl --max-time 5 -fsSI "$REMOTE_HTTP_URL" >/dev/null 2>&1
}

need curl
[[ -x "$RECOVER_SCRIPT" ]] || fail "missing recovery script: $RECOVER_SCRIPT"

attempt=0
while :; do
  attempt=$((attempt + 1))
  log "probe attempt #$attempt host=$REMOTE_HOST sshPort=$REMOTE_SSH_PORT"
  ssh_ok=0
  http_ok=0
  if probe_ssh; then
    ssh_ok=1
  fi
  if probe_http; then
    http_ok=1
  fi
  if [[ "$ssh_ok" == "1" && "$http_ok" == "1" ]]; then
    log "remote host recovered; starting publish verification"
    exec "$RECOVER_SCRIPT"
  fi
  if [[ "$MAX_ATTEMPTS" -gt 0 && "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
    fail "remote host did not recover after $attempt attempts"
  fi
  log "remote not ready yet ssh=$ssh_ok http=$http_ok; sleeping ${SLEEP_SECONDS}s"
  sleep "$SLEEP_SECONDS"
done
