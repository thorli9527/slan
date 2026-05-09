#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$ROOT_DIR/client_v2/rust/target/debug/client-core-service}"
SMOKE_TIMEOUT="${SLAN_TRIDEVICE_MESSAGE_SMOKE_TIMEOUT:-55s}"
BIZ_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"

if [[ ! -x "$SERVICE_BIN" ]]; then
  echo "client-core-service binary is missing: $SERVICE_BIN" >&2
  echo "run: cd $ROOT_DIR/client_v2/rust && cargo build -p client-core-service" >&2
  exit 1
fi

echo "+ go run scripts/client_core_service_tridevice_message_smoke.go -biz-url $BIZ_URL -service-bin $SERVICE_BIN -timeout $SMOKE_TIMEOUT"
(
  cd "$ROOT_DIR"
  go run scripts/client_core_service_tridevice_message_smoke.go \
    -biz-url "$BIZ_URL" \
    -service-bin "$SERVICE_BIN" \
    -timeout "$SMOKE_TIMEOUT"
)

echo "macAndroidIosMessageCheck: ok bizUrl=$BIZ_URL"
