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
AUTHORIZATION_KEY="${SLAN_DEVICE_AUTHORIZATION_KEY:-}"
TIMEOUT="${SLAN_MACOS_ACTIVATION_CHECK_TIMEOUT:-30s}"

if [[ -z "$AUTHORIZATION_KEY" ]]; then
  echo "SLAN_DEVICE_AUTHORIZATION_KEY is required" >&2
  exit 1
fi

echo "+ go run activation check -biz-url $BIZ_URL -address $ADDRESS -timeout $TIMEOUT"
(
  cd "$ROOT_DIR"
  go run scripts/tests/shared/client_core_service_login_check.go \
    -biz-url "$BIZ_URL" \
    -address "$ADDRESS" \
    -authorization-key "$AUTHORIZATION_KEY" \
    -timeout "$TIMEOUT"
)
