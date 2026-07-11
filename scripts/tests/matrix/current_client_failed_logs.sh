#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

MODE="full"
TAIL_LINES="${SLAN_CURRENT_FAILED_LOG_TAIL_LINES:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    full|quick)
      MODE="$1"
      ;;
    --tail)
      shift
      [[ $# -gt 0 ]] || {
        printf 'missing value for --tail\n' >&2
        exit 1
      }
      TAIL_LINES="$1"
      ;;
    -h|--help)
      cat <<'EOF'
Usage:
  bash scripts/current_client_failed_logs.sh [full|quick] [--tail N]

Purpose:
  Print failed step names and their log paths from the latest regression
  summary.

Examples:
  bash scripts/current_client_failed_logs.sh
  bash scripts/current_client_failed_logs.sh quick
  bash scripts/current_client_failed_logs.sh full --tail 80
EOF
      exit 0
      ;;
    *)
      printf 'unsupported argument: %s\n' "$1" >&2
      printf 'use: [full|quick] [--tail N]\n' >&2
      exit 1
      ;;
  esac
  shift
done

case "$MODE" in
  full)
    RESULT_ROOT="${SLAN_CURRENT_REGRESSION_RESULT_ROOT:-${TMPDIR:-/tmp}/slan-current-regression}"
    LATEST_LINK="${SLAN_CURRENT_REGRESSION_LATEST_LINK:-$RESULT_ROOT/latest}"
    SUMMARY_FILE="${SLAN_CURRENT_REGRESSION_SUMMARY_FILE:-$LATEST_LINK/summary.txt}"
    ;;
  quick)
    RESULT_ROOT="${SLAN_CURRENT_QUICK_REGRESSION_RESULT_ROOT:-${SLAN_CURRENT_REGRESSION_RESULT_ROOT:-${TMPDIR:-/tmp}/slan-current-quick-regression}}"
    LATEST_LINK="${SLAN_CURRENT_QUICK_REGRESSION_LATEST_LINK:-${SLAN_CURRENT_REGRESSION_LATEST_LINK:-$RESULT_ROOT/latest}}"
    SUMMARY_FILE="${SLAN_CURRENT_QUICK_REGRESSION_SUMMARY_FILE:-$LATEST_LINK/summary.txt}"
    ;;
esac

if [[ ! "$TAIL_LINES" =~ ^[0-9]+$ ]]; then
  printf 'invalid --tail value: %s\n' "$TAIL_LINES" >&2
  exit 1
fi

if [[ ! -f "$SUMMARY_FILE" ]]; then
  printf 'missing regression summary: %s\n' "$SUMMARY_FILE" >&2
  if [[ "$MODE" == "quick" ]]; then
    printf 'run: bash scripts/current_client_quick_regression.sh\n' >&2
  else
    printf 'run: bash scripts/current_client_regression.sh\n' >&2
  fi
  exit 1
fi

failed_logs=()
while IFS= read -r line; do
  failed_logs+=("$line")
done < <(awk '
  /^\[RUN \]/ {
    current=$0
    sub(/^\[RUN \] /, "", current)
    next
  }
  /^       log=/ {
    current_log=$0
    sub(/^       log=/, "", current_log)
    next
  }
  /^\[FAIL\]/ {
    failed=$0
    sub(/^\[FAIL\] /, "", failed)
    sub(/ exit=.*/, "", failed)
    printf "FAIL %s\n", failed
    if (current_log != "") {
      printf "LOG  %s\n", current_log
    }
    print ""
  }
' "$SUMMARY_FILE")

if [[ "${#failed_logs[@]}" -eq 0 ]]; then
  printf 'no failed steps found in latest %s regression summary\n' "$MODE"
  exit 0
fi

printf '%s\n' "${failed_logs[@]}"

if [[ "$TAIL_LINES" -gt 0 ]]; then
  current_log=""
  while IFS= read -r line; do
    if [[ "$line" == LOG\ * ]]; then
      current_log="${line#LOG  }"
      if [[ -f "$current_log" ]]; then
        printf '\n==> tail -n %s %s\n' "$TAIL_LINES" "$current_log"
        tail -n "$TAIL_LINES" "$current_log"
      else
        printf '\nmissing log file: %s\n' "$current_log" >&2
      fi
    fi
  done < <(printf '%s\n' "${failed_logs[@]}")
fi
