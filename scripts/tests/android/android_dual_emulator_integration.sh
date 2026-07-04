#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"

APP_DIR="$ROOT_DIR/client_v2/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
FLUTTER_BIN="${SLAN_FLUTTER_BIN:-flutter}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
if [[ -n "${SLAN_ANDROID_BIZ_URL:-}" ]]; then
  ANDROID_BIZ_URL="$SLAN_ANDROID_BIZ_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  ANDROID_BIZ_URL="http://10.0.2.2:28080"
else
  ANDROID_BIZ_URL="$BIZ_URL"
fi
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
DEVICE_A="${SLAN_ANDROID_DEVICE_A:-emulator-5554}"
DEVICE_B="${SLAN_ANDROID_DEVICE_B:-emulator-5556}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
TIMEOUT="${SLAN_ANDROID_DUAL_TIMEOUT:-10m}"
WORK_DIR="${SLAN_ANDROID_DUAL_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-android-dual.XXXXXX")}"
EMAIL="${SLAN_TEST_EMAIL:-android-dual-$(date +%s%N)@example.test}"
REQUESTED_DEVICE_ID_A="${SLAN_ANDROID_DEVICE_ID_A:-11111111111141118111111111111111}"
REQUESTED_DEVICE_ID_B="${SLAN_ANDROID_DEVICE_ID_B:-22222222222242228222222222222222}"
DEVICE_ID_A=""
DEVICE_ID_B=""
UDP_PORT="${SLAN_ANDROID_DUAL_UDP_PORT:-40001}"
TCP_PORT="${SLAN_ANDROID_DUAL_TCP_PORT:-41001}"
NETWORK_MODULE_RULES_MIN="${SLAN_ANDROID_DUAL_MIN_SECURITY_RULES:-12}"
ANDROID_DEVICE_ID_WAIT_SECONDS="${SLAN_ANDROID_DUAL_DEVICE_ID_WAIT_SECONDS:-180}"
ANDROID_DEBUG_EMULATOR_VPN_BYPASS="${SLAN_ANDROID_DEBUG_EMULATOR_VPN_BYPASS:-1}"
# Phase 3/4 launch the receiver first, then wait for relay refresh, then build
# and run the sender side. On slower emulator / Gradle cycles 120s is not
# enough, and the receiver can exit just before the sender begins probing.
ANDROID_PHASE34_RECEIVER_HOLD_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_RECEIVER_HOLD_SECONDS:-240}"
ANDROID_PHASE34_MARKER_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_MARKER_WAIT_SECONDS:-300}"
ANDROID_PHASE34_RELAY_REFRESH_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_RELAY_REFRESH_WAIT_SECONDS:-120}"
ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS="${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS:-180}"
ANDROID_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS="${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS:-60}"
ANDROID_PHASE2_RECEIVER_SETTLE_SECONDS="${SLAN_ANDROID_DUAL_PHASE2_RECEIVER_SETTLE_SECONDS:-6}"
ANDROID_STOP_AFTER_PHASE2="${SLAN_ANDROID_DUAL_STOP_AFTER_PHASE2:-0}"
ANDROID_PHASE1_SETTLE_SECONDS="${SLAN_ANDROID_DUAL_PHASE1_SETTLE_SECONDS:-3}"
ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE1_DEVICE_ID_WAIT_SECONDS:-240}"
ANDROID_VPN_CONSENT_GUARD_SECONDS="${SLAN_ANDROID_DUAL_VPN_CONSENT_GUARD_SECONDS:-90}"
ANDROID_VPN_CONSENT_POLL_SECONDS="${SLAN_ANDROID_DUAL_VPN_CONSENT_POLL_SECONDS:-2}"
ANDROID_PRESET_VPN_BYPASS="${SLAN_ANDROID_DUAL_PRESET_VPN_BYPASS:-0}"
ANDROID_BG_RECEIVER_COMPLETION_GRACE_SECONDS="${SLAN_ANDROID_DUAL_BG_RECEIVER_COMPLETION_GRACE_SECONDS:-20}"

LOG_A_PHASE1="$WORK_DIR/android-a-phase1.log"
LOG_B_PHASE1="$WORK_DIR/android-b-phase1.log"
LOG_A_PHASE2="$WORK_DIR/android-a-phase2.log"
LOG_B_PHASE2="$WORK_DIR/android-b-phase2.log"
LOG_A_PHASE2_REPLY="$WORK_DIR/android-a-phase2-reply.log"
LOG_B_PHASE2_REPLY="$WORK_DIR/android-b-phase2-reply.log"
LOGCAT_A_PHASE2="$WORK_DIR/android-a-phase2.logcat"
LOGCAT_B_PHASE2="$WORK_DIR/android-b-phase2.logcat"
LOGCAT_A_PHASE2_REPLY="$WORK_DIR/android-a-phase2-reply.logcat"
LOGCAT_B_PHASE2_REPLY="$WORK_DIR/android-b-phase2-reply.logcat"
LOG_A_PHASE3="$WORK_DIR/android-a-phase3.log"
LOG_B_PHASE3="$WORK_DIR/android-b-phase3.log"
LOG_A_PHASE4="$WORK_DIR/android-a-phase4.log"
LOG_B_PHASE4="$WORK_DIR/android-b-phase4.log"
LOGCAT_A_PHASE3="$WORK_DIR/android-a-phase3.logcat"
LOGCAT_B_PHASE3="$WORK_DIR/android-b-phase3.logcat"
LOGCAT_A_PHASE4="$WORK_DIR/android-a-phase4.logcat"
LOGCAT_B_PHASE4="$WORK_DIR/android-b-phase4.logcat"

PIDS=()
RUN_FLUTTER_BG_PID=""
RULE_IDS=()
RECORD_IDS=()
BG_APP_DIRS=()
ZONE_ID=""
ZONE_NAME=""
NETWORK_ID=""
SECURITY_GROUP_ID=""
USER_ID=""
ANDROID_IP_A=""
ANDROID_IP_B=""

log() {
  printf '==> %s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

extract_json_field() {
  local json="$1"
  local field="$2"
  printf '%s' "$json" | sed -n "s/.*\"${field}\":\"\\([^\"]*\\)\".*/\\1/p" | head -n 1
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

wait_for_boot() {
  local device="$1"
  "$ADB" -s "$device" wait-for-device
  for _ in $(seq 1 120); do
    local boot_completed
    boot_completed="$("$ADB" -s "$device" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
    [[ "$boot_completed" == "1" ]] && return 0
    sleep 1
  done
  fail "device did not finish booting: $device"
}

prepare_device() {
  local device="$1"
  "$ADB" -s "$device" shell pm clear dev.slan.slan_client_v2 >/dev/null 2>&1 || true
  "$ADB" -s "$device" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
}

build_android_ffi() {
  log "build android ffi"
  "$ROOT_DIR/client_v2/scripts/build_android_ffi.sh"
}

start_vpn_guard() {
  local device="$1"
  (
    while true; do
      "$ADB" -s "$device" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
      sleep 0.25
    done
  ) &
  PIDS+=("$!")
}

tap_bounds_center() {
  local device="$1"
  local bounds="$2"
  [[ "$bounds" =~ \[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\] ]] || return 1
  local x1="${BASH_REMATCH[1]}"
  local y1="${BASH_REMATCH[2]}"
  local x2="${BASH_REMATCH[3]}"
  local y2="${BASH_REMATCH[4]}"
  local x=$(( (x1 + x2) / 2 ))
  local y=$(( (y1 + y2) / 2 ))
  "$ADB" -s "$device" shell input tap "$x" "$y" >/dev/null 2>&1 || true
}

start_vpn_consent_guard() {
  local device="$1"
  (
    local deadline=$((SECONDS + ANDROID_VPN_CONSENT_GUARD_SECONDS))
    while true; do
      if (( SECONDS >= deadline )); then
        break
      fi
      xml="$("$ADB" -s "$device" shell uiautomator dump /sdcard/slan-ui.xml >/dev/null 2>&1 && "$ADB" -s "$device" shell cat /sdcard/slan-ui.xml 2>/dev/null || true)"
      if [[ "$xml" == *"package=\"com.android.vpndialogs\""* || "$xml" == *"VPN"* || "$xml" == *"连接请求"* || "$xml" == *"网络请求"* ]]; then
        bounds="$(
          printf '%s' "$xml" | tr '>' '\n' | grep -E \
            'resource-id="android:id/button1"|resource-id="com.android.vpndialogs:id/button1"|text="(确定|OK|继续|允许|始终允许|同意|接受|Connect|Allow)"' \
            | sed -n 's/.*bounds="\([^"]*\)".*/\1/p' | head -n 1
        )"
        if [[ -n "$bounds" ]]; then
          tap_bounds_center "$device" "$bounds"
        else
          "$ADB" -s "$device" shell input keyevent 22 >/dev/null 2>&1 || true
          "$ADB" -s "$device" shell input keyevent 61 >/dev/null 2>&1 || true
          "$ADB" -s "$device" shell input keyevent 66 >/dev/null 2>&1 || true
        fi
      fi
      sleep "$ANDROID_VPN_CONSENT_POLL_SECONDS"
    done
  ) &
  PIDS+=("$!")
}

register_user_if_needed() {
  curl --silent --show-error --fail --connect-timeout 5 --max-time 20 \
    -X POST "${BIZ_URL}/api/app/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true
}

ensure_network_context() {
  [[ -n "$NETWORK_ID" && -n "$USER_ID" && -n "$SECURITY_GROUP_ID" ]] && return 0

  local auth networks groups
  auth="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "${WEB_BASE_URL}/api/web/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
  USER_ID="$(extract_json_field "$auth" "userId")"
  [[ -n "$USER_ID" ]] || fail "failed to parse user id"

  networks="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    "${WEB_BASE_URL}/api/web/networks?userId=${USER_ID}")"
  NETWORK_ID="$(extract_json_field "$networks" "networkId")"
  [[ -n "$NETWORK_ID" ]] || fail "failed to parse network id"

  groups="$(curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/security-groups")"
  SECURITY_GROUP_ID="$(extract_json_field "$groups" "securityGroupId")"
  [[ -n "$SECURITY_GROUP_ID" ]] || fail "failed to parse security group id"
}

create_security_rule() {
  local direction="$1"
  local peer_value="$2"
  local protocol="$3"
  local port="$4"
  local priority="$5"
  local response rule_id
  response="$(create_json "${WEB_BASE_URL}/api/web/security-groups/${SECURITY_GROUP_ID}/rules" \
    "{\"direction\":\"${direction}\",\"priority\":${priority},\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"device\",\"peerValue\":\"${peer_value}\",\"enabled\":true}")"
  rule_id="$(extract_json_field "$response" "ruleId")"
  [[ -n "$rule_id" ]] || fail "failed to create ${protocol}:${port} ${direction} rule for ${peer_value}"
  RULE_IDS+=("$rule_id")
}

provision_dns_acl_resources() {
  ensure_network_context

  local response record_id
  ZONE_NAME="android-dual-$(date +%s)-${RANDOM}.lan"
  response="$(create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/zones" \
    "{\"zoneName\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(extract_json_field "$response" "zoneId")"
  [[ -n "$ZONE_ID" ]] || fail "failed to create dns zone"

  response="$(create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records" \
    "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"android-a\",\"recordType\":\"A\",\"targetDeviceId\":\"${DEVICE_ID_A}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"443\",\"ttl\":60}")"
  record_id="$(extract_json_field "$response" "recordId")"
  [[ -n "$record_id" ]] || fail "failed to create dns record for android-a"
  RECORD_IDS+=("$record_id")

  response="$(create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records" \
    "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"android-b\",\"recordType\":\"A\",\"targetDeviceId\":\"${DEVICE_ID_B}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"443\",\"ttl\":60}")"
  record_id="$(extract_json_field "$response" "recordId")"
  [[ -n "$record_id" ]] || fail "failed to create dns record for android-b"
  RECORD_IDS+=("$record_id")

  local priority=100
  local peer
  for peer in "$DEVICE_ID_A" "$DEVICE_ID_B"; do
    create_security_rule ingress "$peer" tcp 443 "$priority"; priority=$((priority + 10))
    create_security_rule egress "$peer" tcp 443 "$priority"; priority=$((priority + 10))
    create_security_rule ingress "$peer" udp "$UDP_PORT" "$priority"; priority=$((priority + 10))
    create_security_rule egress "$peer" udp "$UDP_PORT" "$priority"; priority=$((priority + 10))
    create_security_rule ingress "$peer" tcp "$TCP_PORT" "$priority"; priority=$((priority + 10))
    create_security_rule egress "$peer" tcp "$TCP_PORT" "$priority"; priority=$((priority + 10))
  done

  log "provisioned dns zone ${ZONE_NAME} with records and acl rules"
}

run_flutter_test_in_dir() {
  local app_dir="$1"
  local device="$2"
  local log_file="$3"
  shift 3
  local markers=()
  while [[ $# -gt 0 && "$1" != "--" ]]; do
    markers+=("$1")
    shift
  done
  [[ $# -gt 0 ]] && shift
  local attempt status
  for attempt in 1 2; do
    status=0
    (
      cd "$app_dir"
      "$FLUTTER_BIN" test integration_test/mobile_login_test.dart \
        -d "$device" \
        --timeout "$TIMEOUT" \
        "$@"
    ) >"$log_file" 2>&1 || status=$?
    if [[ $status -eq 0 ]]; then
      return 0
    fi
    if flutter_cleanup_bug_only "$log_file" "${markers[@]}"; then
      warn "ignoring flutter cleanup-only failure for $log_file"
      return 0
    fi
    if [[ $attempt -lt 2 ]] && flutter_retryable_startup_failure "$log_file"; then
      warn "retrying flutter integration after startup failure on $device ($log_file)"
      sleep 3
      continue
    fi
    cat "$log_file" >&2
    return "$status"
  done
}

run_flutter_test() {
  local device="$1"
  local log_file="$2"
  shift 2
  run_flutter_test_in_dir "$APP_DIR" "$device" "$log_file" "$@"
}

run_flutter_test_bg() {
  local device="$1"
  local log_file="$2"
  shift 2
  local markers=()
  while [[ $# -gt 0 && "$1" != "--" ]]; do
    markers+=("$1")
    shift
  done
  [[ $# -gt 0 ]] && shift
  local bg_root bg_client_v2_dir bg_app_dir
  bg_root="$(mktemp -d "${TMPDIR:-/tmp}/slan-app-flutter-bg.XXXXXX")"
  bg_client_v2_dir="$bg_root/client_v2"
  bg_app_dir="$bg_client_v2_dir/app_flutter"
  mkdir -p "$bg_client_v2_dir"
  BG_APP_DIRS+=("$bg_root")
  rsync -a \
    --exclude '.dart_tool' \
    --exclude 'build' \
    --exclude 'build.rootcache.*' \
    --exclude 'android/.gradle' \
    --exclude 'android/.kotlin' \
    --exclude 'android/app/build' \
    --exclude 'ios/build' \
    --exclude 'ios/Pods' \
    --exclude 'macos/Pods' \
    --exclude 'macos/Flutter/ephemeral' \
    --exclude 'linux/flutter/ephemeral' \
    --exclude 'windows/flutter/ephemeral' \
    "$APP_DIR/" "$bg_app_dir/"
  ln -s "$ROOT_DIR/client_v2/plugins" "$bg_client_v2_dir/plugins"
  (
    run_flutter_test_in_dir "$bg_app_dir" "$device" "$log_file" "${markers[@]}" -- "$@"
  ) >"$log_file" 2>&1 &
  RUN_FLUTTER_BG_PID="$!"
  PIDS+=("$RUN_FLUTTER_BG_PID")
}

run_flutter_test_bg_ready() {
  local device="$1"
  local log_file="$2"
  local ready_marker="$3"
  local ready_timeout="${4:-60}"
  shift 4
  local attempt pid
  for attempt in 1 2; do
    : >"$log_file"
    run_flutter_test_bg "$device" "$log_file" "$@"
    pid="$RUN_FLUTTER_BG_PID"
    if wait_for_device_marker "$pid" "$log_file" "$ready_marker" "$ready_timeout" >/dev/null 2>&1; then
      RUN_FLUTTER_BG_PID="$pid"
      return 0
    fi
    warn "background flutter receiver did not reach ${ready_marker} on ${device} (attempt ${attempt}/2)"
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    if [[ $attempt -lt 2 ]] && flutter_retryable_startup_failure "$log_file"; then
      warn "retrying background flutter receiver startup on $device ($log_file)"
      sleep 3
      continue
    fi
    cat "$log_file" >&2
    fail "background flutter receiver failed before ready marker ${ready_marker}: $log_file"
  done
}

wait_for_bg_flutter_receiver() {
  local pid="$1"
  local log_file="$2"
  shift 2
  local markers=("$@")
  local grace_seconds="${ANDROID_BG_RECEIVER_COMPLETION_GRACE_SECONDS}"
  local attempt
  for attempt in $(seq 1 "$grace_seconds"); do
    if flutter_completed_with_expected_markers "$log_file" "${markers[@]}"; then
      log "background flutter receiver completed markers before wait pid=$pid log=$log_file"
      kill "$pid" 2>/dev/null || true
      for _ in $(seq 1 5); do
        if ! kill -0 "$pid" 2>/dev/null; then
          break
        fi
        sleep 1
      done
      if kill -0 "$pid" 2>/dev/null; then
        warn "background flutter receiver still alive after marker completion; continue without blocking wait for $log_file"
      fi
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 1
  done
  local wait_status=0
  if wait "$pid"; then
    wait_status=0
  else
    wait_status=$?
  fi
  log "background flutter receiver finished pid=$pid status=$wait_status log=$log_file"
  if [[ $wait_status -ne 0 ]]; then
    if flutter_cleanup_bug_only "$log_file" "${markers[@]}"; then
      warn "ignoring background flutter receiver cleanup-only failure for $log_file"
      return 0
    fi
    if flutter_completed_with_expected_markers "$log_file" "${markers[@]}"; then
      warn "ignoring background flutter receiver non-zero exit after successful markers for $log_file"
      return 0
    fi
    cat "$log_file" >&2
    fail "background flutter receiver exited with status $wait_status: $log_file"
  fi
  local marker
  for marker in "${markers[@]}"; do
    if ! grep -Fq "$marker" "$log_file"; then
      cat "$log_file" >&2
      fail "background flutter receiver finished without marker ${marker}: $log_file"
    fi
  done
  return 0
}

run_flutter_test_with_ready_and_completion_markers() {
  local device="$1"
  local log_file="$2"
  local ready_marker="$3"
  local ready_timeout="${4:-60}"
  local completion_marker="$5"
  local completion_timeout="${6:-120}"
  shift 6
  local attempt pid
  local markers=("$@")
  for attempt in 1 2; do
    : >"$log_file"
    run_flutter_test_bg "$device" "$log_file" "$@"
    pid="$RUN_FLUTTER_BG_PID"
    if ! wait_for_log_presence "$pid" "$log_file" "$ready_marker" "$ready_timeout" >/dev/null 2>&1; then
      warn "flutter sender did not reach ${ready_marker} on ${device} (attempt ${attempt}/2)"
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      if [[ $attempt -lt 2 ]]; then
        sleep 3
        continue
      fi
      cat "$log_file" >&2
      fail "flutter sender failed before ready marker ${ready_marker}: $log_file"
    fi
    if ! wait_for_log_presence "$pid" "$log_file" "$completion_marker" "$completion_timeout" >/dev/null 2>&1; then
      warn "flutter sender did not reach ${completion_marker} on ${device} (attempt ${attempt}/2)"
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      if [[ $attempt -lt 2 ]]; then
        sleep 3
        continue
      fi
      cat "$log_file" >&2
      fail "flutter sender failed before completion marker ${completion_marker}: $log_file"
    fi
    log "waiting for flutter sender pid=$pid log=$log_file"
    local wait_status=0
    if wait "$pid"; then
      wait_status=0
    else
      wait_status=$?
    fi
    log "flutter sender finished pid=$pid status=$wait_status log=$log_file"
    if [[ $wait_status -ne 0 ]]; then
      if flutter_cleanup_bug_only "$log_file" "${markers[@]}"; then
        warn "ignoring flutter sender cleanup-only wait failure for $log_file"
        return 0
      fi
      if flutter_completed_with_expected_markers "$log_file" "${markers[@]}"; then
        warn "ignoring flutter sender non-zero exit after successful markers for $log_file"
        return 0
      fi
      cat "$log_file" >&2
      fail "flutter sender exited with status $wait_status after completion marker: $log_file"
    fi
    return 0
  done
}

flutter_cleanup_bug_only() {
  local log_file="$1"
  shift
  local marker
  local has_markers=1
  for marker in "$@"; do
    if ! grep -Fq "$marker" "$log_file"; then
      has_markers=0
      break
    fi
  done
  [[ $has_markers -eq 1 ]] || return 1
  grep -Eq \
    'PathNotFoundException: Deletion failed.*flutter_test_listener|No tests were found\.|A SemanticsHandle was active at the end of the test|did not complete \[E\]' \
    "$log_file"
}

flutter_completed_with_expected_markers() {
  local log_file="$1"
  shift
  local marker
  for marker in "$@"; do
    grep -Fq "$marker" "$log_file" || return 1
  done
  grep -Fq 'All tests passed!' "$log_file"
}

flutter_retryable_startup_failure() {
  local log_file="$1"
  grep -Eq \
    'No tests were found\.|Error waiting for a debug connection|Failed to establish connection with the application instance|Test never completed|VMServiceFlutterDriver: request_data message is taking a long time to complete|Shell subprocess crashed with segmentation fault' \
    "$log_file"
}

warm_flutter_build() {
  local device="$1"
  local log_file="$2"
  (
    cd "$APP_DIR"
    "$FLUTTER_BIN" build apk --debug
  ) >"$log_file" 2>&1 || true
}

start_logcat_capture() {
  local device="$1"
  local log_file="$2"
  "$ADB" -s "$device" logcat -c >/dev/null 2>&1 || true
  (
    "$ADB" -s "$device" logcat -v threadtime \
      SlanVpnService:I \
      client-core-ffi:I \
      flutter:I \
      RustStdoutStderr:I \
      '*:S'
  ) >"$log_file" 2>&1 &
  PIDS+=("$!")
}

set_android_vpn_bypass_pref() {
  local device="$1"
  local enabled="$2"
  [[ "$ANDROID_PRESET_VPN_BYPASS" == "1" ]] || return 0
  "$ADB" -s "$device" shell am start -n dev.slan.slan_client_v2/.MainActivity >/dev/null 2>&1 || true
  sleep 2
  (
    cd "$APP_DIR"
    "$FLUTTER_BIN" test integration_test/mobile_login_test.dart \
      -d "$device" \
      --timeout "$TIMEOUT" \
      --plain-name "mobile password login signs in through client-core-service" \
      --dart-define="SLAN_TEST_BIZ_URL=$ANDROID_BIZ_URL" \
      --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$ANDROID_BIZ_URL" \
      --dart-define="SLAN_TEST_EMAIL=$EMAIL" \
      --dart-define="SLAN_TEST_PASSWORD=$PASSWORD" \
      --dart-define="SLAN_TEST_REGISTER_USER=false" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=false" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=0" \
      --dart-define="SLAN_TEST_ANDROID_SET_VPN_BYPASS_ONLY=true" \
      --dart-define="SLAN_TEST_ANDROID_DEBUG_EMULATOR_VPN_BYPASS=$enabled"
  ) >/dev/null 2>&1 || true
}

extract_log_value() {
  local log_file="$1"
  local prefix="$2"
  sed -n "s/.*${prefix}=\\([^[:space:]]*\\).*/\\1/p" "$log_file" | tail -n 1
}

relay_admin_base_url_from_udp_address() {
  local relay_address="$1"
  local trimmed host port admin_port
  trimmed="${relay_address#udp://}"
  host="${trimmed%:*}"
  port="${trimmed##*:}"
  [[ -n "$host" && "$port" =~ ^[0-9]+$ ]] || return 1
  admin_port=$((port + 1))
  printf 'http://%s:%s\n' "$host" "$admin_port"
}

extract_json_int_field() {
  local json="$1"
  local field="$2"
  printf '%s' "$json" | sed -n "s/.*\"${field}\":\\([0-9][0-9]*\\).*/\\1/p" | head -n 1
}

wait_for_relay_refresh_marker() {
  local admin_base_url="$1"
  local participant_id="$2"
  local previous_refresh_count="$3"
  local timeout_seconds="${4:-30}"
  local metrics refresh_count
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    metrics="$(curl --silent --show-error --max-time 5 "${admin_base_url}/metrics" 2>/dev/null || true)"
    refresh_count="$(extract_json_int_field "$metrics" "participantRefreshCount")"
    if [[ -n "$refresh_count" &&
          "$refresh_count" =~ ^[0-9]+$ &&
          "$refresh_count" -gt "$previous_refresh_count" ]]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_device_marker() {
  local pid="$1"
  local log_file="$2"
  local marker="$3"
  local timeout_seconds="${4:-$ANDROID_DEVICE_ID_WAIT_SECONDS}"
  local value=""
  for _ in $(seq 1 "$timeout_seconds"); do
    value="$(extract_log_value "$log_file" "$marker")"
    [[ -n "$value" ]] && break
    if ! kill -0 "$pid" 2>/dev/null; then
      sleep 1
      value="$(extract_log_value "$log_file" "$marker")"
      [[ -n "$value" ]] && break
      cat "$log_file" >&2
      fail "process exited before marker ${marker}: $log_file"
    fi
    sleep 1
  done
  [[ -n "$value" ]] || {
    cat "$log_file" >&2
    fail "timed out waiting for marker ${marker}: $log_file"
  }
  printf '%s\n' "$value"
}

run_phase1_capture() {
  run_flutter_test "$DEVICE_A" "$LOG_A_PHASE1" \
    "SLAN_TEST_CLIENT_DEVICE_ID=" \
    "SLAN_TEST_NETWORK_IP=" \
    "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
    -- \
    "${DEVICE_A_DART_DEFINES[@]}"

  run_flutter_test "$DEVICE_B" "$LOG_B_PHASE1" \
    "SLAN_TEST_CLIENT_DEVICE_ID=" \
    "SLAN_TEST_NETWORK_IP=" \
    "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
    -- \
    "${DEVICE_B_DART_DEFINES[@]}"

  # The second emulator often flushes its final login/network markers a little
  # later than process exit, especially after fresh installs on slower hosts.
  sleep "$ANDROID_PHASE1_SETTLE_SECONDS"

  DEVICE_ID_A="$(wait_for_completed_log_marker "$LOG_A_PHASE1" "SLAN_TEST_CLIENT_DEVICE_ID" "$ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS")"
  DEVICE_ID_B="$(wait_for_completed_log_marker "$LOG_B_PHASE1" "SLAN_TEST_CLIENT_DEVICE_ID" "$ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS")"
  [[ -n "$DEVICE_ID_A" ]] || { cat "$LOG_A_PHASE1" >&2; fail "failed to capture android A device id"; }
  [[ -n "$DEVICE_ID_B" ]] || { cat "$LOG_B_PHASE1" >&2; fail "failed to capture android B device id"; }

  ANDROID_IP_A="$(wait_for_completed_log_marker "$LOG_A_PHASE1" "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS")"
  ANDROID_IP_B="$(wait_for_completed_log_marker "$LOG_B_PHASE1" "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS")"
  [[ -n "$ANDROID_IP_A" ]] || { cat "$LOG_A_PHASE1" >&2; fail "failed to capture android A virtual ip"; }
  [[ -n "$ANDROID_IP_B" ]] || { cat "$LOG_B_PHASE1" >&2; fail "failed to capture android B virtual ip"; }

  RELAY_ADDRESS_B="$(extract_log_value "$LOG_B_PHASE1" "relayAddress")"
  RELAY_ADMIN_BASE_URL=""
  if [[ -n "$RELAY_ADDRESS_B" ]]; then
    RELAY_ADMIN_BASE_URL="$(relay_admin_base_url_from_udp_address "$RELAY_ADDRESS_B" || true)"
  fi
}

wait_for_completed_log_marker() {
  local log_file="$1"
  local marker="$2"
  local timeout_seconds="$3"
  local value=""
  for _ in $(seq 1 "$timeout_seconds"); do
    value="$(extract_log_value "$log_file" "$marker")"
    [[ -n "$value" ]] && break
    sleep 1
  done
  [[ -n "$value" ]] || {
    cat "$log_file" >&2
    fail "timed out waiting for completed log marker ${marker}: $log_file"
  }
  printf '%s\n' "$value"
}

wait_for_log_presence() {
  local pid="$1"
  local log_file="$2"
  local pattern="$3"
  local timeout_seconds="${4:-60}"
  for _ in $(seq 1 "$timeout_seconds"); do
    if grep -Fq "$pattern" "$log_file"; then
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      sleep 1
      if grep -Fq "$pattern" "$log_file"; then
        return 0
      fi
      cat "$log_file" >&2
      fail "process exited before log pattern ${pattern}: $log_file"
    fi
    sleep 1
  done
  cat "$log_file" >&2
  fail "timed out waiting for log pattern ${pattern}: $log_file"
}

wait_for_completed_log_markers() {
  local log_file="$1"
  local timeout_seconds="$2"
  shift 2
  local marker
  for _ in $(seq 1 "$timeout_seconds"); do
    if flutter_completed_with_expected_markers "$log_file" "$@"; then
      return 0
    fi
    sleep 1
  done
  echo "---- $(basename "$log_file") ----" >&2
  cat "$log_file" >&2
  for marker in "$@"; do
    grep -Fq "$marker" "$log_file" || warn "missing completion marker ${marker} in $log_file"
  done
  fail "timed out waiting for completed log markers: $log_file"
}

stop_bg_flutter_receiver() {
  local pid="$1"
  local status=0
  [[ -n "$pid" ]] || return 0
  log "stopping background flutter receiver pid=$pid"
  kill "$pid" 2>/dev/null || true
  if wait "$pid" 2>/dev/null; then
    status=0
  else
    status=$?
  fi
  log "background flutter receiver stopped pid=$pid status=$status"
  return 0
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    local log_file
    for log_file in \
      "$LOG_A_PHASE1" "$LOG_B_PHASE1" \
      "$LOG_A_PHASE2" "$LOG_B_PHASE2" \
      "$LOG_A_PHASE2_REPLY" "$LOG_B_PHASE2_REPLY" \
      "$LOGCAT_A_PHASE2" "$LOGCAT_B_PHASE2" \
      "$LOGCAT_A_PHASE2_REPLY" "$LOGCAT_B_PHASE2_REPLY" \
      "$LOG_A_PHASE3" "$LOG_B_PHASE3" \
      "$LOG_A_PHASE4" "$LOG_B_PHASE4" \
      "$LOGCAT_A_PHASE3" "$LOGCAT_B_PHASE3" \
      "$LOGCAT_A_PHASE4" "$LOGCAT_B_PHASE4"; do
      if [[ -f "$log_file" ]]; then
        echo "---- $(basename "$log_file") ----" >&2
        cat "$log_file" >&2
      fi
    done
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  local bg_dir
  for bg_dir in "${BG_APP_DIRS[@]:-}"; do
    rm -rf "$bg_dir" 2>/dev/null || true
  done
  if [[ -n "$NETWORK_ID" ]]; then
    local index
    for ((index=${#RULE_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${WEB_BASE_URL}/api/web/security-groups/rules/${RULE_IDS[$index]}"
    done
    for ((index=${#RECORD_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records/${RECORD_IDS[$index]}"
    done
    [[ -n "$ZONE_ID" ]] && best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/zones/${ZONE_ID}"
  fi
  slan_cleanup_remote_test_devices "$WEB_BASE_URL" "$EMAIL" "$PASSWORD" 1
  if [[ $status -ne 0 || "${SLAN_KEEP_ANDROID_DUAL_WORK_DIR:-0}" == "1" ]]; then
    echo "kept work dir: $WORK_DIR"
  else
    rm -rf "$WORK_DIR"
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

command -v "$ADB" >/dev/null 2>&1 || fail "adb missing: $ADB"
command -v "$FLUTTER_BIN" >/dev/null 2>&1 || fail "flutter missing: $FLUTTER_BIN"
mkdir -p "$WORK_DIR"

log "wait for emulator boot"
wait_for_boot "$DEVICE_A"
wait_for_boot "$DEVICE_B"
prepare_device "$DEVICE_A"
prepare_device "$DEVICE_B"
if [[ "$ANDROID_PRESET_VPN_BYPASS" == "1" ]]; then
  log "preset android emulator vpn bypass preference"
fi
set_android_vpn_bypass_pref "$DEVICE_A" "$ANDROID_DEBUG_EMULATOR_VPN_BYPASS"
set_android_vpn_bypass_pref "$DEVICE_B" "$ANDROID_DEBUG_EMULATOR_VPN_BYPASS"
start_vpn_guard "$DEVICE_A"
start_vpn_guard "$DEVICE_B"
start_vpn_consent_guard "$DEVICE_A"
start_vpn_consent_guard "$DEVICE_B"
register_user_if_needed
"$ADB" -s "$DEVICE_A" shell appops get dev.slan.slan_client_v2 ACTIVATE_VPN >/dev/null 2>&1 || true
"$ADB" -s "$DEVICE_B" shell appops get dev.slan.slan_client_v2 ACTIVATE_VPN >/dev/null 2>&1 || true

COMMON_DART_DEFINES=(
  --dart-define="SLAN_TEST_BIZ_URL=$ANDROID_BIZ_URL"
  --dart-define="SLAN_EMBEDDED_CONTROL_BASE_URL=$ANDROID_BIZ_URL"
  --dart-define="SLAN_TEST_EMAIL=$EMAIL"
  --dart-define="SLAN_TEST_PASSWORD=$PASSWORD"
  --dart-define="SLAN_TEST_REGISTER_USER=false"
  --dart-define="SLAN_TEST_WAIT_MQTT=true"
  --dart-define="SLAN_TEST_CHECK_SWITCH=true"
  --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=3"
  --dart-define="SLAN_UI_DIAGNOSTICS=true"
)
DEVICE_A_DART_DEFINES=(
  "${COMMON_DART_DEFINES[@]}"
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_A"
)
DEVICE_B_DART_DEFINES=(
  "${COMMON_DART_DEFINES[@]}"
  --dart-define="SLAN_TEST_DEVICE_ID=$REQUESTED_DEVICE_ID_B"
)

build_android_ffi
log "warm flutter build cache"
warm_flutter_build "$DEVICE_A" "$WORK_DIR/flutter-warmup.log"
# Warmup can register a throwaway device under the same test account, which
# pollutes later phase-1 peer/relay counts. Clear both the remote device list
# and local app state again before the real dual-device run starts.
slan_cleanup_remote_test_devices "$WEB_BASE_URL" "$EMAIL" "$PASSWORD" 1
prepare_device "$DEVICE_A"
prepare_device "$DEVICE_B"

log "phase 1: establish both android devices and capture device ids / virtual ips"
PHASE1_MAX_ATTEMPTS="${SLAN_ANDROID_DUAL_PHASE1_MAX_ATTEMPTS:-3}"
for phase1_attempt in $(seq 1 "$PHASE1_MAX_ATTEMPTS"); do
  log "phase 1 attempt ${phase1_attempt}/${PHASE1_MAX_ATTEMPTS}"
  run_phase1_capture
  if [[ "$ANDROID_IP_A" != "$ANDROID_IP_B" ]]; then
    break
  fi
  log "phase 1 duplicate virtual ip detected: android-a=$ANDROID_IP_A android-b=$ANDROID_IP_B"
  if [[ "$phase1_attempt" -ge "$PHASE1_MAX_ATTEMPTS" ]]; then
    cat "$LOG_A_PHASE1" >&2
    cat "$LOG_B_PHASE1" >&2
    fail "phase 1 assigned duplicate virtual ip to both android devices: $ANDROID_IP_A"
  fi
  slan_cleanup_remote_test_devices "$WEB_BASE_URL" "$EMAIL" "$PASSWORD" 1
  prepare_device "$DEVICE_A"
  prepare_device "$DEVICE_B"
  sleep 3
done

log "phase 1 result: android-a deviceId=$DEVICE_ID_A ip=$ANDROID_IP_A"
log "phase 1 result: android-b deviceId=$DEVICE_ID_B ip=$ANDROID_IP_B relay=${RELAY_ADDRESS_B:-n/a} admin=${RELAY_ADMIN_BASE_URL:-n/a}"

provision_dns_acl_resources

MESSAGE_A_TO_B="android-a-to-b-$(date +%s%N)"
MESSAGE_B_TO_A="android-b-to-a-$(date +%s%N)"
TARGET_A_DNS="android-a.${ZONE_NAME}"
TARGET_B_DNS="android-b.${ZONE_NAME}"

log "phase 2: validate network module and bidirectional client messages"
start_logcat_capture "$DEVICE_A" "$LOGCAT_A_PHASE2"
start_logcat_capture "$DEVICE_B" "$LOGCAT_B_PHASE2"
run_flutter_test_bg_ready "$DEVICE_B" "$LOG_B_PHASE2" "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_CLIENT_MESSAGE_OK=" \
  -- \
  "${DEVICE_B_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS=2" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=$NETWORK_MODULE_RULES_MIN" \
  --dart-define="SLAN_TEST_EXPECT_MQTT_TIMEOUT_SECONDS=$ANDROID_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_TIMEOUT_SECONDS=$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MESSAGE_A_TO_B" >/dev/null
PHASE2_BG_PID="$RUN_FLUTTER_BG_PID"
DEVICE_ID_B_PHASE2="$(wait_for_device_marker "$PHASE2_BG_PID" "$LOG_B_PHASE2" "SLAN_TEST_CLIENT_DEVICE_ID" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS")"
[[ -n "$DEVICE_ID_B_PHASE2" ]] || { cat "$LOG_B_PHASE2" >&2; fail "failed to capture android B phase2 device id"; }
log "phase 2 forward receiver current deviceId=$DEVICE_ID_B_PHASE2 (phase1=$DEVICE_ID_B)"
wait_for_device_marker "$PHASE2_BG_PID" "$LOG_B_PHASE2" "SLAN_TEST_MQTT_STATUS" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" >/dev/null
log "phase 2 forward receiver mqtt ready; settling ${ANDROID_PHASE2_RECEIVER_SETTLE_SECONDS}s before sender"
sleep "$ANDROID_PHASE2_RECEIVER_SETTLE_SECONDS"
run_flutter_test_with_ready_and_completion_markers \
  "$DEVICE_A" "$LOG_A_PHASE2" \
  "SLAN_TEST_MQTT_STATUS" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_CLIENT_MESSAGE_SENT=${DEVICE_ID_B_PHASE2}:${MESSAGE_A_TO_B}" "$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_MQTT_STATUS" \
  "SLAN_TEST_CLIENT_MESSAGE_SENT=${DEVICE_ID_B_PHASE2}:${MESSAGE_A_TO_B}" \
  "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
  -- \
  "${DEVICE_A_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS=2" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=$NETWORK_MODULE_RULES_MIN" \
  --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$DEVICE_ID_B_PHASE2" \
  --dart-define="SLAN_TEST_SEND_BODY=$MESSAGE_A_TO_B"
wait_for_completed_log_markers \
  "$LOG_B_PHASE2" "$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_CLIENT_MESSAGE_OK="
stop_bg_flutter_receiver "$PHASE2_BG_PID"
log "phase 2 forward receiver completed"
DEVICE_ID_A_PHASE2="$(wait_for_completed_log_marker "$LOG_A_PHASE2" "SLAN_TEST_CLIENT_DEVICE_ID" "$ANDROID_PHASE1_DEVICE_ID_WAIT_SECONDS")"
[[ -n "$DEVICE_ID_A_PHASE2" ]] || { cat "$LOG_A_PHASE2" >&2; fail "failed to capture android A phase2 device id"; }
log "phase 2 forward sender current deviceId=$DEVICE_ID_A_PHASE2 (phase1=$DEVICE_ID_A)"

log "phase 2 reverse receiver bootstrap begin"
start_logcat_capture "$DEVICE_A" "$LOGCAT_A_PHASE2_REPLY"
start_logcat_capture "$DEVICE_B" "$LOGCAT_B_PHASE2_REPLY"
run_flutter_test_bg_ready "$DEVICE_A" "$LOG_A_PHASE2_REPLY" "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_CLIENT_MESSAGE_OK=" \
  -- \
  "${DEVICE_A_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS=2" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=$NETWORK_MODULE_RULES_MIN" \
  --dart-define="SLAN_TEST_EXPECT_MQTT_TIMEOUT_SECONDS=$ANDROID_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_TIMEOUT_SECONDS=$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  --dart-define="SLAN_TEST_EXPECT_MESSAGE_BODY=$MESSAGE_B_TO_A" >/dev/null
PHASE2_REPLY_BG_PID="$RUN_FLUTTER_BG_PID"
log "phase 2 reverse receiver background pid=$PHASE2_REPLY_BG_PID"
DEVICE_ID_A_PHASE2_REPLY="$(wait_for_device_marker "$PHASE2_REPLY_BG_PID" "$LOG_A_PHASE2_REPLY" "SLAN_TEST_CLIENT_DEVICE_ID" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS")"
[[ -n "$DEVICE_ID_A_PHASE2_REPLY" ]] || { cat "$LOG_A_PHASE2_REPLY" >&2; fail "failed to capture android A phase2 reply device id"; }
log "phase 2 reverse receiver current deviceId=$DEVICE_ID_A_PHASE2_REPLY (forward sender=$DEVICE_ID_A_PHASE2)"
wait_for_device_marker "$PHASE2_REPLY_BG_PID" "$LOG_A_PHASE2_REPLY" "SLAN_TEST_MQTT_STATUS" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" >/dev/null
log "phase 2 reverse receiver mqtt ready; settling ${ANDROID_PHASE2_RECEIVER_SETTLE_SECONDS}s before sender"
sleep "$ANDROID_PHASE2_RECEIVER_SETTLE_SECONDS"
run_flutter_test_with_ready_and_completion_markers \
  "$DEVICE_B" "$LOG_B_PHASE2_REPLY" \
  "SLAN_TEST_MQTT_STATUS" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_CLIENT_MESSAGE_SENT=${DEVICE_ID_A_PHASE2_REPLY}:${MESSAGE_B_TO_A}" "$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_MQTT_STATUS" \
  "SLAN_TEST_CLIENT_MESSAGE_SENT=${DEVICE_ID_A_PHASE2_REPLY}:${MESSAGE_B_TO_A}" \
  "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
  -- \
  "${DEVICE_B_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS=2" \
  --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=$NETWORK_MODULE_RULES_MIN" \
  --dart-define="SLAN_TEST_SEND_TARGET_DEVICE_ID=$DEVICE_ID_A_PHASE2_REPLY" \
  --dart-define="SLAN_TEST_SEND_BODY=$MESSAGE_B_TO_A"
wait_for_completed_log_markers \
  "$LOG_A_PHASE2_REPLY" "$ANDROID_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_NETWORK_MODULE=" \
  "SLAN_TEST_CLIENT_MESSAGE_OK="
stop_bg_flutter_receiver "$PHASE2_REPLY_BG_PID"

if [[ "$ANDROID_STOP_AFTER_PHASE2" == "1" ]]; then
  log "stopping after phase 2 by request"
  log "android dual phase2 integration ok email=$EMAIL a=$DEVICE_ID_A/$ANDROID_IP_A b=$DEVICE_ID_B/$ANDROID_IP_B zone=$ZONE_NAME"
  exit 0
fi

log "phase 3: validate android-b sends UDP/TCP to android-a via DNS"
PHASE3_REFRESH_COUNT=0
if [[ -n "$RELAY_ADMIN_BASE_URL" ]]; then
  PHASE3_METRICS="$(curl --silent --show-error --max-time 5 "${RELAY_ADMIN_BASE_URL}/metrics" 2>/dev/null || true)"
  PHASE3_REFRESH_COUNT="$(extract_json_int_field "$PHASE3_METRICS" "participantRefreshCount")"
  PHASE3_REFRESH_COUNT="${PHASE3_REFRESH_COUNT:-0}"
fi
log "phase 3 receiver wait: deviceId=$DEVICE_ID_A previousRefreshCount=$PHASE3_REFRESH_COUNT relayAdmin=${RELAY_ADMIN_BASE_URL:-n/a}"
start_logcat_capture "$DEVICE_A" "$LOGCAT_A_PHASE3"
start_logcat_capture "$DEVICE_B" "$LOGCAT_B_PHASE3"
run_flutter_test_bg_ready "$DEVICE_A" "$LOG_A_PHASE3" "SLAN_TEST_UDP_ECHO_PORT" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
  "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
  -- \
  "${DEVICE_A_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
  --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
  --dart-define="SLAN_TEST_HOLD_SECONDS=$ANDROID_PHASE34_RECEIVER_HOLD_SECONDS" >/dev/null
PHASE3_BG_PID="$RUN_FLUTTER_BG_PID"
wait_for_device_marker "$PHASE3_BG_PID" "$LOG_A_PHASE3" "SLAN_TEST_TCP_ECHO_PORT" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" >/dev/null
if [[ -n "$RELAY_ADMIN_BASE_URL" ]]; then
  wait_for_relay_refresh_marker "$RELAY_ADMIN_BASE_URL" "$DEVICE_ID_A" "$PHASE3_REFRESH_COUNT" "$ANDROID_PHASE34_RELAY_REFRESH_WAIT_SECONDS" || {
    curl --silent --show-error --max-time 10 "${RELAY_ADMIN_BASE_URL}/metrics" >&2 || true
    curl --silent --show-error --max-time 10 "${RELAY_ADMIN_BASE_URL}/sessions" >&2 || true
    warn "timed out waiting for relay refresh marker for phase3 receiver ${DEVICE_ID_A}; continuing with socket probes"
  }
fi
run_flutter_test_with_ready_and_completion_markers "$DEVICE_B" "$LOG_B_PHASE3" \
  "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_TCP_ECHO_OK=${TARGET_A_DNS}:${TCP_PORT}" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_MQTT_STATUS" \
  "SLAN_TEST_UDP_SEND_TARGET=${TARGET_A_DNS}:${UDP_PORT}" \
  "SLAN_TEST_TCP_SEND_TARGET=${TARGET_A_DNS}:${TCP_PORT}" \
  "SLAN_TEST_ANDROID_PACKET_TUNNEL_READY=" \
  "SLAN_TEST_UDP_ECHO_OK=${TARGET_A_DNS}:${UDP_PORT}" \
  "SLAN_TEST_TCP_ECHO_OK=${TARGET_A_DNS}:${TCP_PORT}" \
  "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
  -- \
  "${DEVICE_B_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_UDP_SEND_TARGET=${TARGET_A_DNS}:${UDP_PORT}" \
  --dart-define="SLAN_TEST_UDP_SEND_BODY=udp-b-to-a-$(date +%s%N)" \
  --dart-define="SLAN_TEST_TCP_SEND_TARGET=${TARGET_A_DNS}:${TCP_PORT}" \
  --dart-define="SLAN_TEST_TCP_SEND_BODY=tcp-b-to-a-$(date +%s%N)"
log "phase 3 sender returned"
stop_bg_flutter_receiver "$PHASE3_BG_PID"
log "phase 3 complete"

log "phase 4: validate android-a sends UDP/TCP to android-b via DNS"
PHASE4_REFRESH_COUNT=0
if [[ -n "$RELAY_ADMIN_BASE_URL" ]]; then
  PHASE4_METRICS="$(curl --silent --show-error --max-time 5 "${RELAY_ADMIN_BASE_URL}/metrics" 2>/dev/null || true)"
  PHASE4_REFRESH_COUNT="$(extract_json_int_field "$PHASE4_METRICS" "participantRefreshCount")"
  PHASE4_REFRESH_COUNT="${PHASE4_REFRESH_COUNT:-0}"
fi
log "phase 4 receiver wait: deviceId=$DEVICE_ID_B previousRefreshCount=$PHASE4_REFRESH_COUNT relayAdmin=${RELAY_ADMIN_BASE_URL:-n/a}"
start_logcat_capture "$DEVICE_A" "$LOGCAT_A_PHASE4"
start_logcat_capture "$DEVICE_B" "$LOGCAT_B_PHASE4"
run_flutter_test_bg_ready "$DEVICE_B" "$LOG_B_PHASE4" "SLAN_TEST_UDP_ECHO_PORT" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
  "SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
  -- \
  "${DEVICE_B_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_UDP_ECHO_PORT=$UDP_PORT" \
  --dart-define="SLAN_TEST_TCP_ECHO_PORT=$TCP_PORT" \
  --dart-define="SLAN_TEST_HOLD_SECONDS=$ANDROID_PHASE34_RECEIVER_HOLD_SECONDS" >/dev/null
PHASE4_BG_PID="$RUN_FLUTTER_BG_PID"
wait_for_device_marker "$PHASE4_BG_PID" "$LOG_B_PHASE4" "SLAN_TEST_TCP_ECHO_PORT" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" >/dev/null
if [[ -n "$RELAY_ADMIN_BASE_URL" ]]; then
  wait_for_relay_refresh_marker "$RELAY_ADMIN_BASE_URL" "$DEVICE_ID_B" "$PHASE4_REFRESH_COUNT" "$ANDROID_PHASE34_RELAY_REFRESH_WAIT_SECONDS" || {
    curl --silent --show-error --max-time 10 "${RELAY_ADMIN_BASE_URL}/metrics" >&2 || true
    curl --silent --show-error --max-time 10 "${RELAY_ADMIN_BASE_URL}/sessions" >&2 || true
    warn "timed out waiting for relay refresh marker for phase4 receiver ${DEVICE_ID_B}; continuing with socket probes"
  }
fi
run_flutter_test_with_ready_and_completion_markers "$DEVICE_A" "$LOG_A_PHASE4" \
  "SLAN_TEST_NETWORK_IP" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_TCP_ECHO_OK=${TARGET_B_DNS}:${TCP_PORT}" "$ANDROID_PHASE34_MARKER_WAIT_SECONDS" \
  "SLAN_TEST_NETWORK_IP=" \
  "SLAN_TEST_MQTT_STATUS" \
  "SLAN_TEST_UDP_SEND_TARGET=${TARGET_B_DNS}:${UDP_PORT}" \
  "SLAN_TEST_TCP_SEND_TARGET=${TARGET_B_DNS}:${TCP_PORT}" \
  "SLAN_TEST_ANDROID_PACKET_TUNNEL_READY=" \
  "SLAN_TEST_UDP_ECHO_OK=${TARGET_B_DNS}:${UDP_PORT}" \
  "SLAN_TEST_TCP_ECHO_OK=${TARGET_B_DNS}:${TCP_PORT}" \
  "SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD=" \
  -- \
  "${DEVICE_A_DART_DEFINES[@]}" \
  --dart-define="SLAN_TEST_UDP_SEND_TARGET=${TARGET_B_DNS}:${UDP_PORT}" \
  --dart-define="SLAN_TEST_UDP_SEND_BODY=udp-a-to-b-$(date +%s%N)" \
  --dart-define="SLAN_TEST_TCP_SEND_TARGET=${TARGET_B_DNS}:${TCP_PORT}" \
  --dart-define="SLAN_TEST_TCP_SEND_BODY=tcp-a-to-b-$(date +%s%N)"
log "phase 4 sender returned"
stop_bg_flutter_receiver "$PHASE4_BG_PID"
log "phase 4 complete"

log "android dual emulator integration ok email=$EMAIL a=$DEVICE_ID_A/$ANDROID_IP_A b=$DEVICE_ID_B/$ANDROID_IP_B zone=$ZONE_NAME"
