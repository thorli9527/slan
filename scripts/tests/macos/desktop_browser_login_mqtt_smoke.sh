#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client_v2/rust/target/aarch64-apple-darwin/release/client-core-service}"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46402}"
WEB_BASE="${SLAN_WEB_BASE_URL:-http://47.245.40.231:24200}"
CONTROL_BASE="${SLAN_CONTROL_BASE_URL:-http://47.245.40.231:28080}"
STATE_DIR="${SLAN_TEST_STATE_DIR:-$(mktemp -d /tmp/slan-browser-login.XXXXXX)}"
EMAIL="${SLAN_TEST_EMAIL:-browser-login-$(date +%s)@example.test}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
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

prepared=$(request dispatch '{"type":"openClientLogin"}')
device_id=$(jq -r '.deviceId // empty' <<<"$prepared")
[[ -n "$device_id" ]] || {
  echo "openClientLogin did not return deviceId: $prepared" >&2
  cat "$LOG_FILE" >&2
  exit 1
}

auth=$(curl -fsS -X POST "$WEB_BASE/api/web/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Browser Login Smoke\"}")
access_token=$(jq -r '.auth.session.token // .auth.session.accessToken // empty' <<<"$auth")
[[ -n "$access_token" ]] || {
  echo "register did not return access token: $auth" >&2
  exit 1
}

curl -fsS -X POST "$WEB_BASE/api/web/auth/device-login-devices/$device_id/complete" \
  -H 'Content-Type: application/json' \
  -d "{\"accessToken\":\"$access_token\"}" >/dev/null

state=""
session=""
for _ in $(seq 1 60); do
  state=$(request localState || true)
  session=$(request localSession || true)
  if jq -e '.signedIn == true' >/dev/null 2>&1 <<<"$state" &&
    jq -e '.signedIn == true and (.virtualIp | startswith("10."))' >/dev/null 2>&1 <<<"$session"; then
    echo "ok - desktop browser login completed through MQTT deviceId=$device_id virtualIp=$(jq -r '.virtualIp' <<<"$session")"
    exit 0
  fi
  sleep 0.5
done

echo "desktop did not automatically sign in: state=$state session=$session" >&2
cat "$LOG_FILE" >&2
exit 1
