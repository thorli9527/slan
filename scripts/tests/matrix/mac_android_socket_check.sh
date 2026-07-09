#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/lib/flutter_mobile_login_test.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
GO_BIN="${SLAN_GO_BIN:-/opt/homebrew/bin/go}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"
MACOS_APP_PATH="${SLAN_MACOS_APP_PATH:-$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
if [[ -n "${SLAN_ANDROID_BIZ_URL:-}" ]]; then
  ANDROID_BIZ_URL="$SLAN_ANDROID_BIZ_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  ANDROID_BIZ_URL="http://10.0.2.2:28080"
else
  ANDROID_BIZ_URL="$BIZ_URL"
fi
if [[ -n "${SLAN_WEB_BIZ_URL:-}" ]]; then
  ADMIN_BIZ_URL="$SLAN_WEB_BIZ_URL"
elif [[ -n "${SLAN_NETWORK_ADMIN_BIZ_URL:-}" ]]; then
  ADMIN_BIZ_URL="$SLAN_NETWORK_ADMIN_BIZ_URL"
elif [[ "$BIZ_URL" == *":28080" ]]; then
  ADMIN_BIZ_URL="${BIZ_URL%:28080}:28081"
else
  ADMIN_BIZ_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
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
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
  GENERATED_TEST_EMAIL=0
else
  EMAIL="mac-android-socket-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"
REGISTER_USER="${SLAN_TEST_REGISTER_USER:-true}"
TIMEOUT="${SLAN_MAC_ANDROID_SOCKET_TIMEOUT:-90s}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-android-to-mac-udp-$(date +%s%N)}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-android-to-mac-tcp-$(date +%s%N)}"
SOCKET_TARGET_HOST="${SLAN_TEST_SOCKET_TARGET_HOST:-}"
AUTO_PROVISION_SOCKET_DNS="${SLAN_AUTO_PROVISION_SOCKET_DNS:-0}"
# Give the peer side time to finish relay attach before Android starts
# emitting socket traffic. In practice the macOS service may need one
# relay reconfigure cycle after Android enables the tunnel.
ANDROID_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-35}"
WORK_DIR="${SLAN_MAC_ANDROID_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-android-socket.XXXXXX")}"
ECHO_LOG="$WORK_DIR/macos-echo.log"
ANDROID_LOG="$WORK_DIR/android-socket.log"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"
INSTALLED_MAC_SERVICE_BIN="/Library/Application Support/SLAN/client-core-service"
RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
SUDO_PASSWORD="${SLAN_SUDO_PASSWORD:-}"
ANDROID_TEST_TIMEOUT_SECONDS="${SLAN_ANDROID_TEST_TIMEOUT_SECONDS:-180}"

NETWORK_ID=""
USER_ID=""
ZONE_ID=""
ZONE_NAME=""
RECORD_ID=""
PROVISIONED_SOCKET_TARGET_HOST=""

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
  curl --silent --show-error --connect-timeout 5 --max-time 20 -X DELETE "$url" >/dev/null 2>&1 || true
}

create_json() {
  local url="$1"
  local payload="$2"
  curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "$url" \
    -H 'Content-Type: application/json' \
    -d "$payload"
}

echo "+ mac android socket defaults: account=$EMAIL biz=$BIZ_URL admin_biz=$ADMIN_BIZ_URL service_host=$MAC_SERVICE_HOST android_device=$ANDROID_DEVICE"

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
  mapfile -t ANDROID_COMMON_DART_DEFINES < <(
    slan_mobile_login_common_defines "$ANDROID_BIZ_URL" "$EMAIL" "$PASSWORD" false true
  )
  (
    cd "$APP_DIR"
    flutter test integration_test/mobile_login_test.dart \
      -d "$ANDROID_DEVICE" \
      "${ANDROID_COMMON_DART_DEFINES[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=${SLAN_ANDROID_TEST_DEVICE_ID:-}" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
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
  [[ -x "$INSTALLED_MAC_SERVICE_BIN" ]] || fail "installed mac client-core-service is missing: $INSTALLED_MAC_SERVICE_BIN"

  expected_hash="$(sha256_file "$expected_bin")"
  installed_hash="$(sha256_file "$INSTALLED_MAC_SERVICE_BIN")"
  echo "macServiceExpectedSha256: $expected_hash"
  echo "macServiceInstalledSha256: $installed_hash"
  if [[ "$expected_hash" != "$installed_hash" ]]; then
    fail "installed mac client-core-service is stale; reinstall with: sudo scripts/install_macos_service.sh --binary $expected_bin"
  fi

  if ! health_output="$(
    run_client_core_login_check "mac service health" \
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
    run_client_core_login_check "mac app service health" \
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
    SLAN_RESET_MACOS_IDENTITY=1
    SLAN_DIRECT_UDP_ENDPOINT="${SLAN_DIRECT_UDP_ENDPOINT:-}"
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
  [[ -n "$NETWORK_ID" && -n "$USER_ID" ]] && return 0

  local auth networks
  auth="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "${ADMIN_BIZ_URL}/api/web/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
  USER_ID="$(extract_json_field "$auth" "userId")"
  [[ -n "$USER_ID" ]] || fail "failed to parse user id from auth login response"
  NETWORK_ID="$(extract_network_id_from_auth "$auth")"
  if [[ -n "$NETWORK_ID" ]]; then
    return 0
  fi

  networks="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    "${ADMIN_BIZ_URL}/api/web/networks?userId=${USER_ID}")"
  NETWORK_ID="$(extract_json_field "$networks" "networkId")"
  [[ -n "$NETWORK_ID" ]] || fail "failed to parse network id for user ${USER_ID}"
}

provision_socket_dns_record() {
  local mac_device_id="$1"
  is_truthy "$AUTO_PROVISION_SOCKET_DNS" || return 0
  ensure_network_context

  ZONE_NAME="socket-${RANDOM}-$(date +%s).lan"
  local response
  response="$(create_json "${ADMIN_BIZ_URL}/api/web/networks/${NETWORK_ID}/dns/zones" \
    "{\"actorUserId\":\"${USER_ID}\",\"zoneName\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(extract_json_field "$response" "zoneId")"
  [[ -n "$ZONE_ID" ]] || fail "failed to create dns zone ${ZONE_NAME}"

  response="$(create_json "${ADMIN_BIZ_URL}/api/web/networks/${NETWORK_ID}/dns/records" \
    "{\"actorUserId\":\"${USER_ID}\",\"zoneId\":\"${ZONE_ID}\",\"name\":\"mac\",\"recordType\":\"A\",\"targetDeviceId\":\"${mac_device_id}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"${TCP_PORT}\",\"ttl\":60}")"
  RECORD_ID="$(extract_json_field "$response" "recordId")"
  [[ -n "$RECORD_ID" ]] || fail "failed to create socket dns record"

  PROVISIONED_SOCKET_TARGET_HOST="mac.${ZONE_NAME}"
  SOCKET_TARGET_HOST="$PROVISIONED_SOCKET_TARGET_HOST"
  echo "macAndroidSocketCheck: provisioned dnsZone=${ZONE_NAME} recordHost=${PROVISIONED_SOCKET_TARGET_HOST} network=${NETWORK_ID} device=${mac_device_id}"
}

cleanup() {
  status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$ECHO_LOG" ]] && { echo "---- Mac echo log ----" >&2; cat "$ECHO_LOG" >&2; }
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android socket log ----" >&2; cat "$ANDROID_LOG" >&2; }
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
  if [[ -n "$NETWORK_ID" ]]; then
    [[ -n "$RECORD_ID" ]] && best_effort_delete "${ADMIN_BIZ_URL}/api/web/networks/${NETWORK_ID}/dns/records/${RECORD_ID}?actorUserId=${USER_ID}"
    [[ -n "$ZONE_ID" ]] && best_effort_delete "${ADMIN_BIZ_URL}/api/web/networks/${NETWORK_ID}/dns/zones/${ZONE_ID}?actorUserId=${USER_ID}"
  fi
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
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
  echo "run: cd client_v2/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ $ADB wait-for-device"
"${ADB_ARGS[@]}" wait-for-device

if [[ "$MAC_SERVICE_MODE" == "service" ]]; then
  mkdir -p "$WORK_DIR/state"

  echo "+ start mac client-core-service on $MAC_SERVICE_HOST"
  SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST" \
    SLAN_CONTROL_BASE_URL="$BIZ_URL" \
    SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}" \
    SLAN_DIRECT_UDP_ENDPOINT="${SLAN_DIRECT_UDP_ENDPOINT:-}" \
    SLAN_STATE_DIR="$WORK_DIR/state" \
    "$SERVICE_BIN" >"$MAC_SERVICE_LOG" 2>&1 &
  PIDS+=("$!")
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

echo "+ login and enable Mac service network at $MAC_SERVICE_HOST"
if ! MAC_OUTPUT="$(
  run_client_core_login_check "mac socket login" \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register="$REGISTER_USER" \
    -enable-network=true \
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
MAC_IP="$(echo "$MAC_OUTPUT" | sed -n 's/.*clientCoreServiceNetwork: enabled virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
if [[ -z "$MAC_DEVICE_ID" ]]; then
  echo "failed to parse Mac device id from login output" >&2
  exit 1
fi
if [[ -z "$MAC_IP" ]]; then
  echo "failed to parse Mac virtual IP from login output" >&2
  exit 1
fi
MAC_IP="${MAC_IP%%/*}"
echo "Mac network IP: $MAC_IP"

provision_socket_dns_record "$MAC_DEVICE_ID"

if [[ "${SLAN_MACOS_NETWORK_MOCK:-0}" == "1" ]]; then
  echo "macAndroidSocketCheck: control-plane ok email=$EMAIL macIp=$MAC_IP targetHost=${PROVISIONED_SOCKET_TARGET_HOST:-$MAC_IP}; macOS network mock enabled so real UDP/TCP echo is skipped"
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
if ! install_output="$("${ADB_ARGS[@]}" install -r "$ANDROID_APK" 2>&1)"; then
  echo "$install_output" >&2
  fail "Android APK install failed: $ANDROID_APK"
fi
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

echo "macAndroidSocketCheck: ok email=$EMAIL macIp=$MAC_IP targetHost=$TARGET_HOST udp=$UDP_PORT tcp=$TCP_PORT"
