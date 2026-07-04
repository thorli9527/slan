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
ADDRESS="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46392}"
EMAIL="${SLAN_TEST_EMAIL:-}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
REGISTER_USER="${SLAN_TEST_REGISTER_USER:-false}"
TIMEOUT="${SLAN_MACOS_LOGIN_CHECK_TIMEOUT:-30s}"

if [[ -z "$EMAIL" ]]; then
  echo "SLAN_TEST_EMAIL is required" >&2
  exit 1
fi

echo "+ go run scripts/client_core_service_login_check.go -biz-url $BIZ_URL -address $ADDRESS -email $EMAIL -timeout $TIMEOUT"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$ADDRESS" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register="$REGISTER_USER" \
    -timeout "$TIMEOUT"
)
