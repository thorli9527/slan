#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

MODE=""
KEEP_WORK_DIR_OVERRIDE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    ""|--full-stable|--phase2-only )
      [[ -z "$MODE" ]] || {
        printf 'duplicate mode option: %s\n' "$1" >&2
        exit 1
      }
      MODE="$1"
      ;;
    --keep-work-dir )
      KEEP_WORK_DIR_OVERRIDE="1"
      ;;
    --no-keep-work-dir )
      KEEP_WORK_DIR_OVERRIDE="0"
      ;;
    -h|--help )
      MODE="--help"
      ;;
    * )
      printf 'unknown option: %s\n' "$1" >&2
      printf 'run with --help for usage\n' >&2
      exit 1
      ;;
  esac
  shift
done

if [[ "$MODE" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/android_dual_fast_check.sh
  bash scripts/android_dual_fast_check.sh --full-stable
  bash scripts/android_dual_fast_check.sh --phase2-only
  bash scripts/android_dual_fast_check.sh --phase2-only --keep-work-dir

Purpose:
  Run the known-good dual-Android stable validation using the remote service
  stack and the verified two-emulator defaults. This is the quickest way to
  rerun the full Android business chain after code changes.

Optional environment variables:
  SLAN_BIZ_URL
  SLAN_OPS_BASE_URL
  SLAN_ANDROID_DEVICE_A
  SLAN_ANDROID_DEVICE_B
  SLAN_ANDROID_DEVICE_ID_A
  SLAN_ANDROID_DEVICE_ID_B
  SLAN_ANDROID_AVD_A
  SLAN_ANDROID_AVD_B
  SLAN_ANDROID_DEBUG_EMULATOR_VPN_BYPASS=1|0
  SLAN_ANDROID_DUAL_TIMEOUT
  SLAN_ANDROID_DUAL_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS
  SLAN_ANDROID_DUAL_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS
  SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST
  SLAN_ANDROID_DUAL_REQUIRE_DIRECT_UDP=1|0
  SLAN_KEEP_ANDROID_DUAL_WORK_DIR=1|0
  SLAN_ANDROID_DUAL_KEEP_WORK_DIR=1|0

Examples:
  bash scripts/android_dual_fast_check.sh
  bash scripts/android_dual_fast_check.sh --full-stable
  bash scripts/android_dual_fast_check.sh --phase2-only
  bash scripts/android_dual_fast_check.sh --phase2-only --keep-work-dir
  SLAN_ANDROID_DEVICE_A=emulator-5554 SLAN_ANDROID_DEVICE_B=emulator-5556 bash scripts/android_dual_fast_check.sh
EOF
  exit 0
fi

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
export SLAN_ANDROID_DEVICE_A="${SLAN_ANDROID_DEVICE_A:-emulator-5554}"
export SLAN_ANDROID_DEVICE_B="${SLAN_ANDROID_DEVICE_B:-emulator-5556}"
export SLAN_ANDROID_DEVICE_ID_A="${SLAN_ANDROID_DEVICE_ID_A:-11111111111141118111111111111111}"
export SLAN_ANDROID_DEVICE_ID_B="${SLAN_ANDROID_DEVICE_ID_B:-22222222222242228222222222222222}"
export SLAN_ANDROID_AVD_A="${SLAN_ANDROID_AVD_A:-Pixel_3a_API_34_extension_level_7_arm64-v8a}"
export SLAN_ANDROID_AVD_B="${SLAN_ANDROID_AVD_B:-Pixel_3a_API_34_extension_level_7_arm64-v8a_dual2}"
export SLAN_ANDROID_DEBUG_EMULATOR_VPN_BYPASS="${SLAN_ANDROID_DEBUG_EMULATOR_VPN_BYPASS:-1}"
export SLAN_ANDROID_DUAL_TIMEOUT="${SLAN_ANDROID_DUAL_TIMEOUT:-10m}"
export SLAN_ANDROID_DUAL_PHASE1_MAX_ATTEMPTS="${SLAN_ANDROID_DUAL_PHASE1_MAX_ATTEMPTS:-2}"
export SLAN_ANDROID_DUAL_PHASE1_SETTLE_SECONDS="${SLAN_ANDROID_DUAL_PHASE1_SETTLE_SECONDS:-2}"
export SLAN_ANDROID_DUAL_PHASE1_DEVICE_ID_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE1_DEVICE_ID_WAIT_SECONDS:-180}"
export SLAN_ANDROID_DUAL_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS="${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS:-60}"
export SLAN_ANDROID_DUAL_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS="${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS:-180}"
export SLAN_ANDROID_DUAL_PHASE34_RECEIVER_HOLD_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_RECEIVER_HOLD_SECONDS:-240}"
export SLAN_ANDROID_DUAL_PHASE34_MARKER_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_MARKER_WAIT_SECONDS:-300}"
export SLAN_ANDROID_DUAL_PHASE34_RELAY_REFRESH_WAIT_SECONDS="${SLAN_ANDROID_DUAL_PHASE34_RELAY_REFRESH_WAIT_SECONDS:-120}"
export SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST="${SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST:-udp}"
export SLAN_KEEP_ANDROID_DUAL_WORK_DIR="${SLAN_KEEP_ANDROID_DUAL_WORK_DIR:-${SLAN_ANDROID_DUAL_KEEP_WORK_DIR:-0}}"

if [[ -n "$KEEP_WORK_DIR_OVERRIDE" ]]; then
  export SLAN_KEEP_ANDROID_DUAL_WORK_DIR="$KEEP_WORK_DIR_OVERRIDE"
fi

if [[ "$MODE" == "--phase2-only" ]]; then
  export SLAN_ANDROID_DUAL_STOP_AFTER_PHASE2="${SLAN_ANDROID_DUAL_STOP_AFTER_PHASE2:-1}"
  export SLAN_KEEP_ANDROID_DUAL_WORK_DIR="${SLAN_KEEP_ANDROID_DUAL_WORK_DIR:-1}"
fi

echo "==> android dual fast uses adb serials: ${SLAN_ANDROID_DEVICE_A}, ${SLAN_ANDROID_DEVICE_B}"
echo "==> stable device ids: ${SLAN_ANDROID_DEVICE_ID_A}, ${SLAN_ANDROID_DEVICE_ID_B}"
echo "==> recommended AVDs: ${SLAN_ANDROID_AVD_A}, ${SLAN_ANDROID_AVD_B}"
echo "==> stable mode: phase2 mqtt timeout=${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MQTT_TIMEOUT_SECONDS}s message timeout=${SLAN_ANDROID_DUAL_PHASE2_EXPECT_MESSAGE_TIMEOUT_SECONDS}s"
echo "==> stable mode: phase3/4 hold=${SLAN_ANDROID_DUAL_PHASE34_RECEIVER_HOLD_SECONDS}s marker wait=${SLAN_ANDROID_DUAL_PHASE34_MARKER_WAIT_SECONDS}s relay refresh wait=${SLAN_ANDROID_DUAL_PHASE34_RELAY_REFRESH_WAIT_SECONDS}s"
echo "==> stable mode: phase3/4 relay transport allowlist=${SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST}"
if [[ "$MODE" == "--phase2-only" ]]; then
  echo "==> mode: phase2-only reverse-message debug with retained work dir"
else
  echo "==> mode: full stable chain"
fi
echo "==> keep work dir: ${SLAN_KEEP_ANDROID_DUAL_WORK_DIR}"
echo "==> if devices are not booted, run: bash scripts/start_android_dual_avds.sh"

exec bash "$ROOT_DIR/scripts/android_dual_emulator_integration.sh"
