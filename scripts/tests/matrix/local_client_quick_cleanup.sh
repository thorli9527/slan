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

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/local_client_quick_cleanup.sh

Optional environment variables:
  SLAN_RUN_LOCAL_ANDROID_QUICK=1|0
  SLAN_RUN_LOCAL_IOS_QUICK=1|0

Examples:
  bash scripts/local_client_quick_cleanup.sh
  SLAN_RUN_LOCAL_ANDROID_QUICK=1 SLAN_RUN_LOCAL_IOS_QUICK=0 bash scripts/local_client_quick_cleanup.sh
  SLAN_RUN_LOCAL_ANDROID_QUICK=0 SLAN_RUN_LOCAL_IOS_QUICK=1 bash scripts/local_client_quick_cleanup.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

log "local cleanup toggles: android=${RUN_ANDROID} ios=${RUN_IOS}"

if [[ "$RUN_ANDROID" == "1" ]]; then
  log "Stop Android dual AVDs"
  (
    cd "$ROOT_DIR"
    bash scripts/stop_android_dual_avds.sh
  )
fi

if [[ "$RUN_IOS" == "1" ]]; then
  log "Stop iOS dual simulators"
  (
    cd "$ROOT_DIR"
    bash scripts/stop_ios_dual_sims.sh
  )
fi

log "local client quick cleanup complete"
