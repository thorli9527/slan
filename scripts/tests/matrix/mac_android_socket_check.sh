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
MACOS_APP_PATH="${SLAN_MACOS_APP_PATH:-$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
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
if [[ -n "${SLAN_USE_EXISTING_MAC_SERVICE:-}" ]] && [[ "${SLAN_USE_EXISTING_MAC_SERVICE}" == "1" ]]; then
  MAC_SERVICE_MODE="existing"
fi
if [[ -n "${SLAN_MAC_SERVICE_HOST:-}" ]]; then
  MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST}"
elif [[ "$MAC_SERVICE_MODE" == "app" ]]; then
  MAC_SERVICE_HOST="127.0.0.1:46394"
else
  MAC_SERVICE_HOST="127.0.0.1:46392"
fi
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
ADB_ARGS=("$ADB")
if [[ -n "$ANDROID_DEVICE" ]]; then
  ADB_ARGS+=(-s "$ANDROID_DEVICE")
fi
TIMEOUT="${SLAN_MAC_ANDROID_SOCKET_TIMEOUT:-90s}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr -d '-' | tr '[:upper:]' '[:lower:]')}"
RUN_MAC_LOCAL_DNS_SMOKE="${SLAN_RUN_MAC_LOCAL_DNS_SMOKE:-1}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-android-to-mac-udp-$(date +%s%N)}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-android-to-mac-tcp-$(date +%s%N)}"
SOCKET_TARGET_HOST="${SLAN_TEST_SOCKET_TARGET_HOST:-}"
AUTO_PROVISION_SOCKET_DNS="${SLAN_AUTO_PROVISION_SOCKET_DNS:-1}"
# Give the peer side time to finish relay attach before Android starts
# emitting socket traffic. In practice the macOS service may need one
# relay reconfigure cycle after Android enables the tunnel.
ANDROID_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-35}"
WORK_DIR="${SLAN_MAC_ANDROID_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-android-socket.XXXXXX")}"
ECHO_LOG="$WORK_DIR/macos-echo.log"
ANDROID_LOG="$WORK_DIR/android-socket.log"
ANDROID_ECHO_LOG="$WORK_DIR/android-echo.log"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"
INSTALLED_MAC_SERVICE_BIN="/Library/Application Support/SLAN/client-core-service"
RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
SUDO_PASSWORD="${SLAN_SUDO_PASSWORD:-}"
ANDROID_TEST_TIMEOUT_SECONDS="${SLAN_ANDROID_TEST_TIMEOUT_SECONDS:-180}"
ANDROID_ECHO_HOLD_SECONDS="${SLAN_ANDROID_ECHO_HOLD_SECONDS:-120}"

NETWORK_ID=""
SECURITY_GROUP_ID=""
DEVICE_GROUP_ID=""
ANDROID_TEST_DEVICE_ID="${SLAN_ANDROID_TEST_DEVICE_ID:-$(uuidgen | tr -d '-' | tr '[:upper:]' '[:lower:]')}"
ZONE_ID=""
ZONE_NAME=""
RECORD_ID=""
PROVISIONED_SOCKET_TARGET_HOST=""
RULE_IDS=()
OPS_TOKEN=""
MAC_CREDENTIAL_ID=""
MAC_AUTHORIZATION_KEY=""
ANDROID_CREDENTIAL_ID=""
ANDROID_AUTHORIZATION_KEY=""

PIDS=()

fail() {
  echo "$*" >&2
  exit 1
}

sha256_file() {
  local path="$1"
  shasum -a 256 "$path" | awk '{print $1}'
}

is_truthy() {
  local value="${1:-}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

extract_json_field() {
  local json="$1"
  local field="$2"
  printf '%s' "$json" | sed -n "s/.*\"${field}\":\"\\([^\"]*\\)\".*/\\1/p" | head -n 1
}

extract_network_id_from_auth() {
  local json="$1"
  local value
  value="$(extract_json_field "$json" "activeNetworkId")"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return 0
  fi
  value="$(extract_json_field "$json" "networkId")"
  printf '%s\n' "$value"
}

best_effort_delete() {
  local url="$1"
  curl --silent --show-error --connect-timeout 5 --max-time 20 \
    -X DELETE "$url" \
    -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null 2>&1 || true
}

create_json() {
  local url="$1"
  local payload="$2"
  local response
  if ! response="$(curl --silent --show-error --fail-with-body --connect-timeout 5 --max-time 30 \
    -X POST "$url" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$payload")"; then
    printf 'POST %s failed: %s\n' "$url" "$response" >&2
    return 1
  fi
  printf '%s' "$response"
}

sudo_run() {
  if [[ -n "$SUDO_PASSWORD" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

echo "+ mac android socket defaults: biz=$BIZ_URL ops=$OPS_BASE_URL service_host=$MAC_SERVICE_HOST android_device=$ANDROID_DEVICE force_relay_only=${SLAN_FORCE_RELAY_ONLY:-0}"

android_runtime_json() {
  sed -n 's/.*SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=//p' "$ANDROID_LOG" | tail -n 1
}

json_number_value() {
  local json="$1"
  local key="$2"
  printf '%s' "$json" | sed -n "s/.*\"${key}\":\([0-9][0-9]*\).*/\1/p"
}

assert_android_stat_min() {
  local key="$1"
  local min_value="$2"
  local json value
  json="$(android_runtime_json)"
  [[ -n "$json" ]] || fail "missing SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD in Android log"
  value="$(json_number_value "$json" "$key")"
  [[ -n "$value" ]] || fail "missing Android runtime stat: $key"
  if (( value < min_value )); then
    fail "Android runtime stat $key=$value is below expected minimum $min_value"
  fi
}

assert_android_stat_max() {
  local key="$1"
  local max_value="$2"
  local json value
  json="$(android_runtime_json)"
  [[ -n "$json" ]] || fail "missing SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD in Android log"
  value="$(json_number_value "$json" "$key")"
  [[ -n "$value" ]] || fail "missing Android runtime stat: $key"
  if (( value > max_value )); then
    fail "Android runtime stat $key=$value is above expected maximum $max_value"
  fi
}

assert_android_runtime_contains() {
  local expected="$1"
  local json
  json="$(android_runtime_json)"
  [[ -n "$json" ]] || fail "missing SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD in Android log"
  printf '%s' "$json" | grep -Eq "$expected" \
    || fail "Android runtime stats do not contain expected pattern: $expected"
}

assert_android_config_contains() {
  local expected="$1"
  grep -q "SLAN_ANDROID_NETWORK_CONFIG .*${expected}" "$ANDROID_LOG" \
    || fail "Android network config does not contain expected pattern: $expected"
}

android_success_markers_present() {
  local target_host="$1"
  if [[ "${SLAN_SKIP_ANDROID_UDP_SEND:-0}" != "1" ]] && \
    ! grep -q "SLAN_TEST_UDP_ECHO_OK=$target_host:$UDP_PORT" "$ANDROID_LOG"; then
    return 1
  fi
  if [[ "${SLAN_SKIP_ANDROID_TCP_SEND:-0}" != "1" ]] && \
    ! grep -q "SLAN_TEST_TCP_ECHO_OK=$target_host:$TCP_PORT" "$ANDROID_LOG"; then
    return 1
  fi
  return 0
}

android_runtime_stats_present() {
  grep -q "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" "$ANDROID_LOG"
}

run_android_socket_test() {
  local target_host="$1"
  local -a ANDROID_COMMON_DART_DEFINES=()
  while IFS= read -r define; do
    ANDROID_COMMON_DART_DEFINES+=("$define")
  done < <(
    slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true
  )
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$ANDROID_DEVICE" \
      "${ANDROID_COMMON_DART_DEFINES[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_FORCE_RELAY_ONLY=${SLAN_FORCE_RELAY_ONLY:-0}" \
      --dart-define="SLAN_TEST_WAIT_MQTT=true" \
      --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=4" \
      --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=${SLAN_ANDROID_TEST_HOLD_SECONDS:-0}" \
      --dart-define="SLAN_TEST_UDP_SEND_TARGET=$ANDROID_UDP_TARGET" \
      --dart-define="SLAN_TEST_UDP_SEND_BODY=$UDP_BODY" \
      --dart-define="SLAN_TEST_TCP_SEND_TARGET=$ANDROID_TCP_TARGET" \
      --dart-define="SLAN_TEST_TCP_SEND_BODY=$TCP_BODY" \
      --dart-define="SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST=${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}"
  ) >"$ANDROID_LOG" 2>&1 &
  local android_pid=$!
  PIDS+=("$android_pid")

  local start_ts now elapsed
  start_ts=$(date +%s)
  while kill -0 "$android_pid" 2>/dev/null; do
    if android_success_markers_present "$target_host"; then
      local settle_start now_settle settle_elapsed
      settle_start=$(date +%s)
      while kill -0 "$android_pid" 2>/dev/null; do
        if android_runtime_stats_present; then
          break
        fi
        now_settle=$(date +%s)
        settle_elapsed=$((now_settle - settle_start))
        if (( settle_elapsed >= 20 )); then
          break
        fi
        sleep 1
      done
      if kill -0 "$android_pid" 2>/dev/null; then
        kill -INT "$android_pid" 2>/dev/null || true
        sleep 2
      fi
      if kill -0 "$android_pid" 2>/dev/null; then
        kill -TERM "$android_pid" 2>/dev/null || true
        sleep 2
      fi
      if kill -0 "$android_pid" 2>/dev/null; then
        kill -KILL "$android_pid" 2>/dev/null || true
      fi
      wait "$android_pid" 2>/dev/null || true
      return 0
    fi
    now=$(date +%s)
    elapsed=$((now - start_ts))
    if (( elapsed >= ANDROID_TEST_TIMEOUT_SECONDS )); then
      kill -TERM "$android_pid" 2>/dev/null || true
      sleep 2
      kill -KILL "$android_pid" 2>/dev/null || true
      wait "$android_pid" 2>/dev/null || true
      echo "Android socket integration test timed out after ${ANDROID_TEST_TIMEOUT_SECONDS}s" >&2
      return 1
    fi
    sleep 2
  done

  wait "$android_pid"
}

start_android_echo_test() {
  local android_common_dart_defines=()
  while IFS= read -r define; do
    android_common_dart_defines+=("$define")
  done < <(
    slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true
  )
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$ANDROID_DEVICE" \
      --timeout 12m \
      "${android_common_dart_defines[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_FORCE_RELAY_ONLY=${SLAN_FORCE_RELAY_ONLY:-0}" \
      --dart-define="SLAN_TEST_WAIT_MQTT=true" \
      --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=4" \
      --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
      --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
      --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=$ANDROID_ECHO_HOLD_SECONDS" \
      --dart-define="SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST=${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}"
  ) >"$ANDROID_ECHO_LOG" 2>&1 &
  ANDROID_ECHO_PID="$!"
  PIDS+=("$ANDROID_ECHO_PID")
}

wait_android_echo_ready() {
  local deadline=$((SECONDS + ANDROID_TEST_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    if ! kill -0 "$ANDROID_ECHO_PID" 2>/dev/null; then
      cat "$ANDROID_ECHO_LOG"
      fail "Android reverse echo test exited before readiness"
    fi
    if grep -q "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" "$ANDROID_ECHO_LOG" && \
       grep -q "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" "$ANDROID_ECHO_LOG"; then
      return 0
    fi
    sleep 1
  done
  cat "$ANDROID_ECHO_LOG"
  fail "timed out waiting for Android reverse echo readiness"
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

send_mac_udp_echo() {
  local target_ip="$1"
  local body="$2"
  python3 - "$MAC_IP" "$target_ip" "$UDP_PORT" "$body" <<'PY'
import socket
import sys
import time

source_ip, target_ip, port_value, body_value = sys.argv[1:]
expected = "echo:" + body_value
last = ""
for _ in range(6):
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.settimeout(3)
    try:
        sock.bind((source_ip, 0))
        sock.sendto(body_value.encode(), (target_ip, int(port_value)))
        last = sock.recvfrom(2048)[0].decode()
        if last == expected:
            print(last)
            raise SystemExit(0)
    except OSError as exc:
        last = repr(exc)
    finally:
        sock.close()
    time.sleep(1)
print(last, file=sys.stderr)
raise SystemExit(1)
PY
}

send_mac_tcp_echo() {
  local target_ip="$1"
  local body="$2"
  python3 - "$MAC_IP" "$target_ip" "$TCP_PORT" "$body" <<'PY'
import socket
import sys
import time

source_ip, target_ip, port_value, body_value = sys.argv[1:]
expected = "echo:" + body_value
last = ""
for _ in range(6):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(3)
    try:
        sock.bind((source_ip, 0))
        sock.connect((target_ip, int(port_value)))
        sock.sendall((body_value + "\n").encode())
        last = sock.recv(2048).decode().strip()
        if last == expected:
            print(last)
            raise SystemExit(0)
    except OSError as exc:
        last = repr(exc)
    finally:
        sock.close()
    time.sleep(1)
print(last, file=sys.stderr)
raise SystemExit(1)
PY
}

set_macos_relay_transport_allowlist() {
  local allowlist="${1:-}"
  [[ -n "$allowlist" ]] || return 0
  python3 - "$MAC_SERVICE_HOST" "$allowlist" <<'PY'
import json
import socket
import sys

host, port = sys.argv[1].rsplit(":", 1)
payload = json.dumps({
    "method": "localSetRelayTransportAllowlist",
    "args": {"transports": [item.strip() for item in sys.argv[2].split(",") if item.strip()]},
}).encode() + b"\n"
s = socket.create_connection((host, int(port)), timeout=5)
try:
    s.sendall(payload)
    s.shutdown(socket.SHUT_WR)
    print(s.recv(65535).decode())
finally:
    s.close()
PY
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

verify_existing_macos_service() {
  local expected_bin="$1"
  local expected_hash installed_hash health_output

  if [[ ! -x "$expected_bin" ]]; then
    local bundled_service="${MACOS_APP_PATH%/}/Contents/MacOS/client-core-service"
    if [[ -x "$bundled_service" ]]; then
      expected_bin="$bundled_service"
    fi
  fi

  [[ -x "$expected_bin" ]] || fail "expected mac client-core-service binary is missing: $expected_bin"
  sudo_run test -x "$INSTALLED_MAC_SERVICE_BIN" \
    || fail "installed mac client-core-service is missing: $INSTALLED_MAC_SERVICE_BIN"

  expected_hash="$(sha256_file "$expected_bin")"
  installed_hash="$(sudo_run shasum -a 256 "$INSTALLED_MAC_SERVICE_BIN" | awk '{print $1}')"
  echo "macServiceExpectedSha256: $expected_hash"
  echo "macServiceInstalledSha256: $installed_hash"
  if [[ "$expected_hash" != "$installed_hash" ]]; then
    fail "installed mac client-core-service is stale; reinstall with: sudo scripts/install_macos_service.sh --binary $expected_bin"
  fi

  if ! health_output="$(
    run_client_core_activation_check "mac service health" \
      -address "$MAC_SERVICE_HOST" \
      -health-only=true \
      -timeout 10s
  )"; then
    echo "$health_output" >&2
    fail "installed mac client-core-service local API is unhealthy at $MAC_SERVICE_HOST; reinstall with: sudo scripts/install_macos_service.sh --binary $expected_bin"
  fi
  echo "$health_output"
}

ensure_macos_app_service() {
  [[ -d "$MACOS_APP_PATH" ]] || fail "macOS app bundle is missing: $MACOS_APP_PATH"
  local app_bin="${MACOS_APP_PATH%/}/Contents/MacOS/slan_client_v2"
  local service_bin="${MACOS_APP_PATH%/}/Contents/MacOS/client-core-service"
  [[ -x "$app_bin" ]] || fail "macOS app binary is missing: $app_bin"
  [[ -x "$service_bin" ]] || fail "bundled macOS client-core-service is missing: $service_bin"

  echo "+ start macOS app-hosted client service via $MACOS_APP_PATH"
  open "$MACOS_APP_PATH"

  local health_output
  if ! health_output="$(
    run_client_core_activation_check "mac app service health" \
      -address "$MAC_SERVICE_HOST" \
      -health-only=true \
      -timeout 15s
  )"; then
    echo "$health_output" >&2
    fail "macOS app-hosted client-core-service local API is unhealthy at $MAC_SERVICE_HOST"
  fi
  echo "$health_output"
}

reset_existing_macos_service_identity() {
  local expected_bin="$1"
  local -a install_cmd=(
    env
    SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST"
    SLAN_CONTROL_BASE_URL="$BIZ_URL"
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
    SLAN_FORCE_RELAY_ONLY="${SLAN_FORCE_RELAY_ONLY:-0}"
    SLAN_RESET_MACOS_IDENTITY=1
    SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}"
    "$ROOT_DIR/scripts/install_macos_service.sh"
    --binary "$expected_bin"
  )
  if [[ "$MAC_SERVICE_MODE" != "existing" ]]; then
    return 0
  fi
  if ! is_truthy "$RESET_EXISTING_MAC_SERVICE_IDENTITY"; then
    return 0
  fi
  echo "+ reset existing mac client identity"
  if [[ $EUID -eq 0 ]]; then
    "${install_cmd[@]}"
    return 0
  fi
  if [[ -n "$SUDO_PASSWORD" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "${install_cmd[@]}"
    return 0
  fi
  sudo "${install_cmd[@]}"
}

ensure_network_context() {
  [[ -n "$NETWORK_ID" && -n "$SECURITY_GROUP_ID" ]] && return 0

  local network groups
  echo "+ create test network" >&2
  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "mac-android-socket-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network" | jq -r '.networkId // empty')"
  [[ -n "$NETWORK_ID" ]] || fail "Ops network create returned empty networkId"
  echo "+ create test security group" >&2
  groups="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/security-groups" \
    "{\"name\":\"mac-android-socket-security-$(date +%s%N)\"}")"
  SECURITY_GROUP_ID="$(printf '%s' "$groups" | jq -r '.securityGroupId // empty')"
  [[ -n "$SECURITY_GROUP_ID" ]] || fail "network ${NETWORK_ID} has no security group"
}

create_mac_authorization_key() {
  local credential_json android_json
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac Android Socket Check")"
  MAC_CREDENTIAL_ID="$(printf '%s' "$credential_json" | jq -r '.credentialId // empty')"
  MAC_AUTHORIZATION_KEY="$(printf '%s' "$credential_json" | jq -r '.key // empty')"
  [[ -n "$MAC_CREDENTIAL_ID" && -n "$MAC_AUTHORIZATION_KEY" ]] || fail "failed to create Mac authorization key"
  android_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac Android Socket Android")"
  ANDROID_CREDENTIAL_ID="$(printf '%s' "$android_json" | jq -r '.credentialId // empty')"
  ANDROID_AUTHORIZATION_KEY="$(printf '%s' "$android_json" | jq -r '.key // empty')"
  [[ -n "$ANDROID_CREDENTIAL_ID" && -n "$ANDROID_AUTHORIZATION_KEY" ]] || fail "failed to create Android authorization key"
}

provision_network_membership_and_acl() {
  local mac_device_id="$1"
  local android_json
  echo "+ bootstrap Android device" >&2
  curl --silent --show-error --fail-with-body --connect-timeout 5 --max-time 30 \
    -X POST "${BIZ_URL}/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"${ANDROID_AUTHORIZATION_KEY}\",\"deviceId\":\"${ANDROID_TEST_DEVICE_ID}\"}" >/dev/null
  slan_ops_revoke_device_credential \
    "$OPS_BASE_URL" "$OPS_TOKEN" "$ANDROID_CREDENTIAL_ID" >/dev/null
  android_json="$(slan_ops_create_device_credential \
    "$OPS_BASE_URL" "$OPS_TOKEN" "Mac Android Socket Android Client" "$ANDROID_TEST_DEVICE_ID")"
  ANDROID_CREDENTIAL_ID="$(printf '%s' "$android_json" | jq -r '.credentialId // empty')"
  ANDROID_AUTHORIZATION_KEY="$(printf '%s' "$android_json" | jq -r '.key // empty')"
  [[ -n "$ANDROID_CREDENTIAL_ID" && -n "$ANDROID_AUTHORIZATION_KEY" ]] \
    || fail "failed to create Android client authorization key"

  local group_json device_id rule_json rule_id
  echo "+ create device group" >&2
  group_json="$(create_json "${OPS_BASE_URL}/api/ops/device-groups" \
    "{\"name\":\"mac-android-$(date +%s%N)\",\"description\":\"Mac Android integration devices\"}")"
  DEVICE_GROUP_ID="$(printf '%s' "$group_json" | jq -r '.groupId // empty')"
  [[ -n "$DEVICE_GROUP_ID" ]] || fail "device group create returned empty groupId"

  for device_id in "$mac_device_id" "$ANDROID_TEST_DEVICE_ID"; do
    echo "+ add device $device_id to device group $DEVICE_GROUP_ID" >&2
    curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
      -X POST "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}/devices" \
      -H "Authorization: Bearer ${OPS_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"deviceId\":\"${device_id}\"}" >/dev/null
  done

  echo "+ attach device group $DEVICE_GROUP_ID to network $NETWORK_ID" >&2
  create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups" \
    "{\"groupId\":\"${DEVICE_GROUP_ID}\"}" >/dev/null

  local direction protocol port priority
  priority=100
  for protocol in udp tcp; do
    port="$UDP_PORT"
    [[ "$protocol" == tcp ]] && port="$TCP_PORT"
    for direction in ingress egress; do
      echo "+ create $protocol $direction ACL rule" >&2
      rule_json="$(create_json "${OPS_BASE_URL}/api/ops/security-groups/${SECURITY_GROUP_ID}/rules" \
        "{\"direction\":\"${direction}\",\"priority\":${priority},\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"device_group\",\"peerValue\":\"${DEVICE_GROUP_ID}\",\"enabled\":true}")"
      rule_id="$(printf '%s' "$rule_json" | jq -r '.ruleId // empty')"
      [[ -n "$rule_id" ]] || fail "failed to create ${protocol} ${direction} ACL rule"
      RULE_IDS+=("$rule_id")
      priority=$((priority + 10))
    done
  done
}

provision_socket_dns_record() {
  local mac_device_id="$1"
  is_truthy "$AUTO_PROVISION_SOCKET_DNS" || return 0
  ensure_network_context

  ZONE_NAME="socket-${RANDOM}-$(date +%s).lan"
  local response
  response="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/dns/zones" \
    "{\"name\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(extract_json_field "$response" "zoneId")"
  [[ -n "$ZONE_ID" ]] || fail "failed to create dns zone ${ZONE_NAME}"

  response="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/dns/records" \
    "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"mac\",\"type\":\"A\",\"value\":\"${mac_device_id}\",\"port\":\"\",\"ttl\":60}")"
  RECORD_ID="$(extract_json_field "$response" "recordId")"
  [[ -n "$RECORD_ID" ]] || fail "failed to create socket dns record"

  PROVISIONED_SOCKET_TARGET_HOST="mac.${ZONE_NAME}"
  SOCKET_TARGET_HOST="$PROVISIONED_SOCKET_TARGET_HOST"
  echo "macAndroidSocketCheck: provisioned dnsZone=${ZONE_NAME} recordHost=${PROVISIONED_SOCKET_TARGET_HOST} network=${NETWORK_ID} device=${mac_device_id}"
}

wait_mac_network_module() {
  local peer_device_id="$1"
  local timeout_seconds="${2:-120}"
  python3 - "$MAC_SERVICE_HOST" "$peer_device_id" "$timeout_seconds" <<'PY'
import json
import socket
import sys
import time

address, peer_device_id, timeout_value = sys.argv[1:]
host, port = address.rsplit(":", 1)
deadline = time.time() + int(timeout_value)
last = {}
while time.time() < deadline:
    try:
        sock = socket.create_connection((host, int(port)), timeout=5)
        sock.settimeout(5)
        sock.sendall(b'{"method":"localNetworkModule","args":{}}\n')
        sock.shutdown(socket.SHUT_WR)
        chunks = []
        while True:
            chunk = sock.recv(65535)
            if not chunk:
                break
            chunks.append(chunk)
        sock.close()
        last = json.loads(b"".join(chunks).decode() or "{}")
        configs = last.get("configs") or []
        peers = [peer for config in configs for peer in (config.get("peers") or [])]
        records = [record for config in configs for record in (config.get("resolverRecords") or [])]
        rules = [rule for config in configs for rule in (config.get("securityRules") or [])]
        record_count = int(last.get("dnsRecordCount") or len(records))
        rule_count = int(last.get("securityRuleCount") or len(rules))
        if any(str(peer.get("deviceId") or "") == peer_device_id for peer in peers) and record_count >= 1 and rule_count >= 4:
            print(json.dumps(last, separators=(",", ":")))
            raise SystemExit(0)
    except (OSError, ValueError):
        pass
    time.sleep(1)
print(json.dumps(last, separators=(",", ":")), file=sys.stderr)
raise SystemExit(1)
PY
}

reload_mac_network_snapshot() {
  if [[ "$MAC_SERVICE_MODE" == "existing" ]]; then
    echo "+ restart installed Mac service and reload complete network snapshot"
    sudo_run launchctl kickstart -k system/dev.slan.client-core-service
  else
    echo "+ reload complete network snapshot through local Mac service"
  fi
  local output
  output="$(
    run_client_core_activation_check "mac snapshot reload" \
      -biz-url "$BIZ_URL" \
      -address "$MAC_SERVICE_HOST" \
      -activate=false \
      -enable-network=true \
      -timeout "$TIMEOUT"
  )" || {
    echo "$output" >&2
    fail "Mac service failed to reload network snapshot"
  }
  echo "$output"
  local reloaded_ip
  reloaded_ip="$(echo "$output" | sed -n 's/.*clientCoreServiceNetwork: enabled virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
  reloaded_ip="${reloaded_ip%%/*}"
  [[ -n "$reloaded_ip" ]] && MAC_IP="$reloaded_ip"
  wait_mac_network_module "$ANDROID_TEST_DEVICE_ID" 120 >/dev/null \
    || fail "Mac network module did not receive Android peer, DNS, and ACL snapshot"
  sleep "${SLAN_MAC_ENDPOINT_REPORT_SETTLE_SECONDS:-10}"
}

mac_virtual_ip_is_bound() {
  ifconfig | grep -Eq "inet[[:space:]]+${MAC_IP//./\\.}([[:space:]]|$)"
}

wait_mac_virtual_ip_bound() {
  local timeout_seconds="${1:-30}"
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    mac_virtual_ip_is_bound && return 0
    sleep 1
  done
  return 1
}

ensure_mac_virtual_ip_bound() {
  wait_mac_virtual_ip_bound 30 && return 0
  echo "+ Mac virtual IP is not bound yet; retry network activation"
  run_client_core_activation_check "mac interface activation retry" \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -activate=false \
    -enable-network=true \
    -timeout "$TIMEOUT"
  wait_mac_virtual_ip_bound 30 \
    || fail "Mac virtual IP $MAC_IP was not bound to a system interface"
}

cleanup() {
  status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$ECHO_LOG" ]] && { echo "---- Mac echo log ----" >&2; cat "$ECHO_LOG" >&2; }
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android socket log ----" >&2; cat "$ANDROID_LOG" >&2; }
    [[ -f "$ANDROID_ECHO_LOG" ]] && { echo "---- Android reverse echo log ----" >&2; cat "$ANDROID_ECHO_LOG" >&2; }
  fi
  terminate_tree() {
    local root_pid="$1"
    local child_pid
    while IFS= read -r child_pid; do
      [[ -n "$child_pid" ]] || continue
      terminate_tree "$child_pid"
    done < <(pgrep -P "$root_pid" 2>/dev/null || true)
    kill "$root_pid" 2>/dev/null || true
  }
  for pid in "${PIDS[@]:-}"; do
    terminate_tree "$pid"
  done
  if [[ -n "$MAC_CREDENTIAL_ID" && -n "$OPS_TOKEN" ]]; then
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$MAC_CREDENTIAL_ID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$ANDROID_CREDENTIAL_ID" && -n "$OPS_TOKEN" ]]; then
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$ANDROID_CREDENTIAL_ID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$NETWORK_ID" ]]; then
    local rule_id
    for rule_id in "${RULE_IDS[@]:-}"; do
      [[ -n "$rule_id" ]] && best_effort_delete "${OPS_BASE_URL}/api/ops/security-rules/${rule_id}"
    done
    [[ -n "$RECORD_ID" ]] && best_effort_delete "${OPS_BASE_URL}/api/ops/dns/records/${RECORD_ID}"
    [[ -n "$ZONE_ID" ]] && best_effort_delete "${OPS_BASE_URL}/api/ops/dns/zones/${ZONE_ID}"
    [[ -n "$DEVICE_GROUP_ID" ]] && best_effort_delete "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups/${DEVICE_GROUP_ID}"
    slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$DEVICE_GROUP_ID" ]]; then
    best_effort_delete "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}"
  fi
  if [[ "${SLAN_KEEP_MAC_ANDROID_SOCKET_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [[ ! -x "$ADB" ]]; then
  echo "adb is missing or not executable: $ADB" >&2
  exit 1
fi
if [[ ! -x "$GO_BIN" ]]; then
  echo "go binary is missing or not executable: $GO_BIN" >&2
  exit 1
fi
if [[ "$MAC_SERVICE_MODE" == "service" && ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd client/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ $ADB wait-for-device"
"${ADB_ARGS[@]}" wait-for-device
create_mac_authorization_key

if [[ "$MAC_SERVICE_MODE" == "service" ]]; then
  mkdir -p "$WORK_DIR/state"

  echo "+ start mac client-core-service on $MAC_SERVICE_HOST"
  SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST" \
    SLAN_CONTROL_BASE_URL="$BIZ_URL" \
    SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}" \
    SLAN_STATE_DIR="$WORK_DIR/state" \
    "$SERVICE_BIN" >"$MAC_SERVICE_LOG" 2>&1 &
  PIDS+=("$!")
  run_client_core_activation_check "mac local service health" \
    -address "$MAC_SERVICE_HOST" \
    -health-only=true \
    -timeout 10s
elif [[ "$MAC_SERVICE_MODE" == "app" ]]; then
  ensure_macos_app_service
else
  echo "+ use existing mac client-core-service at $MAC_SERVICE_HOST"
  reset_existing_macos_service_identity "$SERVICE_BIN"
  verify_existing_macos_service "$SERVICE_BIN"
fi

if [[ -n "${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-}" ]]; then
  echo "+ set macOS relay transport allowlist: ${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST}"
  set_macos_relay_transport_allowlist "${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST}"
fi

echo "+ activate Mac service at $MAC_SERVICE_HOST"
if ! MAC_OUTPUT="$(
  run_client_core_activation_check "mac socket activation" \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -authorization-key "$MAC_AUTHORIZATION_KEY" \
    -enable-network=false \
    -timeout "$TIMEOUT"
)"; then
  echo "$MAC_OUTPUT" >&2
  if grep -q "connect utun control socket: Operation not permitted" "$MAC_SERVICE_LOG" 2>/dev/null; then
    echo "macOS utun creation was denied. Run this socket data-plane check from a host context that can open utun, or set SLAN_MACOS_NETWORK_MOCK=1 for a control-plane-only check." >&2
  fi
  if grep -q "connect utun control socket: Operation not permitted" "$MAC_CORE_LOG" 2>/dev/null; then
    echo "macOS utun creation was denied. Run this socket data-plane check from a host context that can open utun, or set SLAN_MACOS_NETWORK_MOCK=1 for a control-plane-only check." >&2
  fi
  exit 1
fi
echo "$MAC_OUTPUT"
MAC_DEVICE_ID="$(echo "$MAC_OUTPUT" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
if [[ -z "$MAC_DEVICE_ID" ]]; then
  echo "failed to parse Mac device id from login output" >&2
  exit 1
fi

ensure_network_context
provision_network_membership_and_acl "$MAC_DEVICE_ID"
provision_socket_dns_record "$MAC_DEVICE_ID"
reload_mac_network_snapshot
if [[ -z "${MAC_IP:-}" ]]; then
  fail "failed to parse Mac virtual IP after network assignment"
fi
echo "Mac network IP: $MAC_IP"
ensure_mac_virtual_ip_bound

if [[ "$RUN_MAC_LOCAL_DNS_SMOKE" == "1" ]]; then
  echo "+ run mac local dns smoke on $MAC_SERVICE_HOST"
  (
    cd "$ROOT_DIR"
    SLAN_CLIENT_CORE_SERVICE_HOST="${MAC_SERVICE_HOST%:*}" \
      SLAN_CLIENT_CORE_SERVICE_PORT="${MAC_SERVICE_HOST##*:}" \
      bash scripts/tests/shared/desktop_local_dns_smoke.sh
  )
fi

if [[ "${SLAN_MACOS_NETWORK_MOCK:-0}" == "1" ]]; then
  echo "macAndroidSocketCheck: control-plane ok macIp=$MAC_IP targetHost=${PROVISIONED_SOCKET_TARGET_HOST:-$MAC_IP}; macOS network mock enabled so real UDP/TCP echo is skipped"
  exit 0
fi

echo "+ start Mac UDP/TCP echo server"
(
  cd "$ROOT_DIR"
  "$GO_BIN" run scripts/socket_echo_server.go \
    -udp-port "$UDP_PORT" \
    -tcp-port "$TCP_PORT" \
    -listen-host "$MAC_IP"
) >"$ECHO_LOG" 2>&1 &
ECHO_PID="$!"
PIDS+=("$ECHO_PID")

for _ in $(seq 1 30); do
  if grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$ECHO_LOG" && grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$ECHO_LOG"; then
    break
  fi
  if ! kill -0 "$ECHO_PID" 2>/dev/null; then
    cat "$ECHO_LOG"
    echo "Mac echo server exited before ready" >&2
    exit 1
  fi
  sleep 1
done
if ! grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$ECHO_LOG" || ! grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$ECHO_LOG"; then
  cat "$ECHO_LOG"
  echo "Mac echo server did not become ready" >&2
  exit 1
fi

echo "+ build and pre-authorize Android VPN"
if [[ "${SLAN_SKIP_ANDROID_BUILD:-0}" != "1" ]]; then
  (
    cd "$APP_DIR"
    flutter build apk --debug >/dev/null
  )
else
  echo "+ skip Android build and reuse existing debug APK"
fi
ANDROID_APK="$APP_DIR/build/app/outputs/flutter-apk/app-debug.apk"
install_output=""
for install_attempt in 1 2 3; do
  if install_output="$("${ADB_ARGS[@]}" install -r "$ANDROID_APK" 2>&1)"; then
    break
  fi
  if [[ "$install_output" != *"INSTALL_FAILED_PACKAGE_CHANGED"* || "$install_attempt" == "3" ]]; then
    echo "$install_output" >&2
    fail "Android APK install failed: $ANDROID_APK"
  fi
  echo "Android package changed during install; retrying ($install_attempt/3)" >&2
  sleep 2
done
echo "$install_output"
if [[ "${SLAN_ANDROID_CLEAR_APP:-1}" == "1" ]]; then
  if ! clear_output="$("${ADB_ARGS[@]}" shell pm clear dev.slan.slan_client_v2 2>&1)"; then
    echo "$clear_output" >&2
    fail "Android app clear failed"
  fi
  echo "$clear_output"
fi
if ! appops_set_output="$("${ADB_ARGS[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow 2>&1)"; then
  echo "$appops_set_output" >&2
  fail "Android ACTIVATE_VPN appops set failed"
fi
[[ -n "$appops_set_output" ]] && echo "$appops_set_output"
if ! appops_get_output="$("${ADB_ARGS[@]}" shell cmd appops get dev.slan.slan_client_v2 ACTIVATE_VPN 2>&1)"; then
  echo "$appops_get_output" >&2
  fail "Android ACTIVATE_VPN appops get failed"
fi
echo "$appops_get_output"
start_android_vpn_appops_guard

echo "+ run Android UDP/TCP sender target=${SOCKET_TARGET_HOST:-$MAC_IP} udp=$UDP_PORT tcp=$TCP_PORT"
TARGET_HOST="${SOCKET_TARGET_HOST:-$MAC_IP}"
echo "+ Android post-enable wait seconds: $ANDROID_POST_ENABLE_WAIT_SECONDS"
if [[ "${SLAN_SKIP_ANDROID_SOCKET_SEND:-0}" == "1" ]]; then
  ANDROID_UDP_TARGET=""
  ANDROID_TCP_TARGET=""
else
  if [[ "${SLAN_SKIP_ANDROID_UDP_SEND:-0}" == "1" ]]; then
    ANDROID_UDP_TARGET=""
  else
    ANDROID_UDP_TARGET="$TARGET_HOST:$UDP_PORT"
  fi
  if [[ "${SLAN_SKIP_ANDROID_TCP_SEND:-0}" == "1" ]]; then
    ANDROID_TCP_TARGET=""
  else
    ANDROID_TCP_TARGET="$TARGET_HOST:$TCP_PORT"
  fi
fi
run_android_socket_test "$TARGET_HOST"
cat "$ANDROID_LOG"

ANDROID_IP="$(sed -n 's/.*SLAN_TEST_NETWORK_IP=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
[[ -n "$ANDROID_IP" ]] || fail "failed to parse Android virtual IP for reverse socket checks"
echo "+ start Android reverse UDP/TCP echo target=$ANDROID_IP"
start_android_echo_test
wait_android_echo_ready
ensure_mac_virtual_ip_bound
if [[ "${SLAN_SKIP_MAC_UDP_SEND:-0}" != "1" ]]; then
  MAC_UDP_BODY="mac-to-android-udp-$(date +%s%N)"
  UDP_RESULT="$(send_mac_udp_echo "$ANDROID_IP" "$MAC_UDP_BODY")" \
    || fail "Mac -> Android UDP echo failed"
  [[ "$UDP_RESULT" == "echo:${MAC_UDP_BODY}" ]] \
    || fail "Mac -> Android UDP response mismatch: $UDP_RESULT"
fi
if [[ "${SLAN_SKIP_MAC_TCP_SEND:-0}" != "1" ]]; then
  MAC_TCP_BODY="mac-to-android-tcp-$(date +%s%N)"
  TCP_RESULT="$(send_mac_tcp_echo "$ANDROID_IP" "$MAC_TCP_BODY")" \
    || fail "Mac -> Android TCP echo failed"
  [[ "$TCP_RESULT" == "echo:${MAC_TCP_BODY}" ]] \
    || fail "Mac -> Android TCP response mismatch: $TCP_RESULT"
fi
kill "$ANDROID_ECHO_PID" 2>/dev/null || true
wait "$ANDROID_ECHO_PID" 2>/dev/null || true
cat "$ANDROID_ECHO_LOG"

if [[ "${SLAN_SKIP_ANDROID_UDP_SEND:-0}" != "1" ]] && ! grep -q "SLAN_TEST_UDP_ECHO_OK=$TARGET_HOST:$UDP_PORT" "$ANDROID_LOG"; then
  echo "Android UDP echo check did not complete" >&2
  exit 1
fi
if [[ "${SLAN_SKIP_ANDROID_TCP_SEND:-0}" != "1" ]] && ! grep -q "SLAN_TEST_TCP_ECHO_OK=$TARGET_HOST:$TCP_PORT" "$ANDROID_LOG"; then
  echo "Android TCP echo check did not complete" >&2
  exit 1
fi
if [[ "${SLAN_SKIP_ANDROID_UDP_SEND:-0}" != "1" ]] && ! grep -q "SOCKET_ECHO_UDP_RECEIVED=" "$ECHO_LOG"; then
  echo "Mac UDP echo server did not receive data" >&2
  exit 1
fi
if [[ "${SLAN_SKIP_ANDROID_TCP_SEND:-0}" != "1" ]] && ! grep -q "SOCKET_ECHO_TCP_RECEIVED=" "$ECHO_LOG"; then
  echo "Mac TCP echo server did not receive data" >&2
  exit 1
fi
if [[ -n "${SLAN_EXPECT_ANDROID_RELAY_URL_CONTAINS:-}" ]]; then
  assert_android_config_contains "relayUrls=[^ ]*${SLAN_EXPECT_ANDROID_RELAY_URL_CONTAINS}"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_PATH_KIND_CONTAINS:-}" ]]; then
  assert_android_config_contains "pathKinds=[^ ]*${SLAN_EXPECT_ANDROID_PATH_KIND_CONTAINS}"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_DIRECT_CANDIDATES_CONTAINS:-}" ]]; then
  assert_android_config_contains "directCandidates=[^ ]*${SLAN_EXPECT_ANDROID_DIRECT_CANDIDATES_CONTAINS}"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_DIRECT_READY_MIN:-}" ]]; then
  assert_android_stat_min "directUdpReadyPeerCount" "$SLAN_EXPECT_ANDROID_DIRECT_READY_MIN"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_DIRECT_READY_MAX:-}" ]]; then
  assert_android_stat_max "directUdpReadyPeerCount" "$SLAN_EXPECT_ANDROID_DIRECT_READY_MAX"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_DIRECT_FRAMES_SENT_MIN:-}" ]]; then
  assert_android_stat_min "directUdpFramesSent" "$SLAN_EXPECT_ANDROID_DIRECT_FRAMES_SENT_MIN"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_RELAY_FRAMES_SENT_MIN:-}" ]]; then
  assert_android_stat_min "relayFramesSent" "$SLAN_EXPECT_ANDROID_RELAY_FRAMES_SENT_MIN"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_RELAY_TCP_SYN_ACK_MIN:-}" ]]; then
  assert_android_stat_min "relayTcpSynAckReceived" "$SLAN_EXPECT_ANDROID_RELAY_TCP_SYN_ACK_MIN"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_RELAY_TCP_FRAMES_RECEIVED_MIN:-}" ]]; then
  assert_android_stat_min "relayTcpFramesReceived" "$SLAN_EXPECT_ANDROID_RELAY_TCP_FRAMES_RECEIVED_MIN"
fi
if [[ -n "${SLAN_EXPECT_ANDROID_DERP_PEER_IPS_CONTAINS:-}" ]]; then
  assert_android_runtime_contains "\"derpPeerVirtualIps\":\\[[^]]*${SLAN_EXPECT_ANDROID_DERP_PEER_IPS_CONTAINS}"
fi
cat "$ECHO_LOG"

echo "macAndroidSocketCheck: ok macIp=$MAC_IP targetHost=$TARGET_HOST udp=$UDP_PORT tcp=$TCP_PORT"
echo "macAndroidBusinessCheck: ok mqtt=connected network=ready dns=resolved acl=applied androidToMac=udp,tcp macToAndroid=udp,tcp"
