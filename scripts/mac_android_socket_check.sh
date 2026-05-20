#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
GO_BIN="${SLAN_GO_BIN:-/opt/homebrew/bin/go}"
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client_v2/rust/target/debug/client-core-service}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-http://10.0.2.2:28080}"
MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST:-127.0.0.1:$((46380 + RANDOM % 200))}"
USE_EXISTING_MAC_SERVICE="${SLAN_USE_EXISTING_MAC_SERVICE:-0}"
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
EMAIL="${SLAN_TEST_EMAIL:-mac-android-socket-$(date +%s%N)@example.test}"
TIMEOUT="${SLAN_MAC_ANDROID_SOCKET_TIMEOUT:-90s}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
UDP_BODY="${SLAN_TEST_UDP_SEND_BODY:-hello-android-to-mac-udp-$(date +%s%N)}"
TCP_BODY="${SLAN_TEST_TCP_SEND_BODY:-hello-android-to-mac-tcp-$(date +%s%N)}"
WORK_DIR="${SLAN_MAC_ANDROID_SOCKET_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-android-socket.XXXXXX")}"
ECHO_LOG="$WORK_DIR/macos-echo.log"
ANDROID_LOG="$WORK_DIR/android-socket.log"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"

PIDS=()

cleanup() {
  status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$ECHO_LOG" ]] && { echo "---- Mac echo log ----" >&2; cat "$ECHO_LOG" >&2; }
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android socket log ----" >&2; cat "$ANDROID_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
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
if [[ "$USE_EXISTING_MAC_SERVICE" != "1" && ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd client_v2/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ $ADB wait-for-device"
"$ADB" wait-for-device

if [[ "$USE_EXISTING_MAC_SERVICE" != "1" ]]; then
  mkdir -p "$WORK_DIR/state"

  echo "+ start mac client-core-service on $MAC_SERVICE_HOST"
  SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST" \
    SLAN_CONTROL_BASE_URL="$BIZ_URL" \
    SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}" \
    SLAN_STATE_DIR="$WORK_DIR/state" \
    "$SERVICE_BIN" >"$MAC_SERVICE_LOG" 2>&1 &
  PIDS+=("$!")
else
  echo "+ use existing mac client-core-service at $MAC_SERVICE_HOST"
fi

echo "+ login and enable Mac service network at $MAC_SERVICE_HOST"
if ! MAC_OUTPUT="$(
  cd "$ROOT_DIR"
  "$GO_BIN" run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$MAC_SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register=true \
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
MAC_IP="$(echo "$MAC_OUTPUT" | sed -n 's/.*clientCoreServiceNetwork: enabled virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
if [[ -z "$MAC_IP" ]]; then
  echo "failed to parse Mac virtual IP from login output" >&2
  exit 1
fi
MAC_IP="${MAC_IP%%/*}"
echo "Mac network IP: $MAC_IP"

echo "+ start Mac UDP/TCP echo server"
(
  cd "$ROOT_DIR"
  "$GO_BIN" run scripts/socket_echo_server.go \
    -udp-port "$UDP_PORT" \
    -tcp-port "$TCP_PORT"
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
(
  cd "$APP_DIR"
  flutter build apk --debug >/dev/null
)
"$ADB" install -r "$APP_DIR/build/app/outputs/flutter-apk/app-debug.apk" >/dev/null
"$ADB" shell pm clear dev.slan.slan_client_v2 >/dev/null
"$ADB" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow
"$ADB" shell cmd appops get dev.slan.slan_client_v2 ACTIVATE_VPN

echo "+ run Android UDP/TCP sender target=$MAC_IP udp=$UDP_PORT tcp=$TCP_PORT"
(
  cd "$APP_DIR"
  flutter test integration_test/mobile_login_test.dart \
    -d "$ANDROID_DEVICE" \
    --dart-define="SLAN_TEST_BIZ_URL=$ANDROID_BIZ_URL" \
    --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$ANDROID_BIZ_URL" \
    --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
    --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
    --dart-define="SLAN_TEST_REGISTER_USER=false" \
    --dart-define="SLAN_TEST_WAIT_MQTT=true" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-8}" \
    --dart-define="SLAN_TEST_HOLD_SECONDS=${SLAN_ANDROID_TEST_HOLD_SECONDS:-0}" \
    --dart-define="SLAN_TEST_UDP_SEND_TARGET=$MAC_IP:$UDP_PORT" \
    --dart-define="SLAN_TEST_UDP_SEND_BODY=$UDP_BODY" \
    --dart-define="SLAN_TEST_TCP_SEND_TARGET=$MAC_IP:$TCP_PORT" \
    --dart-define="SLAN_TEST_TCP_SEND_BODY=$TCP_BODY"
) >"$ANDROID_LOG" 2>&1
cat "$ANDROID_LOG"

if ! grep -q "SLAN_TEST_UDP_ECHO_OK=$MAC_IP:$UDP_PORT" "$ANDROID_LOG"; then
  echo "Android UDP echo check did not complete" >&2
  exit 1
fi
if ! grep -q "SLAN_TEST_TCP_ECHO_OK=$MAC_IP:$TCP_PORT" "$ANDROID_LOG"; then
  echo "Android TCP echo check did not complete" >&2
  exit 1
fi
if ! grep -q "SOCKET_ECHO_UDP_RECEIVED=" "$ECHO_LOG"; then
  echo "Mac UDP echo server did not receive data" >&2
  exit 1
fi
if ! grep -q "SOCKET_ECHO_TCP_RECEIVED=" "$ECHO_LOG"; then
  echo "Mac TCP echo server did not receive data" >&2
  exit 1
fi
cat "$ECHO_LOG"

echo "macAndroidSocketCheck: ok email=$EMAIL macIp=$MAC_IP udp=$UDP_PORT tcp=$TCP_PORT"
