#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

RUN_ANDROID="${SLAN_RUN_LOCAL_ANDROID_QUICK:-1}"
RUN_IOS="${SLAN_RUN_LOCAL_IOS_QUICK:-1}"
RUN_MAC_ANDROID="${SLAN_RUN_LOCAL_MAC_ANDROID_QUICK:-0}"
RUN_MAC_IOS="${SLAN_RUN_LOCAL_MAC_IOS_QUICK:-0}"
IOS_MODE="${SLAN_LOCAL_IOS_QUICK_MODE:-default}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/local_client_quick_check.sh

Optional environment variables:
  SLAN_RUN_LOCAL_ANDROID_QUICK=1|0
  SLAN_RUN_LOCAL_IOS_QUICK=1|0
  SLAN_RUN_LOCAL_MAC_ANDROID_QUICK=1|0
  SLAN_RUN_LOCAL_MAC_IOS_QUICK=1|0
  SLAN_LOCAL_IOS_QUICK_MODE=default|quick-only|message-only

Examples:
  bash scripts/local_client_quick_check.sh
  SLAN_RUN_LOCAL_ANDROID_QUICK=1 SLAN_RUN_LOCAL_IOS_QUICK=0 bash scripts/local_client_quick_check.sh
  SLAN_RUN_LOCAL_ANDROID_QUICK=0 SLAN_RUN_LOCAL_IOS_QUICK=1 bash scripts/local_client_quick_check.sh
  SLAN_LOCAL_IOS_QUICK_MODE=message-only bash scripts/local_client_quick_check.sh
  SLAN_RUN_LOCAL_MAC_ANDROID_QUICK=1 SLAN_RUN_LOCAL_IOS_QUICK=0 bash scripts/local_client_quick_check.sh
  SLAN_RUN_LOCAL_MAC_IOS_QUICK=1 SLAN_RUN_LOCAL_ANDROID_QUICK=0 bash scripts/local_client_quick_check.sh

Related helpers:
  bash scripts/setup_macos_client.sh
  bash scripts/start_android_dual_avds.sh
  bash scripts/stop_android_dual_avds.sh
  bash scripts/android_dual_fast_check.sh
  bash scripts/start_ios_dual_sims.sh
  bash scripts/stop_ios_dual_sims.sh
  bash scripts/mac_android_fast_check.sh
  bash scripts/mac_ios_fast_check.sh
  bash scripts/mac_android_ios_stable_check.sh
  bash scripts/local_client_quick_cleanup.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

log "local quick toggles: android=${RUN_ANDROID} ios=${RUN_IOS} ios-mode=${IOS_MODE} mac-android=${RUN_MAC_ANDROID} mac-ios=${RUN_MAC_IOS}"
log "use --help for examples and platform-only variants"

run_android() {
  log "Start Android dual AVDs"
  (
    cd "$ROOT_DIR"
    bash scripts/start_android_dual_avds.sh
  )
  log "Run Android dual fast validation"
  (
    cd "$ROOT_DIR"
    bash scripts/android_dual_fast_check.sh
  )
}

run_ios() {
  local mode_arg=""
  case "$IOS_MODE" in
    default)
      ;;
    quick-only|message-only)
      mode_arg="--${IOS_MODE}"
      ;;
    *)
      printf 'unsupported SLAN_LOCAL_IOS_QUICK_MODE: %s\n' "$IOS_MODE" >&2
      exit 1
      ;;
  esac
  log "Run iOS dual fast validation"
  (
    cd "$ROOT_DIR"
    bash scripts/ios_dual_fast_check.sh ${mode_arg:+"$mode_arg"}
  )
}

run_mac_android() {
  log "Run Mac + Android quick validation"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_android_fast_check.sh
  )
}

run_mac_ios() {
  log "Run Mac + iOS quick validation"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_ios_fast_check.sh
  )
}

if [[ "$RUN_ANDROID" == "1" ]]; then
  run_android
fi

if [[ "$RUN_IOS" == "1" ]]; then
  run_ios
fi

if [[ "$RUN_MAC_ANDROID" == "1" ]]; then
  run_mac_android
fi

if [[ "$RUN_MAC_IOS" == "1" ]]; then
  run_mac_ios
fi

log "local client quick validation complete"
