#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client/rust/target/aarch64-apple-darwin/release/client-core-service}"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46402}"
CONTROL_BASE="${SLAN_CONTROL_BASE_URL:-http://47.245.40.231:28080}"
STATE_DIR="${SLAN_TEST_STATE_DIR:-$(mktemp -d /tmp/slan-device-activation.XXXXXX)}"
AUTHORIZATION_KEY="${SLAN_DEVICE_AUTHORIZATION_KEY:-}"
LOG_FILE="$STATE_DIR/service.log"

host=${SERVICE_HOST%:*}
port=${SERVICE_HOST##*:}
service_pid=""

cleanup() {
  if [[ -n "$service_pid" ]]; then
    kill "$service_pid" >/dev/null 2>&1 || true
    wait "$service_pid" >/dev/null 2>&1 || true
  fi
  if [[ -z "${SLAN_TEST_STATE_DIR:-}" ]]; then
    rm -rf "$STATE_DIR"
  fi
}
trap cleanup EXIT

request() {
  local method=$1
  local args=${2:-}
  if [[ -z "$args" ]]; then
    args='{}'
  fi
  printf '{"method":"%s","args":%s}\n' "$method" "$args" | nc -w 5 "$host" "$port"
}

[[ -x "$SERVICE_BIN" ]] || {
  echo "missing service binary: $SERVICE_BIN" >&2
  exit 1
}
[[ -n "$AUTHORIZATION_KEY" ]] || {
  echo "SLAN_DEVICE_AUTHORIZATION_KEY is required" >&2
  exit 1
}
mkdir -p "$STATE_DIR"

SLAN_STATE_DIR="$STATE_DIR" \
SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
SLAN_CONTROL_BASE_URL="$CONTROL_BASE" \
SLAN_MACOS_NETWORK_MOCK=1 \
  "$SERVICE_BIN" >"$LOG_FILE" 2>&1 &
service_pid=$!

for _ in $(seq 1 30); do
  if nc -z "$host" "$port" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

activated=$(request dispatch "$(jq -cn --arg key "$AUTHORIZATION_KEY" '{type:"localActivateDevice",payload:{key:$key}}')")
device_id=$(jq -r '.deviceId // empty' <<<"$activated")
[[ -n "$device_id" ]] || {
  echo "device activation did not return deviceId: $activated" >&2
  cat "$LOG_FILE" >&2
  exit 1
}

state=""
session=""
for _ in $(seq 1 60); do
  state=$(request localState || true)
  session=$(request localSession || true)
  if jq -e '.activated == true' >/dev/null 2>&1 <<<"$state" &&
    jq -e '.activated == true and (.virtualIp | startswith("10."))' >/dev/null 2>&1 <<<"$session"; then
    control=$(request localControlStatus || true)
    if jq -e '.ready == true' >/dev/null 2>&1 <<<"$control"; then
      echo "ok - desktop device activation completed deviceId=$device_id virtualIp=$(jq -r '.virtualIp' <<<"$session")"
      exit 0
    fi
  fi
  sleep 0.5
done

echo "desktop did not reach activated control-ready state: state=$state session=$session" >&2
cat "$LOG_FILE" >&2
exit 1
