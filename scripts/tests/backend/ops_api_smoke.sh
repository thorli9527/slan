#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
PORT="${SLAN_OPS_SMOKE_PORT:-39180}"
START_LOCAL_BIZ="${SLAN_OPS_SMOKE_START:-1}"
if [[ -n "${SLAN_APP_BASE_URL:-}" ]]; then
  APP_BASE_URL="$SLAN_APP_BASE_URL"
elif [[ -n "${SLAN_BIZ_BASE_URL:-}" ]]; then
  APP_BASE_URL="$SLAN_BIZ_BASE_URL"
elif [[ "$START_LOCAL_BIZ" == "1" ]]; then
  APP_BASE_URL="http://127.0.0.1:${PORT}"
else
  APP_BASE_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
fi
if [[ -n "${SLAN_OPS_BASE_URL:-}" ]]; then
  OPS_BASE_URL="$SLAN_OPS_BASE_URL"
elif [[ "$START_LOCAL_BIZ" == "1" ]]; then
  OPS_BASE_URL="http://127.0.0.1:${PORT}"
else
  OPS_BASE_URL="$SLAN_DEFAULT_OPS_BASE_URL"
fi
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"
RUN_ID="$(date +%s%N)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/slan-ops-smoke.XXXXXX")"
LOG_FILE="${TMP_DIR}/service-biz.log"

cleanup() {
  if [[ -n "${BIZ_PID:-}" ]]; then
    kill "${BIZ_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

fail() {
  echo "ops api smoke failed: $*" >&2
  if [[ -f "${LOG_FILE}" ]]; then
    tail -80 "${LOG_FILE}" >&2 || true
  fi
  exit 1
}

json_value() {
  local key="$1"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p"
}

auth_curl() {
  curl --silent --fail -H "Authorization: Bearer ${OPS_TOKEN}" "$@"
}

if [[ "${START_LOCAL_BIZ}" != "0" ]]; then
  cd "${ROOT_DIR}/server/service-biz"
  SLAN_BIZ_ADDR="127.0.0.1:${PORT}" \
  SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  go run ./cmd/service-biz >"${LOG_FILE}" 2>&1 &
  BIZ_PID=$!

  for _ in {1..80}; do
    if curl --silent --fail "${APP_BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1; then
      break
    fi
    sleep 0.25
  done
  curl --silent --fail "${APP_BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1 || fail "service-biz did not start"
fi

for _ in {1..30}; do
  if curl --silent --fail "${OPS_BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
curl --silent --fail "${OPS_BASE_URL}/healthz" >/dev/null 2>&1 || fail "ops healthz failed"

OPS_AUTH=""
for _ in {1..30}; do
  if OPS_AUTH="$(curl --silent --fail -X POST "${OPS_BASE_URL}/api/ops/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"email":"admin1","password":"admin1"}' 2>/dev/null)"; then
    break
  fi
  sleep 0.5
done
[[ -n "${OPS_AUTH}" ]] || fail "ops login failed"
OPS_TOKEN="$(printf '%s' "${OPS_AUTH}" | json_value token)"
[[ -n "${OPS_TOKEN}" ]] || fail "missing ops token"

auth_curl "${OPS_BASE_URL}/api/ops/dashboard" >/dev/null || fail "dashboard failed"
auth_curl "${OPS_BASE_URL}/api/ops/operators" >/dev/null || fail "operators list failed"
OPERATOR_EMAIL="ops-smoke-${RUN_ID}@staticlss.com"
OPERATOR="$(auth_curl -X POST "${OPS_BASE_URL}/api/ops/operators" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Ops\",\"email\":\"${OPERATOR_EMAIL}\",\"role\":\"ops\",\"password\":\"smoke-password-123\",\"status\":\"active\"}")" || fail "operator create failed"
OPERATOR_ID="$(printf '%s' "${OPERATOR}" | json_value operatorId)"
[[ -n "${OPERATOR_ID}" ]] || fail "missing operator id"
auth_curl -X PATCH "${OPS_BASE_URL}/api/ops/operators/${OPERATOR_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Ops Updated","email":"ops-smoke@staticlss.com","role":"admin","status":"active"}' >/dev/null || fail "operator update failed"
auth_curl -X POST "${OPS_BASE_URL}/api/ops/operators/${OPERATOR_ID}/password" \
  -H 'Content-Type: application/json' \
  -d '{"password":"smoke-password-123"}' >/dev/null || fail "operator password update failed"

RELAY_NODE="$(auth_curl -X POST "${OPS_BASE_URL}/api/ops/relay-nodes" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Relay\",\"region\":\"smoke-${RUN_ID}\",\"transport\":\"relay_udp\",\"publicAddr\":\"udp://127.0.0.1:39210\",\"maxBandwidthMbps\":1000,\"monthlyTrafficGb\":1024,\"maxSessions\":100,\"status\":\"active\"}")" || fail "relay node create failed"
RELAY_NODE_ID="$(printf '%s' "${RELAY_NODE}" | json_value nodeId)"
[[ -n "${RELAY_NODE_ID}" ]] || fail "missing relay node id"
auth_curl -X PATCH "${OPS_BASE_URL}/api/ops/relay-nodes/${RELAY_NODE_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Relay Updated\",\"region\":\"smoke-${RUN_ID}\",\"transport\":\"relay_udp\",\"publicAddr\":\"udp://127.0.0.1:39210\",\"maxBandwidthMbps\":900,\"monthlyTrafficGb\":2048,\"maxSessions\":120,\"status\":\"disabled\"}" >/dev/null || fail "relay node update failed"
auth_curl "${OPS_BASE_URL}/api/ops/relay-nodes" >/dev/null || fail "relay nodes list failed"
auth_curl -X DELETE "${OPS_BASE_URL}/api/ops/relay-nodes/${RELAY_NODE_ID}" >/dev/null || fail "relay node delete failed"

auth_curl "${OPS_BASE_URL}/api/ops/customers" >/dev/null || fail "customers list failed"

DEVICE="$(auth_curl -X POST "${OPS_BASE_URL}/api/ops/devices" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ops Smoke Mac","platform":"macos","osName":"macOS","osVersion":"15.3","alias":"Ops Smoke Mac","publicKey":"ops-smoke-public-key"}')" || fail "device create failed"
DEVICE_ID="$(printf '%s' "${DEVICE}" | json_value deviceId)"
[[ -n "${DEVICE_ID}" ]] || fail "missing created device id"
DELETE_DEVICE="$(auth_curl -X POST "${OPS_BASE_URL}/api/ops/devices" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ops Smoke Delete","platform":"linux","osName":"Linux","osVersion":"6.8","alias":"Ops Smoke Delete","publicKey":"ops-smoke-delete-public-key"}')" || fail "delete device create failed"
DELETE_DEVICE_ID="$(printf '%s' "${DELETE_DEVICE}" | json_value deviceId)"
[[ -n "${DELETE_DEVICE_ID}" ]] || fail "missing delete device id"
auth_curl -X PATCH "${OPS_BASE_URL}/api/ops/devices/${DEVICE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"alias":"Ops Smoke Mac Updated","status":"active","enabled":true}' >/dev/null || fail "device update failed"
auth_curl -X DELETE "${OPS_BASE_URL}/api/ops/devices/${DELETE_DEVICE_ID}" >/dev/null || fail "device delete failed"
auth_curl "${OPS_BASE_URL}/api/ops/devices" >/dev/null || fail "devices list failed"

CREDENTIAL="$(auth_curl -X POST "${OPS_BASE_URL}/api/ops/device-credentials" \
  -H 'Content-Type: application/json' \
  -d "{\"deviceId\":\"${DEVICE_ID}\",\"name\":\"Ops Smoke Key\",\"scopes\":\"standard_device\",\"expiresAt\":0}")" || fail "device credential create failed"
CREDENTIAL_ID="$(printf '%s' "${CREDENTIAL}" | json_value credentialId)"
AUTHORIZATION_KEY="$(printf '%s' "${CREDENTIAL}" | json_value key)"
[[ -n "${CREDENTIAL_ID}" && -n "${AUTHORIZATION_KEY}" ]] || fail "missing device credential id or key"
DEVICE_SESSION="$(curl --silent --fail -X POST "${APP_BASE_URL}/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"${AUTHORIZATION_KEY}\",\"deviceId\":\"${DEVICE_ID}\"}")" || fail "device credential exchange failed"
printf '%s' "${DEVICE_SESSION}" | grep -q '"deviceToken":"' || fail "device credential exchange missing token"
auth_curl "${OPS_BASE_URL}/api/ops/device-credentials" >/dev/null || fail "device credentials list failed"
auth_curl -X POST "${OPS_BASE_URL}/api/ops/device-credentials/${CREDENTIAL_ID}/revoke" >/dev/null || fail "device credential revoke failed"
auth_curl -X DELETE "${OPS_BASE_URL}/api/ops/devices/${DEVICE_ID}" >/dev/null || fail "credential device delete failed"


AUDIT_EVENTS="$(auth_curl "${OPS_BASE_URL}/api/ops/audit-events?limit=20")" || fail "audit events list failed"
printf '%s' "${AUDIT_EVENTS}" | grep -q '"items"' || fail "audit events response missing items"

echo "ops api smoke passed app=${APP_BASE_URL} ops=${OPS_BASE_URL}"
