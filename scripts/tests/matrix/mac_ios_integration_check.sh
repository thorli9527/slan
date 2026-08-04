#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/lib/flutter_mobile_activation_test.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"
APP_DIR="$ROOT_DIR/client/app_flutter"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
SERVICE_HOST="${SLAN_MAC_IOS_SERVICE_HOST:-127.0.0.1:46395}"
RUN_IOS_APP_DNS_ACL_SMOKE="${SLAN_RUN_IOS_APP_DNS_ACL_SMOKE:-1}"
IOS_APP_DNS_ACL_CHECK_MESSAGES="${SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES:-1}"
IOS_APP_DNS_ACL_CLIENTS="${SLAN_IOS_APP_DNS_ACL_CLIENTS:-2}"
TIMEOUT="${SLAN_MAC_IOS_TIMEOUT:-60s}"
WORK_DIR="${SLAN_MAC_IOS_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-ios.XXXXXX")}"
RUN_MAC_LOCAL_DNS_SMOKE="${SLAN_RUN_MAC_LOCAL_DNS_SMOKE:-1}"
MAC_LOG="$WORK_DIR/macos-service.log"
IOS_LOG="$WORK_DIR/ios-flutter-test.log"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
IOS_TEST_DEVICE_ID="${SLAN_IOS_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
IOS_TO_MAC_BODY="${SLAN_IOS_TO_MAC_BODY:-hello-ios-to-mac-$(date +%s%N)}"
MAC_TO_IOS_BODY="${SLAN_MAC_TO_IOS_BODY:-hello-mac-to-ios-$(date +%s%N)}"

PIDS=()
TEST_NETWORK_ID=""
TEST_DEVICE_GROUP_ID=""
OPS_TOKEN=""
MAC_CREDENTIAL_ID=""
MAC_AUTHORIZATION_KEY=""
IOS_CREDENTIAL_ID=""
IOS_AUTHORIZATION_KEY=""

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
  status=$?
  if [[ $status -ne 0 && -f "$IOS_LOG" ]]; then
    echo "---- iOS Flutter log ----" >&2
    cat "$IOS_LOG" >&2
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  cleanup_ops_resources
  if [[ "${SLAN_KEEP_MAC_IOS_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

provision_ops_resources() {
  local credential group network device_id
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac iOS Integration Mac" "$MAC_TEST_DEVICE_ID")"
  MAC_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  MAC_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"
  credential="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Mac iOS Integration iOS" "$IOS_TEST_DEVICE_ID")"
  IOS_CREDENTIAL_ID="$(printf '%s' "$credential" | jq -er '.credentialId')"
  IOS_AUTHORIZATION_KEY="$(printf '%s' "$credential" | jq -er '.key')"

  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"$MAC_AUTHORIZATION_KEY\",\"deviceId\":\"$MAC_TEST_DEVICE_ID\"}" >/dev/null
  curl --silent --show-error --fail -X POST "$BIZ_URL/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"$IOS_AUTHORIZATION_KEY\",\"deviceId\":\"$IOS_TEST_DEVICE_ID\"}" >/dev/null

  network="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "mac-ios-$(date +%s%N)")"
  TEST_NETWORK_ID="$(printf '%s' "$network" | jq -er '.networkId')"
  group="$(curl --silent --show-error --fail \
    -X POST "$OPS_BASE_URL/api/ops/device-groups" \
    -H "Authorization: Bearer $OPS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"name\":\"mac-ios-$(date +%s%N)\",\"description\":\"Mac iOS integration devices\"}")"
  TEST_DEVICE_GROUP_ID="$(printf '%s' "$group" | jq -er '.groupId')"

  for device_id in "$MAC_TEST_DEVICE_ID" "$IOS_TEST_DEVICE_ID"; do
    curl --silent --show-error --fail \
      -X POST "$OPS_BASE_URL/api/ops/device-groups/$TEST_DEVICE_GROUP_ID/devices" \
      -H "Authorization: Bearer $OPS_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "{\"deviceId\":\"$device_id\"}" >/dev/null
  done
  curl --silent --show-error --fail \
    -X POST "$OPS_BASE_URL/api/ops/networks/$TEST_NETWORK_ID/device-groups" \
    -H "Authorization: Bearer $OPS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"groupId\":\"$TEST_DEVICE_GROUP_ID\"}" >/dev/null
  echo "+ provisioned Ops network membership: network=$TEST_NETWORK_ID group=$TEST_DEVICE_GROUP_ID"
}

cleanup_ops_resources() {
  if [[ -n "$TEST_NETWORK_ID" && -n "$TEST_DEVICE_GROUP_ID" && -n "$OPS_TOKEN" ]]; then
    curl --silent --show-error -X DELETE \
      "$OPS_BASE_URL/api/ops/networks/$TEST_NETWORK_ID/device-groups/$TEST_DEVICE_GROUP_ID" \
      -H "Authorization: Bearer $OPS_TOKEN" >/dev/null 2>&1 || true
  fi
  if [[ -n "$TEST_DEVICE_GROUP_ID" && -n "$OPS_TOKEN" ]]; then
    curl --silent --show-error -X DELETE "$OPS_BASE_URL/api/ops/device-groups/$TEST_DEVICE_GROUP_ID" \
      -H "Authorization: Bearer $OPS_TOKEN" >/dev/null 2>&1 || true
  fi
  slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$TEST_NETWORK_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$MAC_CREDENTIAL_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$IOS_CREDENTIAL_ID" >/dev/null 2>&1 || true
}

run_mac_local_api_check() {
  (
    cd "$ROOT_DIR"
    go run scripts/client_core_service_login_check.go "$@"
  )
}

start_mac_service() {
  SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
    SLAN_CONTROL_BASE_URL="$BIZ_URL" \
    SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
    SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-1}" \
    SLAN_STATE_DIR="$WORK_DIR/state" \
    "$SERVICE_BIN" >"$MAC_LOG" 2>&1 &
  MAC_SERVICE_PID="$!"
  PIDS+=("$MAC_SERVICE_PID")
}

start_ios_flutter_message_harness() {
  local ios_common_dart_defines=()
  local ios_send_dart_defines=()
  local ios_expect_dart_defines=()
  read_lines_into_array ios_common_dart_defines < <(
    slan_mobile_activation_common_defines "$BIZ_URL" "$IOS_AUTHORIZATION_KEY" true
  )
  read_lines_into_array ios_send_dart_defines < <(
    slan_mobile_message_send_defines "$MAC_DEVICE_ID" "$IOS_TO_MAC_BODY"
  )
  read_lines_into_array ios_expect_dart_defines < <(
    slan_mobile_message_expect_defines "$MAC_DEVICE_ID" "$MAC_TO_IOS_BODY"
  )
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$IOS_DEVICE" \
      "${ios_common_dart_defines[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$IOS_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=${SLAN_TEST_CHECK_SWITCH:-false}" \
      --dart-define="SLAN_TEST_MQTT_READY_TIMEOUT_SECONDS=${SLAN_TEST_MQTT_READY_TIMEOUT_SECONDS:-120}" \
      --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=${SLAN_TEST_EXPECT_NETWORK_MODULE:-false}" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=${SLAN_TEST_MIN_NETWORK_MODULE_PEERS:-0}" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=${SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES:-0}" \
      "${ios_send_dart_defines[@]}" \
      "${ios_expect_dart_defines[@]}"
  ) >"$IOS_LOG" 2>&1 &
  IOS_PID="$!"
  PIDS+=("$IOS_PID")
}

capture_ios_device_id_or_die() {
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
  if [[ "$IOS_DEVICE_ID" != "$IOS_TEST_DEVICE_ID" ]]; then
    echo "iOS device id mismatch: expected=$IOS_TEST_DEVICE_ID actual=$IOS_DEVICE_ID" >&2
    exit 1
  fi
  echo "iOS device id: $IOS_DEVICE_ID"
}

if [[ ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd client/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ mac ios integration defaults: biz=$BIZ_URL ops=$OPS_BASE_URL service_host=$SERVICE_HOST"

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
provision_ops_resources

echo "+ start mac client-core-service on $SERVICE_HOST"
start_mac_service

echo "+ activate mac client-core-service"
MAC_OUTPUT="$(
  run_mac_local_api_check \
    -biz-url "$BIZ_URL" \
    -address "$SERVICE_HOST" \
    -authorization-key "$MAC_AUTHORIZATION_KEY" \
    -enable-network=true \
    -timeout "$TIMEOUT"
)"
echo "$MAC_OUTPUT"
if [[ "$RUN_MAC_LOCAL_DNS_SMOKE" == "1" ]]; then
  echo "+ run mac local dns smoke on $SERVICE_HOST"
  (
    cd "$ROOT_DIR"
    SLAN_CLIENT_CORE_SERVICE_HOST="${SERVICE_HOST%:*}" \
      SLAN_CLIENT_CORE_SERVICE_PORT="${SERVICE_HOST##*:}" \
      bash scripts/tests/shared/desktop_local_dns_smoke.sh
  )
fi
MAC_DEVICE_ID="$(echo "$MAC_OUTPUT" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
if [[ -z "$MAC_DEVICE_ID" ]]; then
  echo "failed to parse mac device id from login output" >&2
  exit 1
fi
if [[ "$MAC_DEVICE_ID" != "$MAC_TEST_DEVICE_ID" ]]; then
  echo "Mac device id mismatch: expected=$MAC_TEST_DEVICE_ID actual=$MAC_DEVICE_ID" >&2
  exit 1
fi

if [[ "$RUN_IOS_APP_DNS_ACL_SMOKE" == "1" ]]; then
  echo "+ run iOS app dns/acl smoke"
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$BIZ_URL" \
      SLAN_CLIENT_CORE_SERVICE_BIN="$SERVICE_BIN" \
      SLAN_EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-}" \
      SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES="$IOS_APP_DNS_ACL_CHECK_MESSAGES" \
      SLAN_IOS_APP_DNS_ACL_CLIENTS="$IOS_APP_DNS_ACL_CLIENTS" \
      bash scripts/ios_app_dns_acl_smoke.sh
  )
fi

echo "+ start iOS flutter message harness"
start_ios_flutter_message_harness
capture_ios_device_id_or_die

echo "+ wait mac receive iOS message"
run_mac_local_api_check \
  -biz-url "$BIZ_URL" \
  -address "$SERVICE_HOST" \
  -activate=false \
  -expect-from "$IOS_DEVICE_ID" \
  -expect-body "$IOS_TO_MAC_BODY" \
  -timeout "$TIMEOUT"

echo "+ send mac message to iOS"
run_mac_local_api_check \
  -biz-url "$BIZ_URL" \
  -address "$SERVICE_HOST" \
  -activate=false \
  -send-target "$IOS_DEVICE_ID" \
  -send-body "$MAC_TO_IOS_BODY" \
  -timeout "$TIMEOUT"

echo "+ wait iOS integration test"
if ! wait "$IOS_PID"; then
  cat "$IOS_LOG"
  echo "iOS integration test failed" >&2
  exit 1
fi
cat "$IOS_LOG"

echo "macIosIntegrationCheck: ok mac=$MAC_DEVICE_ID ios=$IOS_DEVICE_ID"
