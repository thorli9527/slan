#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-${SLAN_BIZ_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}}"
DEFAULT_SERVICE_BIN="$ROOT_DIR/client/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_SERVICE_BIN" ]]; then
  DEFAULT_SERVICE_BIN="$ROOT_DIR/client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_SERVICE_BIN}"
TIMEOUT="${SLAN_IOS_APP_DNS_ACL_TIMEOUT:-75s}"
EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-}"
CHECK_MESSAGES="${SLAN_IOS_APP_DNS_ACL_CHECK_MESSAGES:-1}"
CLIENTS="${SLAN_IOS_APP_DNS_ACL_CLIENTS:-2}"

ARGS=(
  -biz-url "${BIZ_URL}"
  -web-base-url "${WEB_BASE_URL}"
  -service-bin "${SERVICE_BIN}"
  -timeout "${TIMEOUT}"
  -clients "${CLIENTS}"
)

if [[ -n "${EXPECT_MQTT_HOST}" ]]; then
  ARGS+=(-expect-mqtt-host "${EXPECT_MQTT_HOST}")
fi

ARGS+=(-check-messages="${CHECK_MESSAGES}")

cd "${ROOT_DIR}"
go run ./scripts/ios_dual_acl_dns_integration.go "${ARGS[@]}"
