#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

RESULT_ROOT="${SLAN_CURRENT_REGRESSION_RESULT_ROOT:-${TMPDIR:-/tmp}/slan-current-regression}"
LATEST_LINK="${SLAN_CURRENT_REGRESSION_LATEST_LINK:-$RESULT_ROOT/latest}"
SUMMARY_FILE="${SLAN_CURRENT_REGRESSION_SUMMARY_FILE:-$LATEST_LINK/summary.txt}"
SHOW_LOG_DIRS="${SLAN_CURRENT_REGRESSION_SHOW_LOG_DIRS:-0}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/current_client_regression_summary.sh

Purpose:
  Print the latest full current client regression summary.

Optional environment variables:
  SLAN_CURRENT_REGRESSION_RESULT_ROOT
  SLAN_CURRENT_REGRESSION_LATEST_LINK
  SLAN_CURRENT_REGRESSION_SUMMARY_FILE
  SLAN_CURRENT_REGRESSION_SHOW_LOG_DIRS=1|0

Examples:
  bash scripts/current_client_regression_summary.sh
  SLAN_CURRENT_REGRESSION_SHOW_LOG_DIRS=1 bash scripts/current_client_regression_summary.sh
EOF
  exit 0
fi

if [[ ! -f "$SUMMARY_FILE" ]]; then
  printf 'missing current regression summary: %s\n' "$SUMMARY_FILE" >&2
  printf 'run: bash scripts/current_client_regression.sh\n' >&2
  exit 1
fi

cat "$SUMMARY_FILE"

if [[ "$SHOW_LOG_DIRS" == "1" ]]; then
  printf '\n'
  printf 'log files:\n'
  sed -n 's/^       log=//p' "$SUMMARY_FILE"
fi
