#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

MODE="${1:-}"

case "$MODE" in
  "" )
    ;;
  -h|--help )
    cat <<'EOF'
Usage:
  bash scripts/android_dual_phase2_reverse_debug.sh

Purpose:
  Run the dual-Android flow only up to phase2 reverse client-message delivery.
  This is the fastest way to debug the flaky reverse message path with the
  extra embedded Rust logs enabled.

Notes:
  - Keeps the work dir by default.
  - Skips phase3/phase4 socket checks entirely.
EOF
    exit 0
    ;;
  * )
    printf 'unknown option: %s\n' "$MODE" >&2
    exit 1
    ;;
esac

export SLAN_KEEP_ANDROID_DUAL_WORK_DIR="${SLAN_KEEP_ANDROID_DUAL_WORK_DIR:-1}"
export SLAN_ANDROID_DUAL_STOP_AFTER_PHASE2="${SLAN_ANDROID_DUAL_STOP_AFTER_PHASE2:-1}"

exec bash "$ROOT_DIR/scripts/android_dual_emulator_integration.sh"
