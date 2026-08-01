#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

collect_html_methods() {
  local root="$1"
  rg -o 'vm\.[A-Za-z0-9_]+\(' "$root"/**/*.html -N \
    | sed 's/.*vm\.//; s/(//' \
    | sort -u
}

collect_ts_methods() {
  local root="$1"
  rg -n '^[[:space:]]*(async[[:space:]]+)?[A-Za-z0-9_]+\(' "$root" -g '*.ts' -N \
    | sed -E 's#.*[[:space:]]([A-Za-z0-9_]+)\(.*#\1#' \
    | sort -u
}

audit_project() {
  local name="$1"
  local root="$2"
  local html_methods
  local ts_methods
  local missing

  html_methods="$(collect_html_methods "$root" || true)"
  ts_methods="$(collect_ts_methods "$root" || true)"
  missing="$(comm -23 <(printf '%s\n' "$html_methods") <(printf '%s\n' "$ts_methods") || true)"

  if [ -n "${missing//$'\n'/}" ]; then
    echo "ui template audit failed for ${name}:"
    printf '%s\n' "$missing"
    return 1
  fi

  echo "ui template audit ok: ${name}"
}

audit_project "opt-ui" "$ROOT_DIR/server/opt-ui/src"
