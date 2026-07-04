#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

RUN_ANDROID_DUAL="${SLAN_RUN_STABLE_ANDROID_DUAL:-1}"
RUN_IOS_DUAL="${SLAN_RUN_STABLE_IOS_DUAL:-1}"
RUN_MAC_ANDROID="${SLAN_RUN_STABLE_MAC_ANDROID:-1}"
RUN_MAC_IOS="${SLAN_RUN_STABLE_MAC_IOS:-1}"
RUN_MAC_ACTIVE_MATRIX="${SLAN_RUN_STABLE_MAC_ACTIVE_MATRIX:-1}"
RUN_MAC_ANDROID_ACTIVE="${SLAN_RUN_MAC_ANDROID_ACTIVE:-1}"
RUN_MAC_IOS_ACTIVE="${SLAN_RUN_MAC_IOS_ACTIVE:-0}"
RUN_MAC_LINUX_ACTIVE="${SLAN_RUN_MAC_LINUX_ACTIVE:-1}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/mac_android_ios_stable_check.sh

Purpose:
  Run the stable dual-Android, dual-iOS, Mac <-> Android, and Mac <-> iOS
  validation wrappers in one pass, using the shared remote environment
  defaults and the installed macOS privileged client-core-service.

Optional environment variables:
  SLAN_RUN_STABLE_ANDROID_DUAL=1|0
  SLAN_RUN_STABLE_IOS_DUAL=1|0
  SLAN_RUN_STABLE_MAC_ANDROID=1|0
  SLAN_RUN_STABLE_MAC_IOS=1|0
  SLAN_RUN_STABLE_MAC_ACTIVE_MATRIX=1|0
  SLAN_RUN_MAC_ANDROID_ACTIVE=1|0
  SLAN_RUN_MAC_IOS_ACTIVE=1|0
  SLAN_RUN_MAC_LINUX_ACTIVE=1|0
  SLAN_BIZ_URL
  SLAN_WEB_BASE_URL
  SLAN_SUDO_PASSWORD
  SLAN_ANDROID_FLUTTER_DEVICE
  SLAN_IOS_FLUTTER_DEVICE
  SLAN_SKIP_ANDROID_BUILD=1|0

Examples:
  bash scripts/mac_android_ios_stable_check.sh
  SLAN_RUN_STABLE_ANDROID_DUAL=0 bash scripts/mac_android_ios_stable_check.sh
  SLAN_RUN_STABLE_IOS_DUAL=0 bash scripts/mac_android_ios_stable_check.sh
  SLAN_RUN_STABLE_MAC_IOS=0 bash scripts/mac_android_ios_stable_check.sh
  SLAN_RUN_STABLE_MAC_ANDROID=0 bash scripts/mac_android_ios_stable_check.sh
  SLAN_RUN_STABLE_MAC_ACTIVE_MATRIX=0 bash scripts/mac_android_ios_stable_check.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"

log "stable matrix toggles: android-dual=${RUN_ANDROID_DUAL} ios-dual=${RUN_IOS_DUAL} mac-android=${RUN_MAC_ANDROID} mac-ios=${RUN_MAC_IOS} mac-active-matrix=${RUN_MAC_ACTIVE_MATRIX}"
log "biz url: ${SLAN_BIZ_URL}"

if [[ "$RUN_ANDROID_DUAL" == "1" ]]; then
  log "Run stable dual Android validation"
  (
    cd "$ROOT_DIR"
    bash scripts/android_dual_fast_check.sh
  )
fi

if [[ "$RUN_IOS_DUAL" == "1" ]]; then
  log "Run stable dual iOS validation"
  (
    cd "$ROOT_DIR"
    bash scripts/ios_dual_fast_check.sh
  )
fi

if [[ "$RUN_MAC_ANDROID" == "1" ]]; then
  log "Run stable Mac + Android validation"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_android_fast_check.sh
  )
fi

if [[ "$RUN_MAC_IOS" == "1" ]]; then
  log "Run stable Mac + iOS validation"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_ios_fast_check.sh
  )
fi

if [[ "$RUN_MAC_ACTIVE_MATRIX" == "1" ]]; then
  log "Run stable macOS-originated active socket matrix"
  (
    cd "$ROOT_DIR"
    SLAN_RUN_MAC_ANDROID_ACTIVE="$RUN_MAC_ANDROID_ACTIVE" \
      SLAN_RUN_MAC_IOS_ACTIVE="$RUN_MAC_IOS_ACTIVE" \
      SLAN_RUN_MAC_LINUX_ACTIVE="$RUN_MAC_LINUX_ACTIVE" \
      bash scripts/mac_active_socket_matrix.sh
  )
fi

log "stable multi-client validation complete"
