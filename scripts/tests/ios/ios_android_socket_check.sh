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
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
if [[ -n "${SLAN_ANDROID_BIZ_URL:-}" ]]; then
  ANDROID_BIZ_URL="$SLAN_ANDROID_BIZ_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  ANDROID_BIZ_URL="http://10.0.2.2:28080"
else
  ANDROID_BIZ_URL="$BIZ_URL"
fi
if [[ -n "${SLAN_SKIP_DB_FRESH_WAIT:-}" ]]; then
  SKIP_DB_FRESH_WAIT="$SLAN_SKIP_DB_FRESH_WAIT"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  SKIP_DB_FRESH_WAIT=0
else
  # Remote biz stacks do not share the local postgres container used below.
  SKIP_DB_FRESH_WAIT=1
fi
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
ADB_ARGS=("$ADB")
if [[ -n "$ANDROID_DEVICE" ]]; then
  ADB_ARGS+=(-s "$ANDROID_DEVICE")
fi
IOS_DEVICE="${SLAN_IOS_FLUTTER_DEVICE:-$(xcrun simctl list devices booted | awk -F'[()]' '/Booted/ && /iPhone|iPad/ { print $2; exit }')}"
IOS_TEST_DEVICE_ID="${SLAN_IOS_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-')}"
ANDROID_TEST_DEVICE_ID="${SLAN_ANDROID_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-')}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-ios-to-android-socket-$(date +%s%N)}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-ios-to-android-tcp-$(date +%s%N)}"
IOS_SEND_UDP="${SLAN_IOS_SEND_UDP:-0}"
# These flows start the receiver side first, then keep it alive while the peer
# is built / launched and begins probing. The shorter historical defaults were
# enough to let the receiver exit on slower runs before socket probes started.
IOS_PEER_HOLD_SECONDS="${SLAN_IOS_PEER_HOLD_SECONDS:-240}"
ANDROID_ECHO_HOLD_SECONDS="${SLAN_ANDROID_ECHO_HOLD_SECONDS:-240}"
WORK_DIR="${SLAN_IOS_ANDROID_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-ios-android-socket.XXXXXX")}"
ANDROID_LOG="$WORK_DIR/android-echo.log"
IOS_LOG="$WORK_DIR/ios-client.log"
POSTGRES_CONTAINER="${SLAN_POSTGRES_CONTAINER:-}"
POSTGRES_USER="${SLAN_POSTGRES_USER:-postgres}"
POSTGRES_DB="${SLAN_POSTGRES_DB:-slan}"

PIDS=()
OPS_TOKEN=""
NETWORK_ID=""
IOS_CREDENTIAL_ID=""
IOS_AUTHORIZATION_KEY=""
ANDROID_CREDENTIAL_ID=""
ANDROID_AUTHORIZATION_KEY=""

read_lines_into_array() {
  local target_var="$1"
  local line
  local -a values=()
  while IFS= read -r line; do
    values+=("$line")
  done
  eval "$target_var=()"
  local value
  for value in "${values[@]}"; do
    eval "$target_var+=(\"\$value\")"
  done
}

provision_ops_resources() {
  local credential network
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "iOS Android Socket iOS")"
  IOS_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  IOS_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "iOS Android Socket Android")"
  ANDROID_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  ANDROID_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "ios-android-socket-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network" | jq -er '.networkId')"
}

resolve_postgres_container() {
  if [[ -n "$POSTGRES_CONTAINER" ]]; then
    return 0
  fi
  POSTGRES_CONTAINER="$(docker compose -f "$ROOT_DIR/docker-compose.local.yml" ps -q postgres 2>/dev/null || true)"
  if [[ -z "$POSTGRES_CONTAINER" ]]; then
    POSTGRES_CONTAINER="slan-postgres"
  fi
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

wait_fresh_control_session() {
  local device_id="$1"
  local label="$2"
  if [[ "$SKIP_DB_FRESH_WAIT" == "1" ]]; then
    echo "$label skipping local control session freshness check for remote biz url: $BIZ_URL"
    return 0
  fi
  if [[ -z "$device_id" ]]; then
    echo "$label device id is empty; cannot wait for control session" >&2
    return 1
  fi
  resolve_postgres_container
  local escaped_device_id="${device_id//\'/\'\'}"
  local sql="
select count(*)
from control_sessions cs
where cs.device_id = '$escaped_device_id'
  and cs.last_seen_at >= extract(epoch from now())::bigint - 45;
"
  for _ in $(seq 1 "${SLAN_CONTROL_SESSION_WAIT_SECONDS:-45}"); do
    local count
    count="$(docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "$sql" 2>/dev/null | tr -d '[:space:]' || true)"
    if [[ "$count" =~ ^[0-9]+$ ]] && (( count > 0 )); then
      echo "$label control session is fresh"
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $label fresh control session deviceId=$device_id" >&2
  return 1
}

wait_online_network_state() {
  local device_id="$1"
  local label="$2"
  if [[ "$SKIP_DB_FRESH_WAIT" == "1" ]]; then
    echo "$label skipping local network state check for remote biz url: $BIZ_URL"
    return 0
  fi
  if [[ -z "$device_id" ]]; then
    echo "$label device id is empty; cannot wait for online network state" >&2
    return 1
  fi
  resolve_postgres_container
  local escaped_device_id="${device_id//\'/\'\'}"
  local sql="
select count(*)
from device_network_states dns
where dns.device_id = '$escaped_device_id'
  and dns.control_reachable = true
  and dns.network_online = true
  and dns.tunnel_up = true
  and dns.last_seen_at >= extract(epoch from now())::bigint - 45;
"
  for _ in $(seq 1 "${SLAN_NETWORK_STATE_WAIT_SECONDS:-45}"); do
    local count
    count="$(docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "$sql" 2>/dev/null | tr -d '[:space:]' || true)"
    if [[ "$count" =~ ^[0-9]+$ ]] && (( count > 0 )); then
      echo "$label network state is online"
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $label online network state deviceId=$device_id" >&2
  return 1
}

cleanup() {
  status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$WORK_DIR/android-prelogin.log" ]] && { echo "---- Android prelogin log ----" >&2; cat "$WORK_DIR/android-prelogin.log" >&2; }
    [[ -f "$WORK_DIR/ios-prelogin.log" ]] && { echo "---- iOS prelogin log ----" >&2; cat "$WORK_DIR/ios-prelogin.log" >&2; }
    [[ -f "$WORK_DIR/android-warm-network.log" ]] && { echo "---- Android warm network log ----" >&2; cat "$WORK_DIR/android-warm-network.log" >&2; }
    [[ -f "$WORK_DIR/ios-warm-network.log" ]] && { echo "---- iOS warm network log ----" >&2; cat "$WORK_DIR/ios-warm-network.log" >&2; }
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android echo log ----" >&2; cat "$ANDROID_LOG" >&2; }
    [[ -f "$IOS_LOG" ]] && { echo "---- iOS client log ----" >&2; cat "$IOS_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$IOS_CREDENTIAL_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$ANDROID_CREDENTIAL_ID" >/dev/null 2>&1 || true
  if [[ "${SLAN_KEEP_IOS_ANDROID_SOCKET_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [[ -z "$IOS_DEVICE" ]]; then
  echo "no booted iOS simulator found; boot one or set SLAN_IOS_FLUTTER_DEVICE" >&2
  exit 1
fi
if [[ ! -x "$ADB" ]]; then
  echo "adb is missing or not executable: $ADB" >&2
  exit 1
fi

echo "+ $ADB wait-for-device"
"${ADB_ARGS[@]}" wait-for-device

echo "+ build and pre-authorize Android VPN"
(
  cd "$APP_DIR"
  flutter build apk --debug >/dev/null
)
"${ADB_ARGS[@]}" install -r "$APP_DIR/build/app/outputs/flutter-apk/app-debug.apk" >/dev/null
"${ADB_ARGS[@]}" shell pm clear dev.slan.slan_client_v2 >/dev/null
"${ADB_ARGS[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow
"${ADB_ARGS[@]}" shell cmd appops get dev.slan.slan_client_v2 ACTIVATE_VPN

xcrun simctl uninstall "$IOS_DEVICE" dev.slan.client.v2 >/dev/null 2>&1 || true
provision_ops_resources

echo "+ activate iOS device before network assignment"
read_lines_into_array IOS_ACTIVATION_DART_DEFINES < <(
  slan_mobile_activation_common_defines "$BIZ_URL" "$IOS_AUTHORIZATION_KEY" true
)
(
  cd "$APP_DIR"
  flutter test integration_test/device_activation_harness_test.dart \
    -d "$IOS_DEVICE" \
    "${IOS_ACTIVATION_DART_DEFINES[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID"
) >"$WORK_DIR/ios-prelogin.log" 2>&1
IOS_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$WORK_DIR/ios-prelogin.log" | tail -n 1)"
[[ "$IOS_DEVICE_ID" == "$IOS_TEST_DEVICE_ID" ]] || {
  cat "$WORK_DIR/ios-prelogin.log"
  echo "iOS activation device id mismatch: expected=$IOS_TEST_DEVICE_ID actual=$IOS_DEVICE_ID" >&2
  exit 1
}

echo "+ activate Android device before network assignment"
read_lines_into_array ANDROID_ACTIVATION_DART_DEFINES < <(
  slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true
)
(
  cd "$APP_DIR"
  flutter test integration_test/device_activation_harness_test.dart \
    -d "$ANDROID_DEVICE" \
    "${ANDROID_ACTIVATION_DART_DEFINES[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID"
) >"$WORK_DIR/android-prelogin.log" 2>&1
ANDROID_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$WORK_DIR/android-prelogin.log" | tail -n 1)"
[[ "$ANDROID_DEVICE_ID" == "$ANDROID_TEST_DEVICE_ID" ]] || {
  cat "$WORK_DIR/android-prelogin.log"
  echo "Android activation device id mismatch: expected=$ANDROID_TEST_DEVICE_ID actual=$ANDROID_DEVICE_ID" >&2
  exit 1
}

slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$IOS_DEVICE_ID"
slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$ANDROID_DEVICE_ID"

if [[ "$IOS_SEND_UDP" != "1" ]]; then
  echo "+ warm-enable iOS network and keep control session fresh"
  read_lines_into_array IOS_WARM_COMMON_DART_DEFINES < <(
    slan_mobile_activation_common_defines "$BIZ_URL" "$IOS_AUTHORIZATION_KEY" true
  )
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$IOS_DEVICE" \
      "${IOS_WARM_COMMON_DART_DEFINES[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=$IOS_PEER_HOLD_SECONDS"
  ) >"$WORK_DIR/ios-warm-network.log" 2>&1 &
  IOS_WARM_PID="$!"
  PIDS+=("$IOS_WARM_PID")
  for _ in $(seq 1 90); do
    if ! kill -0 "$IOS_WARM_PID" 2>/dev/null; then
      cat "$WORK_DIR/ios-warm-network.log"
      echo "iOS warm network test exited before network IP was reported" >&2
      exit 1
    fi
    IOS_IP="$(sed -n 's/.*SLAN_TEST_NETWORK_IP=\([^[:space:]]*\).*/\1/p' "$WORK_DIR/ios-warm-network.log" | tail -n 1)"
    if [[ -n "$IOS_IP" ]]; then
      break
    fi
    sleep 1
  done
  if [[ -z "${IOS_IP:-}" ]]; then
    cat "$WORK_DIR/ios-warm-network.log"
    echo "timed out waiting for iOS network IP" >&2
    exit 1
  fi
  echo "iOS network IP: $IOS_IP"
  IOS_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$WORK_DIR/ios-warm-network.log" | tail -n 1)"
  [[ "$IOS_DEVICE_ID" == "$IOS_TEST_DEVICE_ID" ]] || {
    cat "$WORK_DIR/ios-warm-network.log"
    echo "iOS device id mismatch: expected=$IOS_TEST_DEVICE_ID actual=$IOS_DEVICE_ID" >&2
    exit 1
  }
  wait_fresh_control_session "$IOS_DEVICE_ID" "iOS"
  wait_online_network_state "$IOS_DEVICE_ID" "iOS"
fi

echo "+ start Android UDP echo integration test"
start_android_vpn_appops_guard
read_lines_into_array ANDROID_ECHO_COMMON_DART_DEFINES < <(
  slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true
)
(
  cd "$APP_DIR"
  flutter test integration_test/device_activation_harness_test.dart \
    -d "$ANDROID_DEVICE" \
    "${ANDROID_ECHO_COMMON_DART_DEFINES[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
    --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
    --dart-define="SLAN_TEST_HOLD_SECONDS=$ANDROID_ECHO_HOLD_SECONDS"
) >"$ANDROID_LOG" 2>&1 &
ANDROID_PID="$!"
PIDS+=("$ANDROID_PID")

ANDROID_IP=""
for _ in $(seq 1 "${SLAN_ANDROID_IP_WAIT_SECONDS:-150}"); do
  if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
    cat "$ANDROID_LOG"
    echo "Android echo test exited before network IP was reported" >&2
    exit 1
  fi
  ANDROID_IP="$(sed -n 's/.*SLAN_TEST_NETWORK_IP=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
  if [[ -n "$ANDROID_IP" ]]; then
    break
  fi
  sleep 1
done
if [[ -z "$ANDROID_IP" ]]; then
  cat "$ANDROID_LOG"
  echo "timed out waiting for Android network IP" >&2
  exit 1
fi
echo "Android network IP: $ANDROID_IP"
ANDROID_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
[[ "$ANDROID_DEVICE_ID" == "$ANDROID_TEST_DEVICE_ID" ]] || {
  cat "$ANDROID_LOG"
  echo "Android device id mismatch: expected=$ANDROID_TEST_DEVICE_ID actual=$ANDROID_DEVICE_ID" >&2
  exit 1
}

for _ in $(seq 1 "${SLAN_ANDROID_STATE_WAIT_SECONDS:-30}"); do
  if grep -q "SLAN_TEST_TUNNEL_STATE=" "$ANDROID_LOG"; then
    break
  fi
  if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
    break
  fi
  sleep 1
done

ANDROID_RELAY_SESSION_COUNT="$(sed -n 's/.*"requestedRelaySessionCount":\([0-9][0-9]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
ANDROID_RELAY_SESSION_COUNT="${ANDROID_RELAY_SESSION_COUNT:-0}"
echo "Android requested relay sessions: $ANDROID_RELAY_SESSION_COUNT"

if [[ "$IOS_SEND_UDP" != "1" ]]; then
  cat "$ANDROID_LOG"
  cat "$WORK_DIR/ios-warm-network.log"
  echo "iosAndroidSocketCheck: control/data config ok androidIp=$ANDROID_IP iosIp=$IOS_IP androidRelaySessions=$ANDROID_RELAY_SESSION_COUNT; iOS simulator true UDP/TCP send skipped (set SLAN_IOS_SEND_UDP=1 for a real iOS PacketTunnel target)"
  exit 0
fi

echo "+ run iOS UDP/TCP echo client integration test target=$ANDROID_IP udp=$UDP_PORT tcp=$TCP_PORT"
read_lines_into_array IOS_SEND_COMMON_DART_DEFINES < <(
  slan_mobile_activation_common_defines "$BIZ_URL" "$IOS_AUTHORIZATION_KEY" true
)
(
  cd "$APP_DIR"
  flutter test integration_test/device_activation_harness_test.dart \
    -d "$IOS_DEVICE" \
    "${IOS_SEND_COMMON_DART_DEFINES[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=${SLAN_IOS_SEND_POST_ENABLE_WAIT_SECONDS:-8}" \
    --dart-define="SLAN_TEST_UDP_SEND_TARGET=$ANDROID_IP:$UDP_PORT" \
    --dart-define="SLAN_TEST_UDP_SEND_BODY=$UDP_BODY" \
    --dart-define="SLAN_TEST_TCP_SEND_TARGET=$ANDROID_IP:$TCP_PORT" \
    --dart-define="SLAN_TEST_TCP_SEND_BODY=$TCP_BODY"
) >"$IOS_LOG" 2>&1
cat "$IOS_LOG"
IOS_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$IOS_LOG" | tail -n 1)"
[[ "$IOS_DEVICE_ID" == "$IOS_TEST_DEVICE_ID" ]] || {
  echo "iOS device id mismatch: expected=$IOS_TEST_DEVICE_ID actual=$IOS_DEVICE_ID" >&2
  exit 1
}

echo "+ wait Android echo test"
wait "$ANDROID_PID"
cat "$ANDROID_LOG"

echo "iosAndroidSocketCheck: ok androidIp=$ANDROID_IP udp=$UDP_PORT tcp=$TCP_PORT"
