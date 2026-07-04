#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

RUN_MAC_ANDROID_ACTIVE="${SLAN_RUN_MAC_ANDROID_ACTIVE:-1}"
RUN_MAC_IOS_ACTIVE="${SLAN_RUN_MAC_IOS_ACTIVE:-1}"
RUN_MAC_LINUX_ACTIVE="${SLAN_RUN_MAC_LINUX_ACTIVE:-1}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/mac_active_socket_matrix.sh

Purpose:
  Run the macOS-originated active socket checks against Android, iOS,
  and/or Linux Docker peers using the stabilized readiness checks.

Optional environment variables:
  SLAN_RUN_MAC_ANDROID_ACTIVE=1|0
  SLAN_RUN_MAC_IOS_ACTIVE=1|0
  SLAN_RUN_MAC_LINUX_ACTIVE=1|0
  SLAN_BIZ_URL
  SLAN_SUDO_PASSWORD
  SLAN_ANDROID_FLUTTER_DEVICE
  SLAN_IOS_FLUTTER_DEVICE

Examples:
  bash scripts/mac_active_socket_matrix.sh
  SLAN_RUN_MAC_IOS_ACTIVE=0 bash scripts/mac_active_socket_matrix.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

log "default control url: ${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"

if [[ "$RUN_MAC_ANDROID_ACTIVE" == "1" ]]; then
  log "Run macOS -> Android active socket check"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_android_active_socket_check.sh
  )
fi

if [[ "$RUN_MAC_IOS_ACTIVE" == "1" ]]; then
  log "Run macOS -> iOS active socket check"
  (
    cd "$ROOT_DIR"
    bash scripts/mac_ios_active_socket_check.sh
  )
fi

if [[ "$RUN_MAC_LINUX_ACTIVE" == "1" ]]; then
  log "Run macOS <-> Linux Docker active socket check"
  (
    cd "$ROOT_DIR"
    bash scripts/linux_docker_mac_integration.sh
  )
fi

log "mac active socket matrix complete"
