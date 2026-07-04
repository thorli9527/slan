#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

RUN_STOP_SIMS="${SLAN_RUN_IOS_STOP_SIMS:-1}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/ios_full_business_cleanup.sh

Optional environment variables:
  SLAN_RUN_IOS_STOP_SIMS=1|0

Examples:
  bash scripts/ios_full_business_cleanup.sh
  SLAN_RUN_IOS_STOP_SIMS=0 bash scripts/ios_full_business_cleanup.sh
EOF
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

log "ios cleanup toggles: stop_sims=${RUN_STOP_SIMS}"

run_in_root() {
  (
    cd "$ROOT_DIR"
    "$@"
  )
}

if [[ "$RUN_STOP_SIMS" == "1" ]]; then
  log "Stop iOS dual simulators"
  run_in_root bash scripts/stop_ios_dual_sims.sh
fi

log "iOS full business cleanup complete"
