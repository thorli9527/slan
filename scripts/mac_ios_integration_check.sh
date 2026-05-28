#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
APP_DIR="$ROOT_DIR/client_v2/app_flutter"
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client_v2/rust/target/debug/client-core-service}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
SERVICE_HOST="${SLAN_MAC_IOS_SERVICE_HOST:-127.0.0.1:46395}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
GENERATED_TEST_EMAIL=0
if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
else
  EMAIL="mac-ios-integration-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"
TIMEOUT="${SLAN_MAC_IOS_TIMEOUT:-60s}"
WORK_DIR="${SLAN_MAC_IOS_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-ios.XXXXXX")}"
MAC_LOG="$WORK_DIR/macos-service.log"
IOS_LOG="$WORK_DIR/ios-flutter-test.log"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
IOS_TEST_DEVICE_ID="${SLAN_IOS_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
IOS_TO_MAC_BODY="${SLAN_IOS_TO_MAC_BODY:-hello-ios-to-mac-$(date +%s%N)}"
MAC_TO_IOS_BODY="${SLAN_MAC_TO_IOS_BODY:-hello-mac-to-ios-$(date +%s%N)}"

PIDS=()

cleanup() {
  status=$?
  if [[ $status -ne 0 && -f "$IOS_LOG" ]]; then
    echo "---- iOS Flutter log ----" >&2
    cat "$IOS_LOG" >&2
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
  if [[ "${SLAN_KEEP_MAC_IOS_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [[ ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd client_v2/rust && cargo build -p client-core-service" >&2
  exit 1
fi

booted_ios_device() {
  xcrun simctl list devices booted |
    awk -F'[()]' '/Booted/ && /iPhone|iPad/ { print $2; exit }'
}

IOS_DEVICE="${SLAN_IOS_FLUTTER_DEVICE:-$(booted_ios_device)}"
if [[ -z "$IOS_DEVICE" ]]; then
  echo "no booted iOS simulator found; boot one with xcrun simctl boot or set SLAN_IOS_FLUTTER_DEVICE" >&2
  exit 1
fi

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

echo "+ flutter test iOS login and message send/wait"
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
    --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID" \
    --dart-define="SLAN_TEST_CHECK_SWITCH=${SLAN_TEST_CHECK_SWITCH:-false}" \
    --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$MAC_DEVICE_ID" \
    --dart-define="SLAN_TEST_SEND_BODY=$IOS_TO_MAC_BODY" \
    --dart-define="SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID=$MAC_DEVICE_ID" \
    --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MAC_TO_IOS_BODY"
) >"$IOS_LOG" 2>&1 &
IOS_PID="$!"
PIDS+=("$IOS_PID")

IOS_DEVICE_ID=""
for _ in $(seq 1 90); do
  if ! kill -0 "$IOS_PID" 2>/dev/null; then
    cat "$IOS_LOG"
    echo "iOS integration test exited before device id was reported" >&2
    exit 1
  fi
  IOS_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$IOS_LOG" | tail -n 1)"
  if [[ -n "$IOS_DEVICE_ID" ]]; then
    break
  fi
  sleep 1
done

if [[ -z "$IOS_DEVICE_ID" ]]; then
  cat "$IOS_LOG"
  echo "timed out waiting for iOS device id marker" >&2
  exit 1
fi
echo "iOS device id: $IOS_DEVICE_ID"

echo "+ wait mac receive iOS message"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -login=false \
    -expect-from "$IOS_DEVICE_ID" \
    -expect-body "$IOS_TO_MAC_BODY" \
    -timeout "$TIMEOUT"
)

echo "+ send mac message to iOS"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -login=false \
    -send-target "$IOS_DEVICE_ID" \
    -send-body "$MAC_TO_IOS_BODY" \
    -timeout "$TIMEOUT"
)

echo "+ wait iOS integration test"
if ! wait "$IOS_PID"; then
  cat "$IOS_LOG"
  echo "iOS integration test failed" >&2
  exit 1
fi
cat "$IOS_LOG"

echo "macIosIntegrationCheck: ok email=$EMAIL mac=$MAC_DEVICE_ID ios=$IOS_DEVICE_ID"
