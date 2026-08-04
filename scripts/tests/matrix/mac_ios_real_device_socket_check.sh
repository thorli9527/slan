#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/lib/flutter_mobile_activation_test.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"

APP_DIR="$ROOT_DIR/client/app_flutter"
GO_BIN="${SLAN_GO_BIN:-/opt/homebrew/bin/go}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
SERVICE_HOST="${SLAN_MAC_IOS_SERVICE_HOST:-127.0.0.1:46395}"
TIMEOUT="${SLAN_MAC_IOS_SOCKET_TIMEOUT:-90s}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-ios-to-mac-udp-$(date +%s%N)}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-ios-to-mac-tcp-$(date +%s%N)}"
IOS_POST_ENABLE_WAIT_SECONDS="${SLAN_IOS_SEND_POST_ENABLE_WAIT_SECONDS:-8}"
IOS_PEER_HOLD_SECONDS="${SLAN_IOS_PEER_HOLD_SECONDS:-240}"

WORK_DIR="${SLAN_MAC_IOS_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-ios-socket.XXXXXX")}"
MAC_LOG="$WORK_DIR/macos-service.log"
ECHO_LOG="$WORK_DIR/macos-echo.log"
IOS_LOG="$WORK_DIR/ios-socket.log"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
IOS_TEST_DEVICE_ID="${SLAN_IOS_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"

PIDS=()
OPS_TOKEN=""
NETWORK_ID=""
MAC_CREDENTIAL_ID=""
MAC_AUTHORIZATION_KEY=""
IOS_CREDENTIAL_ID=""
IOS_AUTHORIZATION_KEY=""

provision_ops_resources() {
  local credential network device_id
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac iOS Real Device Mac" "$MAC_TEST_DEVICE_ID")"
  MAC_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  MAC_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac iOS Real Device iOS" "$IOS_TEST_DEVICE_ID")"
  IOS_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  IOS_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"$MAC_AUTHORIZATION_KEY\",\"deviceId\":\"$MAC_TEST_DEVICE_ID\"}" >/dev/null
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"$IOS_AUTHORIZATION_KEY\",\"deviceId\":\"$IOS_TEST_DEVICE_ID\"}" >/dev/null
  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "mac-ios-real-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network" | jq -er '.networkId')"
  for device_id in "$MAC_TEST_DEVICE_ID" "$IOS_TEST_DEVICE_ID"; do
    slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$device_id"
  done
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

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_LOG" ]] && { echo "---- macOS service log ----" >&2; cat "$MAC_LOG" >&2; }
    [[ -f "$ECHO_LOG" ]] && { echo "---- macOS echo log ----" >&2; cat "$ECHO_LOG" >&2; }
    [[ -f "$IOS_LOG" ]] && { echo "---- iOS socket log ----" >&2; cat "$IOS_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$MAC_CREDENTIAL_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$IOS_CREDENTIAL_ID" >/dev/null 2>&1 || true
  if [[ "${SLAN_KEEP_MAC_IOS_SOCKET_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [[ ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  exit 1
fi

if [[ -z "${SLAN_IOS_FLUTTER_DEVICE:-}" ]]; then
  if detected_device="$(pick_single_real_ios_device)"; then
    SLAN_IOS_FLUTTER_DEVICE="$detected_device"
    export SLAN_IOS_FLUTTER_DEVICE
    echo "==> auto-selected real iOS device: ${SLAN_IOS_FLUTTER_DEVICE}"
  else
    status=$?
    echo "missing SLAN_IOS_FLUTTER_DEVICE" >&2
    if [[ "$status" == "1" ]]; then
      echo "no real iOS devices detected by 'flutter devices'" >&2
    else
      echo "multiple real iOS devices detected; set SLAN_IOS_FLUTTER_DEVICE explicitly" >&2
    fi
    echo "visible iOS devices:" >&2
    list_flutter_ios_devices >&2
    exit 1
  fi
fi

if flutter devices 2>/dev/null | grep -F "${SLAN_IOS_FLUTTER_DEVICE}" | grep -q 'simulator'; then
  echo "SLAN_IOS_FLUTTER_DEVICE points to an iOS simulator; a real device is required for true PacketTunnel UDP/TCP tests" >&2
  echo "visible iOS devices:" >&2
  list_flutter_ios_devices >&2
  exit 1
fi

mkdir -p "$WORK_DIR/state"
provision_ops_resources

echo "+ start mac client-core-service on $SERVICE_HOST"
SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
  SLAN_CONTROL_BASE_URL="$BIZ_URL" \
  SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
  SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}" \
  SLAN_STATE_DIR="$WORK_DIR/state" \
  "$SERVICE_BIN" >"$MAC_LOG" 2>&1 &
PIDS+=("$!")

echo "+ activate and enable Mac service network"
MAC_OUTPUT="$(
  cd "$ROOT_DIR"
  "$GO_BIN" run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -authorization-key "$MAC_AUTHORIZATION_KEY" \
    -enable-network=true \
    -timeout "$TIMEOUT"
)"
echo "$MAC_OUTPUT"
MAC_DEVICE_ID="$(echo "$MAC_OUTPUT" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
MAC_IP="$(echo "$MAC_OUTPUT" | sed -n 's/.*virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
if [[ -z "$MAC_DEVICE_ID" || -z "$MAC_IP" ]]; then
  echo "failed to parse Mac device info from activation output" >&2
  exit 1
fi
[[ "$MAC_DEVICE_ID" == "$MAC_TEST_DEVICE_ID" ]] || {
  echo "Mac device id mismatch: expected=$MAC_TEST_DEVICE_ID actual=$MAC_DEVICE_ID" >&2
  exit 1
}
echo "Mac network IP: $MAC_IP"

echo "+ start Mac UDP/TCP echo server"
(
  cd "$ROOT_DIR"
  "$GO_BIN" run scripts/socket_echo_server.go \
    -udp-port "$UDP_PORT" \
    -tcp-port "$TCP_PORT" \
    -listen-host "$MAC_IP"
) >"$ECHO_LOG" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 20); do
  if grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$ECHO_LOG" &&
     grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$ECHO_LOG"; then
    break
  fi
  sleep 1
done

if ! grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$ECHO_LOG"; then
  cat "$ECHO_LOG"
  echo "Mac UDP echo server did not start" >&2
  exit 1
fi
if ! grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$ECHO_LOG"; then
  cat "$ECHO_LOG"
  echo "Mac TCP echo server did not start" >&2
  exit 1
fi

echo "+ run iOS real-device UDP/TCP sender target=$MAC_IP udp=$UDP_PORT tcp=$TCP_PORT"
mapfile -t IOS_COMMON_DART_DEFINES < <(
  slan_mobile_activation_common_defines "$BIZ_URL" "$IOS_AUTHORIZATION_KEY" true
)
(
  cd "$APP_DIR"
  flutter test integration_test/device_activation_harness_test.dart \
    -d "$SLAN_IOS_FLUTTER_DEVICE" \
    "${IOS_COMMON_DART_DEFINES[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_HOLD_SECONDS=$IOS_PEER_HOLD_SECONDS" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$IOS_POST_ENABLE_WAIT_SECONDS" \
    --dart-define="SLAN_TEST_UDP_SEND_TARGET=$MAC_IP:$UDP_PORT" \
    --dart-define="SLAN_TEST_UDP_SEND_BODY=$UDP_BODY" \
    --dart-define="SLAN_TEST_TCP_SEND_TARGET=$MAC_IP:$TCP_PORT" \
    --dart-define="SLAN_TEST_TCP_SEND_BODY=$TCP_BODY"
) >"$IOS_LOG" 2>&1

cat "$IOS_LOG"

grep -q "SLAN_TEST_UDP_ECHO_OK=$MAC_IP:$UDP_PORT" "$IOS_LOG" || {
  echo "missing UDP success marker in iOS log" >&2
  exit 1
}
grep -q "SLAN_TEST_TCP_ECHO_OK=$MAC_IP:$TCP_PORT" "$IOS_LOG" || {
  echo "missing TCP success marker in iOS log" >&2
  exit 1
}
IOS_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$IOS_LOG" | tail -n 1)"
[[ "$IOS_DEVICE_ID" == "$IOS_TEST_DEVICE_ID" ]] || {
  echo "iOS device id mismatch: expected=$IOS_TEST_DEVICE_ID actual=$IOS_DEVICE_ID" >&2
  exit 1
}
grep -q "SOCKET_ECHO_UDP_RECEIVED=" "$ECHO_LOG" || {
  echo "missing UDP receive marker in Mac echo log" >&2
  exit 1
}
grep -q "SOCKET_ECHO_TCP_RECEIVED=" "$ECHO_LOG" || {
  echo "missing TCP receive marker in Mac echo log" >&2
  exit 1
}

echo "macIosRealDeviceSocketCheck: ok macIp=$MAC_IP udp=$UDP_PORT tcp=$TCP_PORT iosDevice=${SLAN_IOS_FLUTTER_DEVICE}"
