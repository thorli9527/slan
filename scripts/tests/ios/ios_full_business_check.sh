#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

RUN_START_SIMS="${SLAN_RUN_IOS_START_SIMS:-1}"
RUN_DUAL_QUICK="${SLAN_RUN_IOS_DUAL_QUICK:-1}"
RUN_DUAL_FLUTTER_MESSAGE="${SLAN_RUN_IOS_DUAL_FLUTTER_MESSAGE:-1}"
RUN_MAC_IOS="${SLAN_RUN_IOS_MAC_INTEGRATION:-0}"
RUN_IOS_ANDROID="${SLAN_RUN_IOS_ANDROID_SOCKET_CHECK:-0}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/ios_full_business_check.sh

Optional environment variables:
  SLAN_RUN_IOS_START_SIMS=1|0
  SLAN_RUN_IOS_DUAL_QUICK=1|0
  SLAN_RUN_IOS_DUAL_FLUTTER_MESSAGE=1|0
  SLAN_RUN_IOS_MAC_INTEGRATION=1|0
  SLAN_RUN_IOS_ANDROID_SOCKET_CHECK=1|0

Examples:
  bash scripts/ios_full_business_check.sh
  SLAN_RUN_IOS_MAC_INTEGRATION=1 bash scripts/ios_full_business_check.sh
  SLAN_RUN_IOS_ANDROID_SOCKET_CHECK=1 bash scripts/ios_full_business_check.sh

Real device helper:
  SLAN_IOS_FLUTTER_DEVICE="<real ios device id or name>" bash scripts/ios_real_device_socket_check.sh

Cleanup helper:
  bash scripts/ios_full_business_cleanup.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

log "ios business toggles: start_sims=${RUN_START_SIMS} dual_quick=${RUN_DUAL_QUICK} dual_flutter_message=${RUN_DUAL_FLUTTER_MESSAGE} mac_ios=${RUN_MAC_IOS} ios_android=${RUN_IOS_ANDROID}"

run_in_root() {
  (
    cd "$ROOT_DIR"
    "$@"
  )
}

if [[ "$RUN_START_SIMS" == "1" ]]; then
  log "Start iOS dual simulators"
  run_in_root bash scripts/start_ios_dual_sims.sh
fi

if [[ "$RUN_DUAL_QUICK" == "1" ]]; then
  log "Run iOS dual fast quick validation"
  run_in_root bash scripts/ios_dual_fast_check.sh --quick-only
fi

if [[ "$RUN_DUAL_FLUTTER_MESSAGE" == "1" ]]; then
  log "Run dual iOS Flutter message validation"
  run_in_root bash scripts/ios_dual_flutter_message_check.sh
fi

if [[ "$RUN_MAC_IOS" == "1" ]]; then
  log "Run Mac + iOS business integration"
  run_in_root bash scripts/mac_ios_fast_check.sh
fi

if [[ "$RUN_IOS_ANDROID" == "1" ]]; then
  log "Run iOS + Android socket business validation"
  run_in_root bash scripts/ios_android_socket_check.sh
fi

log "iOS full business validation complete"
