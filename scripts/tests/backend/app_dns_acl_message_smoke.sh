#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
if [[ -n "${SLAN_WEB_BASE_URL:-}" ]]; then
  WEB_BASE_URL="$SLAN_WEB_BASE_URL"
elif [[ -n "${SLAN_BIZ_WEB_BASE_URL:-}" ]]; then
  WEB_BASE_URL="$SLAN_BIZ_WEB_BASE_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  WEB_BASE_URL="${BIZ_URL%:28080}:28081"
else
  WEB_BASE_URL="$SLAN_DEFAULT_WEB_BASE_URL"
fi
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-client/rust/target/release/client-core-service}"
TIMEOUT="${SLAN_APP_DNS_ACL_TIMEOUT:-90s}"
EXPECT_MQTT_HOST="${SLAN_EXPECT_MQTT_HOST:-$SLAN_DEFAULT_MQTT_HOST}"
CHECK_MESSAGES="${SLAN_APP_DNS_ACL_CHECK_MESSAGES:-1}"

ARGS=(
  -biz-url "${BIZ_URL}"
  -web-base-url "${WEB_BASE_URL}"
  -service-bin "${SERVICE_BIN}"
  -timeout "${TIMEOUT}"
)

if [[ -n "${EXPECT_MQTT_HOST}" ]]; then
  ARGS+=(-expect-mqtt-host "${EXPECT_MQTT_HOST}")
fi

if [[ "${CHECK_MESSAGES}" == "0" ]]; then
  ARGS+=(-check-messages=false)
fi

cd "${ROOT_DIR}"
go run ./scripts/app_dns_acl_message_smoke.go "${ARGS[@]}"
