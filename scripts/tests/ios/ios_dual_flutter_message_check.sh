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
BUNDLE_ID="${SLAN_IOS_BUNDLE_ID:-dev.slan.client.v2}"
SIM_A_NAME="${SLAN_IOS_SIM_A_NAME:-iPhone 17 Pro}"
SIM_B_NAME="${SLAN_IOS_SIM_B_NAME:-SLAN iPhone 16 Pro Clean 26.5}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
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
NETWORK_ID=""
DEVICE_GROUP_ID=""
OPS_TOKEN=""
CREDENTIAL_ID_A=""
AUTHORIZATION_KEY_A=""
CREDENTIAL_ID_B=""
AUTHORIZATION_KEY_B=""

read_lines_into_array() {
  local __target_var="$1"
  local __line
  local -a __values=()
  while IFS= read -r __line; do
    __values+=("$__line")
  done
  eval "$__target_var=()"
  local __value
  for __value in "${__values[@]}"; do
    eval "$__target_var+=(\"\$__value\")"
  done
}

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
  if [[ -n "$DEVICE_GROUP_ID" && -n "$OPS_TOKEN" ]]; then
    curl --silent --show-error --connect-timeout 5 --max-time 20 \
      -X DELETE "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups/${DEVICE_GROUP_ID}" \
      -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null 2>&1 || true
    curl --silent --show-error --connect-timeout 5 --max-time 20 \
      -X DELETE "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}" \
      -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null 2>&1 || true
  fi
  slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$CREDENTIAL_ID_A" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$CREDENTIAL_ID_B" >/dev/null 2>&1 || true
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
    FLUTTER_BUILD_DIR="$build_dir" flutter test integration_test/device_activation_harness_test.dart \
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
    FLUTTER_BUILD_DIR="$build_dir" flutter test integration_test/device_activation_harness_test.dart \
      -d "$device" \
      "$@"
  ) >"$log_file" 2>&1
}

capture_device_id_or_die() {
  local log_file="$1"
  local label="$2"
  local device_id
  device_id="$(extract_log_value "$log_file" "SLAN_TEST_CLIENT_DEVICE_ID")"
  [[ -n "$device_id" ]] || {
    cat "$log_file" >&2
    echo "failed to capture ${label} device id" >&2
    exit 1
  }
  printf '%s\n' "$device_id"
}

ios_activation_defines() {
  local requested_device_id="$1"
  if [[ "$requested_device_id" == "$REQUESTED_DEVICE_ID_A" ]]; then
    slan_mobile_activation_common_defines "$BIZ_URL" "$AUTHORIZATION_KEY_A" true
  else
    slan_mobile_activation_common_defines "$BIZ_URL" "$AUTHORIZATION_KEY_B" true
  fi
}

run_ios_activation_capture() {
  local device="$1"
  local log_file="$2"
  local build_dir="$3"
  local requested_device_id="$4"
  local common_defines=()
  read_lines_into_array common_defines < <(ios_activation_defines "$requested_device_id")
  run_flutter_test_fg \
    "$device" \
    "$log_file" \
    "$build_dir" \
    "${common_defines[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$requested_device_id" \
    --dart-define="SLAN_TEST_WAIT_MQTT=false"
}

provision_ops_resources() {
  local credential network response device_id
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Dual iOS A" "$REQUESTED_DEVICE_ID_A")"
  CREDENTIAL_ID_A="$(printf '%s' "$credential" | jq -er '.credentialId')"
  AUTHORIZATION_KEY_A="$(printf '%s' "$credential" | jq -er '.key')"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Dual iOS B" "$REQUESTED_DEVICE_ID_B")"
  CREDENTIAL_ID_B="$(printf '%s' "$credential" | jq -er '.credentialId')"
  AUTHORIZATION_KEY_B="$(printf '%s' "$credential" | jq -er '.key')"
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" -H 'Content-Type: application/json' \
    -d "{\"key\":\"$AUTHORIZATION_KEY_A\",\"deviceId\":\"$REQUESTED_DEVICE_ID_A\"}" >/dev/null
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" -H 'Content-Type: application/json' \
    -d "{\"key\":\"$AUTHORIZATION_KEY_B\",\"deviceId\":\"$REQUESTED_DEVICE_ID_B\"}" >/dev/null
  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "ios-dual-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network" | jq -er '.networkId')"
  response="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "${OPS_BASE_URL}/api/ops/device-groups" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"name\":\"ios-flutter-$(date +%s%N)\",\"description\":\"Dual iOS Flutter devices\"}")"
  DEVICE_GROUP_ID="$(printf '%s' "$response" | sed -n 's/.*"groupId":"\([^"]*\)".*/\1/p')"
  [[ -n "$DEVICE_GROUP_ID" ]] || { echo "failed to create iOS test device group" >&2; exit 1; }
  for device_id in "$REQUESTED_DEVICE_ID_A" "$REQUESTED_DEVICE_ID_B"; do
    curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
      -X POST "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}/devices" \
      -H "Authorization: Bearer ${OPS_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"deviceId\":\"${device_id}\"}" >/dev/null
  done
  curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"groupId\":\"${DEVICE_GROUP_ID}\"}" >/dev/null
  echo "==> attached iOS device group ${DEVICE_GROUP_ID} to network ${NETWORK_ID}"
}

start_ios_message_wait() {
  local device="$1"
  local log_file="$2"
  local build_dir="$3"
  local requested_device_id="$4"
  local expect_from_device_id="$5"
  local expect_body="$6"
  local common_defines=()
  local expect_defines=()
  read_lines_into_array common_defines < <(ios_activation_defines "$requested_device_id")
  read_lines_into_array expect_defines < <(
    slan_mobile_message_expect_defines "$expect_from_device_id" "$expect_body"
  )
  run_flutter_test_bg \
    "$device" \
    "$log_file" \
    "$build_dir" \
    "${common_defines[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$requested_device_id" \
    "${expect_defines[@]}"
}

run_ios_message_send() {
  local device="$1"
  local log_file="$2"
  local build_dir="$3"
  local requested_device_id="$4"
  local target_device_id="$5"
  local body="$6"
  local common_defines=()
  local send_defines=()
  read_lines_into_array common_defines < <(ios_activation_defines "$requested_device_id")
  read_lines_into_array send_defines < <(
    slan_mobile_message_send_defines "$target_device_id" "$body"
  )
  run_flutter_test_fg \
    "$device" \
    "$log_file" \
    "$build_dir" \
    "${common_defines[@]}" \
    --dart-define="SLAN_TEST_DEVICE_ID=$requested_device_id" \
    "${send_defines[@]}"
}

SIM_A_DEVICE="${SLAN_IOS_SIM_A_DEVICE:-$SIM_A_NAME}"
SIM_B_DEVICE="${SLAN_IOS_SIM_B_DEVICE:-$SIM_B_NAME}"

echo "==> dual iOS flutter message check"
echo "==> device A: $SIM_A_DEVICE"
echo "==> device B: $SIM_B_DEVICE"
echo "==> requested ios-a device id: $REQUESTED_DEVICE_ID_A"
echo "==> requested ios-b device id: $REQUESTED_DEVICE_ID_B"

xcrun simctl uninstall "$SIM_A_DEVICE" "$BUNDLE_ID" >/dev/null 2>&1 || true
xcrun simctl uninstall "$SIM_B_DEVICE" "$BUNDLE_ID" >/dev/null 2>&1 || true

provision_ops_resources

echo "==> phase 1: activate ios-a and capture device id"
run_ios_activation_capture \
  "$SIM_A_DEVICE" \
  "$LOG_A_PHASE1" \
  "$WORK_DIR/build-a-phase1" \
  "$REQUESTED_DEVICE_ID_A"
DEVICE_ID_A="$(capture_device_id_or_die "$LOG_A_PHASE1" "ios-a")"
[[ "$DEVICE_ID_A" == "$REQUESTED_DEVICE_ID_A" ]] || {
  echo "ios-a device id mismatch: expected=$REQUESTED_DEVICE_ID_A actual=$DEVICE_ID_A" >&2
  exit 1
}
echo "ios-a device id: $DEVICE_ID_A"

echo "==> phase 1: activate ios-b and capture device id"
run_ios_activation_capture \
  "$SIM_B_DEVICE" \
  "$LOG_B_PHASE1" \
  "$WORK_DIR/build-b-phase1" \
  "$REQUESTED_DEVICE_ID_B"
DEVICE_ID_B="$(capture_device_id_or_die "$LOG_B_PHASE1" "ios-b")"
[[ "$DEVICE_ID_B" == "$REQUESTED_DEVICE_ID_B" ]] || {
  echo "ios-b device id mismatch: expected=$REQUESTED_DEVICE_ID_B actual=$DEVICE_ID_B" >&2
  exit 1
}
echo "ios-b device id: $DEVICE_ID_B"

echo "==> phase 2: ios-b sends message to ios-a"
start_ios_message_wait \
  "$SIM_A_DEVICE" \
  "$LOG_A_WAIT" \
  "$WORK_DIR/build-a-wait" \
  "$REQUESTED_DEVICE_ID_A" \
  "$DEVICE_ID_B" \
  "$MESSAGE_B_TO_A"
WAIT_PID_A="$RUN_FLUTTER_BG_PID"
sleep "$WAIT_BEFORE_SEND_SECONDS"
run_ios_message_send \
  "$SIM_B_DEVICE" \
  "$LOG_B_SEND" \
  "$WORK_DIR/build-b-send" \
  "$REQUESTED_DEVICE_ID_B" \
  "$DEVICE_ID_A" \
  "$MESSAGE_B_TO_A"
wait "$WAIT_PID_A"

echo "==> phase 3: ios-a sends message to ios-b"
start_ios_message_wait \
  "$SIM_B_DEVICE" \
  "$LOG_B_WAIT" \
  "$WORK_DIR/build-b-wait" \
  "$REQUESTED_DEVICE_ID_B" \
  "$DEVICE_ID_A" \
  "$MESSAGE_A_TO_B"
WAIT_PID_B="$RUN_FLUTTER_BG_PID"
sleep "$WAIT_BEFORE_SEND_SECONDS"
run_ios_message_send \
  "$SIM_A_DEVICE" \
  "$LOG_A_SEND" \
  "$WORK_DIR/build-a-send" \
  "$REQUESTED_DEVICE_ID_A" \
  "$DEVICE_ID_B" \
  "$MESSAGE_A_TO_B"
wait "$WAIT_PID_B"

echo "iosDualFlutterMessageCheck: ok ios-a=$DEVICE_ID_A ios-b=$DEVICE_ID_B"
