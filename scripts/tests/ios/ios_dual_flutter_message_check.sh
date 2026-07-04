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
BUNDLE_ID="${SLAN_IOS_BUNDLE_ID:-dev.slan.client.v2}"
SIM_A_NAME="${SLAN_IOS_SIM_A_NAME:-iPhone 17 Pro}"
SIM_B_NAME="${SLAN_IOS_SIM_B_NAME:-SLAN iPhone 16 Pro Clean 26.5}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
GENERATED_TEST_EMAIL=0
if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
else
  EMAIL="ios-dual-flutter-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"
WORK_DIR="${SLAN_IOS_DUAL_FLUTTER_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-ios-dual-flutter.XXXXXX")}"
LOG_A_PHASE1="$WORK_DIR/ios-a-phase1.log"
LOG_B_PHASE1="$WORK_DIR/ios-b-phase1.log"
LOG_A_WAIT="$WORK_DIR/ios-a-wait.log"
LOG_B_WAIT="$WORK_DIR/ios-b-wait.log"
LOG_A_SEND="$WORK_DIR/ios-a-send.log"
LOG_B_SEND="$WORK_DIR/ios-b-send.log"
MESSAGE_A_TO_B="${SLAN_IOS_MESSAGE_A_TO_B:-ios-a-to-b-$(date +%s%N)}"
MESSAGE_B_TO_A="${SLAN_IOS_MESSAGE_B_TO_A:-ios-b-to-a-$(date +%s%N)}"
WAIT_BEFORE_SEND_SECONDS="${SLAN_IOS_WAIT_BEFORE_SEND_SECONDS:-8}"
DEVICE_ID_WAIT_SECONDS="${SLAN_IOS_DEVICE_ID_WAIT_SECONDS:-180}"
REQUESTED_DEVICE_ID_A="${SLAN_IOS_REQUESTED_DEVICE_ID_A:-$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-')}"
REQUESTED_DEVICE_ID_B="${SLAN_IOS_REQUESTED_DEVICE_ID_B:-$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-')}"

PIDS=()

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    local log_file
    for log_file in \
      "$LOG_A_PHASE1" "$LOG_B_PHASE1" \
      "$LOG_A_WAIT" "$LOG_B_WAIT" \
      "$LOG_A_SEND" "$LOG_B_SEND"; do
      if [[ -f "$log_file" ]]; then
        echo "---- $(basename "$log_file") ----" >&2
        cat "$log_file" >&2
      fi
    done
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
  if [[ "${SLAN_KEEP_IOS_DUAL_FLUTTER_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

extract_log_value() {
  local log_file="$1"
  local marker="$2"
  sed -n "s/.*${marker}=\\([^[:space:]]*\\).*/\\1/p" "$log_file" | tail -n 1
}

wait_for_log_value() {
  local pid="$1"
  local log_file="$2"
  local marker="$3"
  local timeout_seconds="$4"
  local value=""
  for _ in $(seq 1 "$timeout_seconds"); do
    if ! kill -0 "$pid" 2>/dev/null; then
      cat "$log_file" >&2
      echo "process exited before marker ${marker}: $log_file" >&2
      exit 1
    fi
    value="$(extract_log_value "$log_file" "$marker")"
    [[ -n "$value" ]] && break
    sleep 1
  done
  if [[ -z "$value" ]]; then
    cat "$log_file" >&2
    echo "timed out waiting for marker ${marker}: $log_file" >&2
    exit 1
  fi
  printf '%s\n' "$value"
}

run_flutter_test_bg() {
  local device="$1"
  local log_file="$2"
  local build_dir="$3"
  shift 3
  (
    cd "$APP_DIR"
    FLUTTER_BUILD_DIR="$build_dir" flutter test integration_test/mobile_login_test.dart \
      -d "$device" \
      "$@"
  ) >"$log_file" 2>&1 &
  RUN_FLUTTER_BG_PID="$!"
  PIDS+=("$RUN_FLUTTER_BG_PID")
}

run_flutter_test_fg() {
  local device="$1"
  local log_file="$2"
  local build_dir="$3"
  shift 3
  (
    cd "$APP_DIR"
    FLUTTER_BUILD_DIR="$build_dir" flutter test integration_test/mobile_login_test.dart \
      -d "$device" \
      "$@"
  ) >"$log_file" 2>&1
}

SIM_A_DEVICE="${SLAN_IOS_SIM_A_DEVICE:-$SIM_A_NAME}"
SIM_B_DEVICE="${SLAN_IOS_SIM_B_DEVICE:-$SIM_B_NAME}"

echo "==> dual iOS flutter message check"
echo "==> device A: $SIM_A_DEVICE"
echo "==> device B: $SIM_B_DEVICE"
echo "==> email: $EMAIL"
echo "==> requested ios-a device id: $REQUESTED_DEVICE_ID_A"
echo "==> requested ios-b device id: $REQUESTED_DEVICE_ID_B"

xcrun simctl uninstall "$SIM_A_DEVICE" "$BUNDLE_ID" >/dev/null 2>&1 || true
xcrun simctl uninstall "$SIM_B_DEVICE" "$BUNDLE_ID" >/dev/null 2>&1 || true

COMMON_DART_DEFINES=(
  --dart-define="SLAN_TEST_BIZ_URL=$BIZ_URL"
  --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$BIZ_URL"
  --dart-define="SLAN_TEST_EMAIL=$EMAIL"
  --dart-define="SLAN_TEST_PASSWORD=$PASSWORD"
  --dart-define="SLAN_TEST_WAIT_MQTT=true"
)

echo "==> phase 1: login ios-a and capture device id"
run_flutter_test_fg \
  "$SIM_A_DEVICE" \
  "$LOG_A_PHASE1" \
  "$WORK_DIR/build-a-phase1" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_A" \
  --dart-define="SLAN_TEST_REGISTER_USER=true"
DEVICE_ID_A="$(extract_log_value "$LOG_A_PHASE1" "SLAN_TEST_CLIENT_DEVICE_ID")"
[[ -n "$DEVICE_ID_A" ]] || {
  cat "$LOG_A_PHASE1" >&2
  echo "failed to capture ios-a device id" >&2
  exit 1
}
echo "ios-a device id: $DEVICE_ID_A"

echo "==> phase 1: login ios-b and capture device id"
run_flutter_test_fg \
  "$SIM_B_DEVICE" \
  "$LOG_B_PHASE1" \
  "$WORK_DIR/build-b-phase1" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_B" \
  --dart-define="SLAN_TEST_REGISTER_USER=false"
DEVICE_ID_B="$(extract_log_value "$LOG_B_PHASE1" "SLAN_TEST_CLIENT_DEVICE_ID")"
[[ -n "$DEVICE_ID_B" ]] || {
  cat "$LOG_B_PHASE1" >&2
  echo "failed to capture ios-b device id" >&2
  exit 1
}
echo "ios-b device id: $DEVICE_ID_B"

echo "==> phase 2: ios-b sends message to ios-a"
run_flutter_test_bg \
  "$SIM_A_DEVICE" \
  "$LOG_A_WAIT" \
  "$WORK_DIR/build-a-wait" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_A" \
  --dart-define="SLAN_TEST_REGISTER_USER=false" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID=$DEVICE_ID_B" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MESSAGE_B_TO_A"
WAIT_PID_A="$RUN_FLUTTER_BG_PID"
sleep "$WAIT_BEFORE_SEND_SECONDS"
run_flutter_test_fg \
  "$SIM_B_DEVICE" \
  "$LOG_B_SEND" \
  "$WORK_DIR/build-b-send" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_B" \
  --dart-define="SLAN_TEST_REGISTER_USER=false" \
  --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$DEVICE_ID_A" \
  --dart-define="SLAN_TEST_SEND_BODY=$MESSAGE_B_TO_A"
wait "$WAIT_PID_A"

echo "==> phase 3: ios-a sends message to ios-b"
run_flutter_test_bg \
  "$SIM_B_DEVICE" \
  "$LOG_B_WAIT" \
  "$WORK_DIR/build-b-wait" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_B" \
  --dart-define="SLAN_TEST_REGISTER_USER=false" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID=$DEVICE_ID_A" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MESSAGE_A_TO_B"
WAIT_PID_B="$RUN_FLUTTER_BG_PID"
sleep "$WAIT_BEFORE_SEND_SECONDS"
run_flutter_test_fg \
  "$SIM_A_DEVICE" \
  "$LOG_A_SEND" \
  "$WORK_DIR/build-a-send" \
  "${COMMON_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_A" \
  --dart-define="SLAN_TEST_REGISTER_USER=false" \
  --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$DEVICE_ID_B" \
  --dart-define="SLAN_TEST_SEND_BODY=$MESSAGE_A_TO_B"
wait "$WAIT_PID_B"

echo "iosDualFlutterMessageCheck: ok email=$EMAIL ios-a=$DEVICE_ID_A ios-b=$DEVICE_ID_B"
