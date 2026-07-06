#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
if [ ! -e "$ROOT_DIR/.git" ]; then
  ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
fi
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
BIZ_BASE_URL="${SLAN_BIZ_REMOTE_BASE_URL:-${SLAN_APP_BASE_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}}"
WEB_BASE_URL="${SLAN_WEB_REMOTE_BASE_URL:-${SLAN_WEB_BASE_URL:-http://${SLAN_DEFAULT_MQTT_HOST}:28081}}"
OPS_BASE_URL="${SLAN_OPS_REMOTE_BASE_URL:-${SLAN_OPS_BASE_URL:-http://${SLAN_DEFAULT_MQTT_HOST}:38080}}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-}"
SEED_WIRE_NODES="${SLAN_SERVICE_BIZ_SMOKE_SEED_WIRE_NODES:-0}"

if [[ -z "${WIRE_TOKEN}" ]]; then
  echo "SLAN_INTERNAL_WIRE_TOKEN is required for remote smoke runs" >&2
  exit 1
fi

echo "[service_biz_remote_smoke] app=${BIZ_BASE_URL} web=${WEB_BASE_URL} ops=${OPS_BASE_URL} seed=${SEED_WIRE_NODES}"

SLAN_BIZ_SMOKE_START=0 \
SLAN_APP_BASE_URL="${BIZ_BASE_URL}" \
SLAN_WEB_BASE_URL="${WEB_BASE_URL}" \
SLAN_OPS_BASE_URL="${OPS_BASE_URL}" \
SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
bash "${ROOT_DIR}/scripts/service_biz_smoke.sh"
