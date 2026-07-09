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
  bash scripts/mac_android_fast_check.sh
  bash scripts/mac_android_fast_check.sh --full-stable

Purpose:
  Run the stable Mac <-> Android fast validation using the already-installed
  privileged macOS client-core-service and the known-good Android defaults.

Optional environment variables:
  SLAN_BIZ_URL
  SLAN_WEB_BASE_URL
  SLAN_ANDROID_FLUTTER_DEVICE
  SLAN_MAC_SERVICE_MODE=existing|app
  SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY=1|0
    Default is `1` so the installed macOS service auto-recovers from stale
    local session/identity state before the socket smoke runs.
  SLAN_SKIP_ANDROID_BUILD=1|0
  SLAN_MACOS_NETWORK_MOCK=1|0
  SLAN_MAC_ANDROID_SOCKET_TIMEOUT
  SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS
  SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST
    Default is `udp` for the stable mixed-client path.
  SLAN_KEEP_MAC_ANDROID_SOCKET_WORK_DIR=1|0

Examples:
  bash scripts/mac_android_fast_check.sh
  bash scripts/mac_android_fast_check.sh --full-stable
EOF
  exit 0
fi

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
export SLAN_ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-$SLAN_BIZ_URL}"
export SLAN_ANDROID_FLUTTER_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
export SLAN_MAC_SERVICE_MODE="${SLAN_MAC_SERVICE_MODE:-existing}"
export SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
export SLAN_SKIP_ANDROID_BUILD="${SLAN_SKIP_ANDROID_BUILD:-1}"
export SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
export SLAN_MAC_ANDROID_SOCKET_TIMEOUT="${SLAN_MAC_ANDROID_SOCKET_TIMEOUT:-90s}"
export SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-35}"
export SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST:-udp}"
export SLAN_KEEP_MAC_ANDROID_SOCKET_WORK_DIR="${SLAN_KEEP_MAC_ANDROID_SOCKET_WORK_DIR:-0}"

echo "==> mac/android fast uses biz=${SLAN_BIZ_URL}"
echo "==> android device: ${SLAN_ANDROID_FLUTTER_DEVICE}"
echo "==> mac service mode: ${SLAN_MAC_SERVICE_MODE}"
echo "==> reset existing mac identity: ${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY}"
echo "==> skip android build: ${SLAN_SKIP_ANDROID_BUILD}"
echo "==> network mock: ${SLAN_MACOS_NETWORK_MOCK} (0 means real data-plane)"
echo "==> stable mode: timeout=${SLAN_MAC_ANDROID_SOCKET_TIMEOUT} android post-enable wait=${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS}s"
echo "==> stable mode: relay transport allowlist=${SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST}"

exec bash "$ROOT_DIR/scripts/mac_android_socket_check.sh"
