#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"

APP_DIR="$ROOT_DIR/client_v2/app_flutter"
GO_BIN="${SLAN_GO_BIN:-/opt/homebrew/bin/go}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
SERVICE_HOST="${SLAN_MAC_IOS_SERVICE_HOST:-127.0.0.1:46395}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
TIMEOUT="${SLAN_MAC_IOS_ACTIVE_TIMEOUT:-120s}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
IOS_POST_ENABLE_WAIT_SECONDS="${SLAN_IOS_SEND_POST_ENABLE_WAIT_SECONDS:-8}"
IOS_HOLD_SECONDS="${SLAN_IOS_ACTIVE_HOLD_SECONDS:-240}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"

GENERATED_TEST_EMAIL=0
if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
else
  EMAIL="mac-ios-active-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"
WORK_DIR="${SLAN_MAC_IOS_ACTIVE_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-ios-active.XXXXXX")}"
MAC_LOG="$WORK_DIR/macos-service.log"
IOS_LOG="$WORK_DIR/ios-active.log"

PIDS=()

fail() {
  echo "$*" >&2
  exit 1
}

booted_ios_device() {
  xcrun simctl list devices booted |
    awk -F'[()]' '/Booted/ && /iPhone|iPad/ { print $2; exit }'
}

list_flutter_ios_devices() {
  flutter devices 2>/dev/null | grep -E '• ios •' || true
}

pick_single_real_ios_device() {
  local matches
  matches="$(flutter devices 2>/dev/null | grep -E '• ios •' | grep -v 'simulator' || true)"
  if [[ -z "$matches" ]]; then
    return 1
  fi
  local count
  count="$(printf '%s\n' "$matches" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "$count" != "1" ]]; then
    return 2
  fi
  printf '%s\n' "$matches" | awk -F'•' 'NR==1 {gsub(/^ +| +$/, "", $2); print $2}'
}

run_client_core_login_check() {
  local label="$1"
  shift
  local attempts="${SLAN_CONTROL_RETRY_ATTEMPTS:-3}"
  local attempt output status
  local args=("$@")
  for attempt in $(seq 1 "$attempts"); do
    set +e
    output="$(
      cd "$ROOT_DIR"
      "$GO_BIN" run scripts/client_core_service_login_check.go "${args[@]}" 2>&1
    )"
    status=$?
    set -e
    if [[ $status -eq 0 ]]; then
      echo "$output"
      return 0
    fi
    echo "$label attempt $attempt/$attempts failed: $output" >&2
    if [[ "$output" == *"HTTP 409"* ]]; then
      for index in "${!args[@]}"; do
        if [[ "${args[$index]}" == "-register=true" ]]; then
          args[$index]="-register=false"
        fi
      done
    fi
    if [[ "$attempt" != "$attempts" ]]; then
      sleep $((attempt * 5))
    fi
  done
  echo "$output"
  return "$status"
}

send_mac_udp() {
  local source_ip="$1"
  local target_ip="$2"
  local body="$3"
  python3 - "$source_ip" "$target_ip" "$UDP_PORT" "$body" <<'PY'
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
body = sys.argv[4].encode()
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(4)
try:
    s.bind((source_ip, 0))
    s.sendto(body, (target_ip, port))
    data, _ = s.recvfrom(2048)
    print(data.decode())
finally:
    s.close()
PY
}

send_mac_tcp() {
  local source_ip="$1"
  local target_ip="$2"
  local body="$3"
  python3 - "$source_ip" "$target_ip" "$TCP_PORT" "$body" <<'PY'
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
body = (sys.argv[4] + '\n').encode()
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(4)
try:
    s.bind((source_ip, 0))
    s.connect((target_ip, port))
    s.sendall(body)
    print(s.recv(2048).decode().strip())
finally:
    s.close()
PY
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_LOG" ]] && { echo "---- macOS service log ----" >&2; cat "$MAC_LOG" >&2; }
    [[ -f "$IOS_LOG" ]] && { echo "---- iOS active log ----" >&2; cat "$IOS_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
  if [[ "${SLAN_KEEP_MAC_IOS_ACTIVE_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

[[ -x "$SERVICE_BIN" ]] || fail "client-core-service binary is missing: $SERVICE_BIN"
[[ -x "$GO_BIN" ]] || fail "go binary is missing or not executable: $GO_BIN"

if [[ -z "${SLAN_IOS_FLUTTER_DEVICE:-}" ]]; then
  if detected_device="$(pick_single_real_ios_device)"; then
    SLAN_IOS_FLUTTER_DEVICE="$detected_device"
    export SLAN_IOS_FLUTTER_DEVICE
  else
    status=$?
    if [[ "$status" == "1" ]]; then
      echo "no real iOS devices detected by 'flutter devices'" >&2
    else
      echo "multiple real iOS devices detected; set SLAN_IOS_FLUTTER_DEVICE explicitly" >&2
    fi
    list_flutter_ios_devices >&2
    exit 1
  fi
fi

if flutter devices 2>/dev/null | grep -F "${SLAN_IOS_FLUTTER_DEVICE}" | grep -q 'simulator'; then
  echo "SLAN_IOS_FLUTTER_DEVICE points to an iOS simulator; a real device is required for true PacketTunnel UDP/TCP tests" >&2
  exit 1
fi

mkdir -p "$WORK_DIR/state"

echo "+ start mac client-core-service on $SERVICE_HOST"
SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
  SLAN_CONTROL_BASE_URL="$BIZ_URL" \
  SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
  SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}" \
  SLAN_STATE_DIR="$WORK_DIR/state" \
  "$SERVICE_BIN" >"$MAC_LOG" 2>&1 &
PIDS+=("$!")

echo "+ login and enable Mac service network"
MAC_OUTPUT="$(
  run_client_core_login_check "mac active login" \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register=true \
    -enable-network=true \
    -timeout "$TIMEOUT"
)"
echo "$MAC_OUTPUT"
MAC_DEVICE_ID="$(echo "$MAC_OUTPUT" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
MAC_IP="$(echo "$MAC_OUTPUT" | sed -n 's/.*virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
[[ -n "$MAC_DEVICE_ID" && -n "$MAC_IP" ]] || fail "failed to parse mac device info from login output"
MAC_IP="${MAC_IP%%/*}"

echo "+ run iOS peer echo server hold=${IOS_HOLD_SECONDS}s"
(
  cd "$APP_DIR"
  flutter test integration_test/mobile_login_test.dart \
    -d "$SLAN_IOS_FLUTTER_DEVICE" \
    --timeout "${SLAN_IOS_FLUTTER_TEST_TIMEOUT:-12m}" \
    --dart-define="SLAN_TEST_BIZ_URL=$BIZ_URL" \
    --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$BIZ_URL" \
    --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
    --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
    --dart-define="SLAN_TEST_REGISTER_USER=false" \
    --dart-define="SLAN_TEST_WAIT_MQTT=true" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$IOS_POST_ENABLE_WAIT_SECONDS" \
    --dart-define="SLAN_TEST_HOLD_SECONDS=$IOS_HOLD_SECONDS" \
    --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
    --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT"
) >"$IOS_LOG" 2>&1 &
IOS_PID="$!"
PIDS+=("$IOS_PID")

IOS_IP=""
for _ in $(seq 1 180); do
  if ! kill -0 "$IOS_PID" 2>/dev/null; then
    cat "$IOS_LOG"
    fail "iOS active socket test exited before reporting network IP"
  fi
  IOS_IP="$(sed -n 's/.*SLAN_TEST_NETWORK_IP=\([^[:space:]]*\).*/\1/p' "$IOS_LOG" | tail -n 1)"
  if [[ -n "$IOS_IP" ]]; then
    break
  fi
  sleep 1
done
[[ -n "$IOS_IP" ]] || { cat "$IOS_LOG"; fail "timed out waiting for iOS network IP"; }

for _ in $(seq 1 60); do
  if ! kill -0 "$IOS_PID" 2>/dev/null; then
    cat "$IOS_LOG"
    fail "iOS active socket test exited before echo server became ready"
  fi
  if grep -q "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" "$IOS_LOG" &&
      grep -q "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" "$IOS_LOG"; then
    break
  fi
  sleep 1
done
grep -q "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" "$IOS_LOG" \
  || { cat "$IOS_LOG"; fail "timed out waiting for iOS UDP echo server readiness"; }
grep -q "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" "$IOS_LOG" \
  || { cat "$IOS_LOG"; fail "timed out waiting for iOS TCP echo server readiness"; }
echo "iOS echo servers are ready: udp=$UDP_PORT tcp=$TCP_PORT"

slan_wait_macos_peer_route_ready "$IOS_IP" "$MAC_IP" "$SERVICE_HOST" 120 \
  || fail "mac peer route did not become stable for iOS target=${IOS_IP} source=${MAC_IP}"

UDP_RESULT="$(send_mac_udp "$MAC_IP" "$IOS_IP" "mac-to-ios-udp-$(date +%s%N)")"
[[ "$UDP_RESULT" == echo:* ]] || fail "Mac -> iOS UDP failed: $UDP_RESULT"
TCP_BODY="mac-to-ios-tcp-$(date +%s%N)"
TCP_RESULT="$(send_mac_tcp "$MAC_IP" "$IOS_IP" "$TCP_BODY")"
[[ "$TCP_RESULT" == "echo:${TCP_BODY}" ]] || fail "Mac -> iOS TCP failed: $TCP_RESULT"

kill "$IOS_PID" 2>/dev/null || true
wait "$IOS_PID" || true
cat "$IOS_LOG"

echo "macIosActiveSocketCheck: ok email=$EMAIL macIp=$MAC_IP iosIp=$IOS_IP udp=$UDP_PORT tcp=$TCP_PORT iosDevice=${SLAN_IOS_FLUTTER_DEVICE}"
