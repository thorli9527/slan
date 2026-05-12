#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client_v2/rust/target/debug/client-core-service}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-http://10.0.2.2:28080}"
SERVICE_HOST="${SLAN_ANDROID_SERVICE_HOST:-127.0.0.1:46396}"
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
EMAIL="${SLAN_TEST_EMAIL:-mac-android-integration-$(date +%s%N)@example.test}"
TIMEOUT="${SLAN_ANDROID_TIMEOUT:-60s}"
WORK_DIR="${SLAN_ANDROID_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-android.XXXXXX")}"
MAC_LOG="$WORK_DIR/macos-service.log"
ANDROID_LOG="$WORK_DIR/android-flutter-test.log"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-mac-android-$(date +%s%N)}"
ANDROID_TEST_DEVICE_ID="${SLAN_ANDROID_TEST_DEVICE_ID:-android-integration-$(date +%s%N)}"
ANDROID_TO_MAC_BODY="${SLAN_ANDROID_TO_MAC_BODY:-hello-android-to-mac-$(date +%s%N)}"
MAC_TO_ANDROID_BODY="${SLAN_MAC_TO_ANDROID_BODY:-hello-mac-to-android-$(date +%s%N)}"

PIDS=()

start_android_vpn_appops_guard() {
  (
    while true; do
      "$ADB" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
      sleep 0.25
    done
  ) &
  PIDS+=("$!")
}

cleanup() {
  status=$?
  if [[ $status -ne 0 && -f "$ANDROID_LOG" ]]; then
    echo "---- Android Flutter log ----" >&2
    cat "$ANDROID_LOG" >&2
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  if [[ "${SLAN_KEEP_ANDROID_WORK_DIR:-0}" != "1" ]]; then
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

if [[ ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd client_v2/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ $ADB wait-for-device"
"$ADB" wait-for-device
boot_completed="$("$ADB" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
if [[ "$boot_completed" != "1" ]]; then
  echo "waiting for Android boot completion"
  for _ in $(seq 1 60); do
    boot_completed="$("$ADB" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
    [[ "$boot_completed" == "1" ]] && break
    sleep 1
  done
fi
if [[ "$boot_completed" != "1" ]]; then
  echo "Android device is connected but did not finish booting" >&2
  exit 1
fi
"$ADB" shell pm clear dev.slan.slan_client_v2 >/dev/null 2>&1 || true
"$ADB" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
start_android_vpn_appops_guard

mkdir -p "$WORK_DIR/state"

echo "+ start mac client-core-service on $SERVICE_HOST"
SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
  SLAN_CONTROL_BASE_URL="$BIZ_URL" \
  SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
  SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-1}" \
  SLAN_STATE_DIR="$WORK_DIR/state" \
  "$SERVICE_BIN" >"$MAC_LOG" 2>&1 &
PIDS+=("$!")

echo "+ login mac client-core-service"
MAC_OUTPUT="$(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
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
if [[ -z "$MAC_DEVICE_ID" ]]; then
  echo "failed to parse mac device id from login output" >&2
  exit 1
fi

echo "+ flutter test Android login and message send/wait"
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
    --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=${SLAN_TEST_CHECK_SWITCH:-false}" \
    --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=${SLAN_TEST_EXPECT_NETWORK_MODULE:-false}" \
    --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=${SLAN_TEST_MIN_NETWORK_MODULE_PEERS:-0}" \
    --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS=${SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS:-0}" \
    --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=${SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES:-0}" \
    --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=${SLAN_TEST_POST_ENABLE_WAIT_SECONDS:-0}" \
    --dart-define="SLAN_TEST_UDP_ECHO_PORT=${SLAN_TEST_UDP_ECHO_PORT:-0}" \
    --dart-define="SLAN_TEST_TCP_ECHO_PORT=${SLAN_TEST_TCP_ECHO_PORT:-0}" \
    --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$MAC_DEVICE_ID" \
    --dart-define="SLAN_TEST_SEND_BODY=$ANDROID_TO_MAC_BODY" \
    --dart-define="SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID=$MAC_DEVICE_ID" \
    --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MAC_TO_ANDROID_BODY"
) >"$ANDROID_LOG" 2>&1 &
ANDROID_PID="$!"
PIDS+=("$ANDROID_PID")

ANDROID_DEVICE_ID=""
for _ in $(seq 1 90); do
  if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
    cat "$ANDROID_LOG"
    echo "Android integration test exited before device id was reported" >&2
    exit 1
  fi
  ANDROID_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
  if [[ -n "$ANDROID_DEVICE_ID" ]]; then
    break
  fi
  sleep 1
done

if [[ -z "$ANDROID_DEVICE_ID" ]]; then
  cat "$ANDROID_LOG"
  echo "timed out waiting for Android device id marker" >&2
  exit 1
fi
echo "Android device id: $ANDROID_DEVICE_ID"

echo "+ wait mac receive Android message"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -login=false \
    -expect-from "$ANDROID_DEVICE_ID" \
    -expect-body "$ANDROID_TO_MAC_BODY" \
    -timeout "$TIMEOUT"
)

echo "+ send mac message to Android"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -login=false \
    -send-target "$ANDROID_DEVICE_ID" \
    -send-body "$MAC_TO_ANDROID_BODY" \
    -timeout "$TIMEOUT"
)

echo "+ wait Android integration test"
if ! wait "$ANDROID_PID"; then
  cat "$ANDROID_LOG"
  echo "Android integration test failed" >&2
  exit 1
fi
cat "$ANDROID_LOG"

echo "androidIntegrationCheck: ok email=$EMAIL mac=$MAC_DEVICE_ID android=$ANDROID_DEVICE_ID"
