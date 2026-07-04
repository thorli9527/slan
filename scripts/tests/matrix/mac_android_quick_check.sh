#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

# Thin wrapper around the full Mac<->Android socket/data-plane check with the
# remote Docker and local host defaults that have been used elsewhere.
export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
export SLAN_ANDROID_BIZ_URL="${SLAN_ANDROID_BIZ_URL:-$SLAN_BIZ_URL}"
export SLAN_ANDROID_FLUTTER_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
export SLAN_MAC_ANDROID_SOCKET_TIMEOUT="${SLAN_MAC_ANDROID_SOCKET_TIMEOUT:-90s}"
export SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_SEND_POST_ENABLE_WAIT_SECONDS:-8}"
export SLAN_MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
export SLAN_SKIP_ANDROID_BUILD="${SLAN_SKIP_ANDROID_BUILD:-0}"

echo "==> mac/android quick uses biz=${SLAN_BIZ_URL}"
echo "==> android device: ${SLAN_ANDROID_FLUTTER_DEVICE}"
echo "==> macOS network mock: ${SLAN_MACOS_NETWORK_MOCK} (0 means real data-plane)"
echo "==> if you only want control-plane/message validation, set SLAN_MACOS_NETWORK_MOCK=1"

exec bash "$ROOT_DIR/scripts/mac_android_socket_check.sh"
