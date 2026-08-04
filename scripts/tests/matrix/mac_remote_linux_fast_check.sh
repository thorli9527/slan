#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

MODE="${1:-}"

case "$MODE" in
  ""|--full-stable )
    ;;
  -h|--help )
    ;;
  * )
    printf 'unknown option: %s\n' "$MODE" >&2
    printf 'run with --help for usage\n' >&2
    exit 1
    ;;
esac

if [[ "$MODE" == "-h" || "$MODE" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/tests/matrix/mac_remote_linux_fast_check.sh
  bash scripts/tests/matrix/mac_remote_linux_fast_check.sh --full-stable

Purpose:
  Run the stable Mac <-> remote Linux fast validation using the already-installed
  privileged macOS client-core-service and a reachable remote Linux host.

Required environment variables:
  SLAN_REMOTE_LINUX_HOST
  SLAN_REMOTE_LINUX_USER
  Either:
    SLAN_REMOTE_LINUX_PASSWORD
  Or:
    SLAN_REMOTE_LINUX_SSH_KEY

Optional environment variables:
  SLAN_BIZ_URL
  SLAN_WEB_BASE_URL
  SLAN_MAC_SERVICE_MODE=existing|app|service
  SLAN_MAC_SERVICE_HOST
  SLAN_CLIENT_CORE_SERVICE_BIN
  SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY=1|0
    Default is `1` so the installed macOS service auto-recovers from stale
    local session/identity state before the mixed remote socket smoke runs.
  SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK=1|0
    Default is `1` so remote install/bootstrap health is verified first.
  SLAN_KEEP_MAC_REMOTE_LINUX_WORK_DIR=1|0

Examples:
  SLAN_REMOTE_LINUX_HOST=100.87.66.24 \
  SLAN_REMOTE_LINUX_USER=root \
  SLAN_REMOTE_LINUX_PASSWORD=secret \
  bash scripts/tests/matrix/mac_remote_linux_fast_check.sh
EOF
  exit 0
fi

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
export SLAN_MAC_SERVICE_MODE="${SLAN_MAC_SERVICE_MODE:-existing}"
export SLAN_MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST:-127.0.0.1:46392}"
export SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
export SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK="${SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK:-1}"
export SLAN_KEEP_MAC_REMOTE_LINUX_WORK_DIR="${SLAN_KEEP_MAC_REMOTE_LINUX_WORK_DIR:-0}"

echo "==> mac/remote-linux fast uses biz=${SLAN_BIZ_URL}"
echo "==> remote linux host: ${SLAN_REMOTE_LINUX_HOST:-<unset>}"
echo "==> remote linux user: ${SLAN_REMOTE_LINUX_USER:-<unset>}"
echo "==> mac service mode: ${SLAN_MAC_SERVICE_MODE}"
echo "==> reset existing mac identity: ${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY}"
echo "==> run remote install preflight: ${SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK}"

exec bash "$ROOT_DIR/scripts/tests/matrix/mac_remote_linux_integration.sh"
