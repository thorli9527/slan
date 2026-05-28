#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-http://10.0.2.2:28080}"
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
IOS_DEVICE="${SLAN_IOS_FLUTTER_DEVICE:-$(xcrun simctl list devices booted | awk -F'[()]' '/Booted/ && /iPhone|iPad/ { print $2; exit }')}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
GENERATED_TEST_EMAIL=0
if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
else
  EMAIL="ios-android-socket-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-ios-to-android-socket-$(date +%s%N)}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-ios-to-android-tcp-$(date +%s%N)}"
IOS_SEND_UDP="${SLAN_IOS_SEND_UDP:-0}"
WORK_DIR="${SLAN_IOS_ANDROID_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-ios-android-socket.XXXXXX")}"
ANDROID_LOG="$WORK_DIR/android-echo.log"
IOS_LOG="$WORK_DIR/ios-client.log"
POSTGRES_CONTAINER="${SLAN_POSTGRES_CONTAINER:-}"
POSTGRES_USER="${SLAN_POSTGRES_USER:-postgres}"
POSTGRES_DB="${SLAN_POSTGRES_DB:-slan}"

PIDS=()

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
      "$ADB" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
      sleep 0.25
    done
  ) &
  PIDS+=("$!")
}

wait_fresh_control_session() {
  local device_id="$1"
  local label="$2"
  if [[ "${SLAN_SKIP_DB_FRESH_WAIT:-0}" == "1" ]]; then
    return 0
  fi
  if [[ -z "$device_id" ]]; then
    echo "$label device id is empty; cannot wait for control session" >&2
    return 1
  fi
  resolve_postgres_container
  local escaped_email="${EMAIL//\'/\'\'}"
  local escaped_device_id="${device_id//\'/\'\'}"
  local sql="
select count(*)
from control_sessions cs
join users u on u.active_network_id = cs.network_id
where u.email = '$escaped_email'
  and cs.device_id = '$escaped_device_id'
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
  if [[ "${SLAN_SKIP_DB_FRESH_WAIT:-0}" == "1" ]]; then
    return 0
  fi
  if [[ -z "$device_id" ]]; then
    echo "$label device id is empty; cannot wait for online network state" >&2
    return 1
  fi
  resolve_postgres_container
  local escaped_email="${EMAIL//\'/\'\'}"
  local escaped_device_id="${device_id//\'/\'\'}"
  local sql="
select count(*)
from device_network_states dns
join users u on u.active_network_id = dns.network_id
where u.email = '$escaped_email'
  and dns.device_id = '$escaped_device_id'
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
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
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
"$ADB" wait-for-device

echo "+ build and pre-authorize Android VPN"
(
  cd "$APP_DIR"
  flutter build apk --debug >/dev/null
)
"$ADB" install -r "$APP_DIR/build/app/outputs/flutter-apk/app-debug.apk" >/dev/null
"$ADB" shell pm clear dev.slan.slan_client_v2 >/dev/null
"$ADB" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow
"$ADB" shell cmd appops get dev.slan.slan_client_v2 ACTIVATE_VPN

xcrun simctl uninstall "$IOS_DEVICE" dev.slan.client.v2 >/dev/null 2>&1 || true

ANDROID_REGISTER_USER=false
if [[ "$IOS_SEND_UDP" == "1" ]]; then
  ANDROID_REGISTER_USER=true
else
  echo "+ warm-enable iOS network and keep control session fresh"
  (
    cd "$APP_DIR"
    flutter test integration_test/mobile_login_test.dart \
      -d "$IOS_DEVICE" \
      --dart-define="SLAN_TEST_BIZ_URL=$BIZ_URL" \
      --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$BIZ_URL" \
      --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
      --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
      --dart-define="SLAN_TEST_REGISTER_USER=true" \
      --dart-define="SLAN_TEST_WAIT_MQTT=true" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=${SLAN_IOS_PEER_HOLD_SECONDS:-120}"
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
  wait_fresh_control_session "$IOS_DEVICE_ID" "iOS"
  wait_online_network_state "$IOS_DEVICE_ID" "iOS"
fi

echo "+ start Android UDP echo integration test"
start_android_vpn_appops_guard
(
  cd "$APP_DIR"
  flutter test integration_test/mobile_login_test.dart \
    -d "$ANDROID_DEVICE" \
    --dart-define="SLAN_TEST_BIZ_URL=$ANDROID_BIZ_URL" \
    --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$ANDROID_BIZ_URL" \
    --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
    --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
    --dart-define="SLAN_TEST_REGISTER_USER=$ANDROID_REGISTER_USER" \
    --dart-define="SLAN_TEST_WAIT_MQTT=true" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
    --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
    --dart-define="SLAN_TEST_HOLD_SECONDS=${SLAN_ANDROID_ECHO_HOLD_SECONDS:-90}"
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
(
  cd "$APP_DIR"
  flutter test integration_test/mobile_login_test.dart \
    -d "$IOS_DEVICE" \
    --dart-define="SLAN_TEST_BIZ_URL=$BIZ_URL" \
    --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$BIZ_URL" \
    --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
    --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
    --dart-define="SLAN_TEST_REGISTER_USER=false" \
    --dart-define="SLAN_TEST_WAIT_MQTT=true" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=${SLAN_IOS_SEND_POST_ENABLE_WAIT_SECONDS:-8}" \
    --dart-define="SLAN_TEST_UDP_SEND_TARGET=$ANDROID_IP:$UDP_PORT" \
    --dart-define="SLAN_TEST_UDP_SEND_BODY=$UDP_BODY" \
    --dart-define="SLAN_TEST_TCP_SEND_TARGET=$ANDROID_IP:$TCP_PORT" \
    --dart-define="SLAN_TEST_TCP_SEND_BODY=$TCP_BODY"
) >"$IOS_LOG" 2>&1
cat "$IOS_LOG"

echo "+ wait Android echo test"
wait "$ANDROID_PID"
cat "$ANDROID_LOG"

echo "iosAndroidSocketCheck: ok email=$EMAIL androidIp=$ANDROID_IP udp=$UDP_PORT tcp=$TCP_PORT"
