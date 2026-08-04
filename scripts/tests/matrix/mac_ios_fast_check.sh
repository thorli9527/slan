#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'EOF'
Usage:
  bash scripts/mac_ios_fast_check.sh

Purpose:
  Run the stable Mac <-> iOS fast validation using the already-installed
  privileged macOS client-core-service and the known-good iOS defaults.

Examples:
  bash scripts/mac_ios_fast_check.sh

Real device UDP/TCP helper:
  SLAN_IOS_FLUTTER_DEVICE="<real ios device id or name>" bash scripts/mac_ios_real_device_socket_check.sh

Optional environment variables:
  SLAN_RUN_MAC_LOCAL_DNS_SMOKE=1|0
EOF
  exit 0
fi

export SLAN_BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
export SLAN_WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
export SLAN_MAC_IOS_TIMEOUT="${SLAN_MAC_IOS_TIMEOUT:-60s}"
export SLAN_RUN_IOS_APP_DNS_ACL_SMOKE="${SLAN_RUN_IOS_APP_DNS_ACL_SMOKE:-1}"
export SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES="${SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES:-1}"
export SLAN_IOS_APP_DNS_ACL_CLIENTS="${SLAN_IOS_APP_DNS_ACL_CLIENTS:-2}"
export SLAN_RUN_MAC_LOCAL_DNS_SMOKE="${SLAN_RUN_MAC_LOCAL_DNS_SMOKE:-1}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
export SLAN_CLIENT_CORE_SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"

echo "==> mac/ios fast uses biz=${SLAN_BIZ_URL}"
echo "==> ios flutter device: ${SLAN_IOS_FLUTTER_DEVICE:-auto-detect-booted-simulator}"
echo "==> run ios dns/acl smoke: ${SLAN_RUN_IOS_APP_DNS_ACL_SMOKE}"
echo "==> run mac local dns smoke: ${SLAN_RUN_MAC_LOCAL_DNS_SMOKE}"
echo "==> mac service binary: ${SLAN_CLIENT_CORE_SERVICE_BIN}"

exec bash "$ROOT_DIR/scripts/mac_ios_integration_check.sh"
