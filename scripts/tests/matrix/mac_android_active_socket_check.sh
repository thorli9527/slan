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
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
GO_BIN="${SLAN_GO_BIN:-/opt/homebrew/bin/go}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
if [[ -n "${SLAN_ANDROID_BIZ_URL:-}" ]]; then
  ANDROID_BIZ_URL="$SLAN_ANDROID_BIZ_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  ANDROID_BIZ_URL="http://10.0.2.2:28080"
else
  ANDROID_BIZ_URL="$BIZ_URL"
fi

MAC_SERVICE_MODE="${SLAN_MAC_SERVICE_MODE:-existing}"
MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST:-127.0.0.1:46392}"
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
TIMEOUT="${SLAN_MAC_ANDROID_ACTIVE_TIMEOUT:-120s}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
ANDROID_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-35}"
ANDROID_HOLD_SECONDS="${SLAN_ANDROID_ACTIVE_HOLD_SECONDS:-240}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
ANDROID_TEST_DEVICE_ID="${SLAN_ANDROID_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
SUDO_PASSWORD="${SLAN_SUDO_PASSWORD:-}"
RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
MAC_FORCE_RELAY_ONLY="${SLAN_MAC_FORCE_RELAY_ONLY:-1}"

WORK_DIR="${SLAN_MAC_ANDROID_ACTIVE_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-android-active.XXXXXX")}"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
ANDROID_LOG="$WORK_DIR/android-active.log"
ANDROID_LOGCAT="$WORK_DIR/android-logcat.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"

PIDS=()
OPS_TOKEN=""
NETWORK_ID=""
MAC_CREDENTIAL_ID=""
MAC_AUTHORIZATION_KEY=""
ANDROID_CREDENTIAL_ID=""
ANDROID_AUTHORIZATION_KEY=""
ADB_ARGS=("$ADB")
if [[ -n "$ANDROID_DEVICE" ]]; then
  ADB_ARGS+=(-s "$ANDROID_DEVICE")
fi

fail() {
  echo "$*" >&2
  exit 1
}

sha256_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

is_truthy() {
  local value="${1:-}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

sudo_run() {
  if [[ -n "$SUDO_PASSWORD" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

run_client_core_activation_check() {
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
    if [[ "$attempt" != "$attempts" ]]; then
      sleep $((attempt * 5))
    fi
  done
  echo "$output"
  return "$status"
}

create_device_authorization_keys() {
  local credential
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac Android Active Mac")"
  MAC_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  MAC_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac Android Active Android" "$ANDROID_TEST_DEVICE_ID")"
  ANDROID_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  ANDROID_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"$ANDROID_AUTHORIZATION_KEY\",\"deviceId\":\"$ANDROID_TEST_DEVICE_ID\"}" >/dev/null
}

create_ops_network() {
  local mac_device_id="$1"
  local network
  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "mac-android-active-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network" | jq -er '.networkId')"
  slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$mac_device_id"
  slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$ANDROID_TEST_DEVICE_ID"
}

verify_existing_macos_service() {
  local expected_bin="$1"
  local expected_hash installed_hash health_output
  [[ -x "$expected_bin" ]] || fail "expected mac client-core-service binary is missing: $expected_bin"
  [[ -x "/Library/Application Support/SLAN/client-core-service" ]] || fail "installed mac client-core-service is missing"
  expected_hash="$(sha256_file "$expected_bin")"
  installed_hash="$(sha256_file "/Library/Application Support/SLAN/client-core-service")"
  [[ "$expected_hash" == "$installed_hash" ]] || fail "installed mac client-core-service is stale; reinstall with scripts/install_macos_service.sh"
  health_output="$(
    run_client_core_activation_check "mac service health" \
      -address "$MAC_SERVICE_HOST" \
      -health-only=true \
      -timeout 10s
  )" || fail "installed mac client-core-service local API is unhealthy at $MAC_SERVICE_HOST"
  echo "$health_output"
}

reset_existing_macos_service_identity() {
  local expected_bin="$1"
  local -a install_cmd=(
    env
    SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST"
    SLAN_CONTROL_BASE_URL="$BIZ_URL"
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
    SLAN_FORCE_RELAY_ONLY="$MAC_FORCE_RELAY_ONLY"
    SLAN_RESET_MACOS_IDENTITY=1
    "$ROOT_DIR/scripts/install_macos_service.sh"
    --binary "$expected_bin"
  )
  if [[ "$MAC_SERVICE_MODE" != "existing" ]]; then
    return 0
  fi
  if ! is_truthy "$RESET_EXISTING_MAC_SERVICE_IDENTITY"; then
    return 0
  fi
  if [[ $EUID -eq 0 ]]; then
    "${install_cmd[@]}"
  else
    sudo_run "${install_cmd[@]}"
  fi
}

tap_bounds_center() {
  local bounds="$1"
  [[ "$bounds" =~ \[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\] ]] || return 1
  local x1="${BASH_REMATCH[1]}"
  local y1="${BASH_REMATCH[2]}"
  local x2="${BASH_REMATCH[3]}"
  local y2="${BASH_REMATCH[4]}"
  local x=$(( (x1 + x2) / 2 ))
  local y=$(( (y1 + y2) / 2 ))
  "${ADB_ARGS[@]}" shell input tap "$x" "$y" >/dev/null 2>&1 || true
}

start_android_vpn_appops_guard() {
  (
    while true; do
      "${ADB_ARGS[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
      sleep 0.25
    done
  ) &
  PIDS+=("$!")
}

start_android_vpn_consent_guard() {
  (
    while true; do
      xml="$("${ADB_ARGS[@]}" shell uiautomator dump /sdcard/slan-ui.xml >/dev/null 2>&1 && "${ADB_ARGS[@]}" shell cat /sdcard/slan-ui.xml 2>/dev/null || true)"
      if [[ "$xml" == *"package=\"com.android.vpndialogs\""* || "$xml" == *"VPN"* || "$xml" == *"连接请求"* || "$xml" == *"网络请求"* ]]; then
        bounds="$(
          printf '%s' "$xml" | tr '>' '\n' | grep -E \
            'resource-id="android:id/button1"|resource-id="com.android.vpndialogs:id/button1"|text="(确定|OK|继续|允许|始终允许|同意|接受|Connect|Allow)"' \
            | sed -n 's/.*bounds="\([^"]*\)".*/\1/p' | head -n 1
        )"
        if [[ -n "$bounds" ]]; then
          tap_bounds_center "$bounds"
        fi
      fi
      sleep 0.5
    done
  ) &
  PIDS+=("$!")
}

start_android_logcat_capture() {
  "${ADB_ARGS[@]}" logcat -c >/dev/null 2>&1 || true
  (
    exec "${ADB_ARGS[@]}" logcat
  ) >"$ANDROID_LOGCAT" 2>&1 &
  PIDS+=("$!")
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

current_macos_virtual_ip() {
  local status_json
  status_json="$(slan_macos_local_status_json "$MAC_SERVICE_HOST" 2>/dev/null || true)"
  printf '%s\n' "$status_json" | sed -n 's/.*"virtualIp":"\([^"]*\)".*/\1/p' | tail -n 1
}

flutter_retryable_startup_failure() {
  local log_file="$1"
  [[ -f "$log_file" ]] || return 1
  grep -Eq 'Failed to start Dart Development Service|DDS|observatory|vm service' "$log_file"
}

start_android_flutter_peer_echo() {
  local attempt status
  local android_common_dart_defines=()
  while IFS= read -r define; do
    android_common_dart_defines+=("$define")
  done < <(
    slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true
  )
  for attempt in 1 2; do
    : >"$ANDROID_LOG"
    (
      cd "$APP_DIR"
      flutter test integration_test/device_activation_harness_test.dart \
        -d "$ANDROID_DEVICE" \
        --timeout "${SLAN_ANDROID_FLUTTER_TEST_TIMEOUT:-12m}" \
        "${android_common_dart_defines[@]}" \
        --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
        --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
        --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
        --dart-define="SLAN_TEST_HOLD_SECONDS=$ANDROID_HOLD_SECONDS" \
        --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
        --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT"
    ) >"$ANDROID_LOG" 2>&1 &
    ANDROID_PID="$!"
    PIDS+=("$ANDROID_PID")
    sleep 3
    if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
      wait "$ANDROID_PID" || status=$?
      if [[ $attempt -lt 2 ]] && flutter_retryable_startup_failure "$ANDROID_LOG"; then
        echo "retry Android flutter peer echo after startup failure (attempt $attempt)" >&2
        sleep 3
        continue
      fi
      return 1
    fi
    return 0
  done
  return 1
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android active log ----" >&2; cat "$ANDROID_LOG" >&2; }
    [[ -f "$ANDROID_LOGCAT" ]] && {
      echo "---- Android logcat key lines ----" >&2
      grep -E 'SLAN_ANDROID_FFI_|SLAN_TEST_(UDP|TCP)_ECHO|client-core-service|SlanVpnService' "$ANDROID_LOGCAT" >&2 || true
    }
    if [[ -f "$ANDROID_LOG" ]]; then
      echo "---- Android runtime stats ----" >&2
      sed -n 's/.*SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=//p' "$ANDROID_LOG" | tail -n 1 >&2 || true
      sed -n 's/.*SLAN_TEST_ANDROID_PACKET_TUNNEL_READY=//p' "$ANDROID_LOG" | tail -n 1 >&2 || true
    fi
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$MAC_CREDENTIAL_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$ANDROID_CREDENTIAL_ID" >/dev/null 2>&1 || true
  if [[ "${SLAN_KEEP_MAC_ANDROID_ACTIVE_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

[[ -x "$ADB" ]] || fail "adb is missing or not executable: $ADB"
[[ -x "$GO_BIN" ]] || fail "go binary is missing or not executable: $GO_BIN"
[[ -x "$SERVICE_BIN" ]] || fail "client-core-service binary is missing: $SERVICE_BIN"

echo "+ ${ADB_ARGS[*]} wait-for-device"
"${ADB_ARGS[@]}" wait-for-device
"${ADB_ARGS[@]}" shell pm clear dev.slan.slan_client_v2 >/dev/null 2>&1 || true
"${ADB_ARGS[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
start_android_logcat_capture
start_android_vpn_appops_guard
start_android_vpn_consent_guard

mkdir -p "$WORK_DIR/state"
create_device_authorization_keys

if [[ "$MAC_SERVICE_MODE" == "existing" ]]; then
  reset_existing_macos_service_identity "$SERVICE_BIN"
  verify_existing_macos_service "$SERVICE_BIN"
else
  fail "only SLAN_MAC_SERVICE_MODE=existing is supported for mac Android active socket check"
fi

echo "+ activate Mac service at $MAC_SERVICE_HOST"
MAC_OUTPUT="$(
  run_client_core_activation_check "mac active activation" \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -authorization-key "$MAC_AUTHORIZATION_KEY" \
    -enable-network=false \
    -timeout "$TIMEOUT"
)"
echo "$MAC_OUTPUT"
MAC_DEVICE_ID="$(echo "$MAC_OUTPUT" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
[[ -n "$MAC_DEVICE_ID" ]] || fail "failed to parse Mac device id"
create_ops_network "$MAC_DEVICE_ID"

echo "+ enable Mac service network after Ops assignment"
MAC_OUTPUT="$(
  run_client_core_activation_check "mac active network enable" \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -activate=false \
    -enable-network=true \
    -timeout "$TIMEOUT"
)"
echo "$MAC_OUTPUT"
MAC_IP="$(echo "$MAC_OUTPUT" | sed -n 's/.*clientCoreServiceNetwork: enabled virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
[[ -n "$MAC_IP" ]] || fail "failed to parse Mac virtual IP"
MAC_IP="${MAC_IP%%/*}"
echo "Mac network IP: $MAC_IP"

echo "+ run Android peer echo server target hold=${ANDROID_HOLD_SECONDS}s"
start_android_flutter_peer_echo || {
  cat "$ANDROID_LOG"
  fail "failed to start Android peer echo flutter test"
}

ANDROID_IP=""
for _ in $(seq 1 180); do
  if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
    cat "$ANDROID_LOG"
    fail "Android active socket test exited before reporting network IP"
  fi
  ANDROID_IP="$(sed -n 's/.*SLAN_TEST_NETWORK_IP=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
  if [[ -n "$ANDROID_IP" ]]; then
    break
  fi
  sleep 1
done
[[ -n "$ANDROID_IP" ]] || { cat "$ANDROID_LOG"; fail "timed out waiting for Android network IP"; }
ANDROID_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
[[ "$ANDROID_DEVICE_ID" == "$ANDROID_TEST_DEVICE_ID" ]] \
  || { cat "$ANDROID_LOG"; fail "Android device id mismatch: expected=$ANDROID_TEST_DEVICE_ID actual=$ANDROID_DEVICE_ID"; }
echo "Android network IP: $ANDROID_IP"

for _ in $(seq 1 60); do
  if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
    cat "$ANDROID_LOG"
    fail "Android active socket test exited before echo server became ready"
  fi
  if grep -q "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" "$ANDROID_LOG" &&
      grep -q "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" "$ANDROID_LOG"; then
    break
  fi
  sleep 1
done
grep -q "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" "$ANDROID_LOG" \
  || { cat "$ANDROID_LOG"; fail "timed out waiting for Android UDP echo server readiness"; }
grep -q "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" "$ANDROID_LOG" \
  || { cat "$ANDROID_LOG"; fail "timed out waiting for Android TCP echo server readiness"; }
echo "Android echo servers are ready: udp=$UDP_PORT tcp=$TCP_PORT"

CURRENT_MAC_IP="$(current_macos_virtual_ip)"
[[ -n "$CURRENT_MAC_IP" ]] || fail "failed to resolve current macOS virtual IP from local status"
if [[ "$CURRENT_MAC_IP" != "$MAC_IP" ]]; then
  echo "Mac network IP refreshed from $MAC_IP to $CURRENT_MAC_IP"
  MAC_IP="$CURRENT_MAC_IP"
fi

slan_wait_macos_peer_route_ready "$ANDROID_IP" "$MAC_IP" "$MAC_SERVICE_HOST" 120 \
  || fail "mac peer route did not become stable for Android target=${ANDROID_IP} source=${MAC_IP}"

if [[ "${SLAN_SKIP_MAC_UDP_SEND:-0}" != "1" ]]; then
  UDP_RESULT="$(send_mac_udp "$MAC_IP" "$ANDROID_IP" "mac-to-android-udp-$(date +%s%N)")"
  [[ "$UDP_RESULT" == echo:* ]] || fail "Mac -> Android UDP failed: $UDP_RESULT"
fi
TCP_BODY="mac-to-android-tcp-$(date +%s%N)"
if [[ "${SLAN_SKIP_MAC_TCP_SEND:-0}" != "1" ]]; then
  TCP_RESULT="$(send_mac_tcp "$MAC_IP" "$ANDROID_IP" "$TCP_BODY")"
  [[ "$TCP_RESULT" == "echo:${TCP_BODY}" ]] || fail "Mac -> Android TCP failed: $TCP_RESULT"
fi

kill "$ANDROID_PID" 2>/dev/null || true
wait "$ANDROID_PID" || true
cat "$ANDROID_LOG"

echo "macAndroidActiveSocketCheck: ok macIp=$MAC_IP androidIp=$ANDROID_IP udp=$UDP_PORT tcp=$TCP_PORT"
