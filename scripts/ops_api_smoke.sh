#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${SLAN_OPS_SMOKE_PORT:-39180}"
BASE_URL="http://127.0.0.1:${PORT}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/slan-ops-smoke.XXXXXX")"
DOWNLOAD_DIR="${TMP_DIR}/downloads"
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

cd "${ROOT_DIR}/server/service-biz"
SLAN_BIZ_ADDR="127.0.0.1:${PORT}" \
SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
SLAN_CLIENT_DOWNLOAD_DIR="${DOWNLOAD_DIR}" \
go run ./cmd/service-biz >"${LOG_FILE}" 2>&1 &
BIZ_PID=$!

for _ in {1..80}; do
  if curl --silent --fail "${BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
curl --silent --fail "${BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1 || fail "service-biz did not start"

UNAUTH_CODE="$(curl --silent --output /dev/null --write-out '%{http_code}' "${BASE_URL}/api/ops/operators")"
[[ "${UNAUTH_CODE}" == "401" ]] || fail "protected ops endpoint expected 401, got ${UNAUTH_CODE}"

OPS_AUTH="$(curl --silent --fail -X POST "${BASE_URL}/api/ops/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@slan.local","password":"admin123456"}')" || fail "ops login failed"
OPS_TOKEN="$(printf '%s' "${OPS_AUTH}" | json_value token)"
[[ -n "${OPS_TOKEN}" ]] || fail "missing ops token"

auth_curl "${BASE_URL}/api/ops/dashboard" >/dev/null || fail "dashboard failed"
auth_curl "${BASE_URL}/api/ops/operators" >/dev/null || fail "operators list failed"
OPERATOR="$(auth_curl -X POST "${BASE_URL}/api/ops/operators" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Ops","email":"ops-smoke@staticlss.com","role":"ops","status":"active"}')" || fail "operator create failed"
OPERATOR_ID="$(printf '%s' "${OPERATOR}" | json_value operatorId)"
[[ -n "${OPERATOR_ID}" ]] || fail "missing operator id"
auth_curl -X PATCH "${BASE_URL}/api/ops/operators/${OPERATOR_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Ops Updated","email":"ops-smoke@staticlss.com","role":"admin","status":"active"}' >/dev/null || fail "operator update failed"
auth_curl -X POST "${BASE_URL}/api/ops/operators/${OPERATOR_ID}/password" \
  -H 'Content-Type: application/json' \
  -d '{"newPassword":"smoke-password-123"}' >/dev/null || fail "operator password update failed"

PLAN_CODE="smoke-plan"
auth_curl -X POST "${BASE_URL}/api/ops/plans" \
  -H 'Content-Type: application/json' \
  -d "{\"code\":\"${PLAN_CODE}\",\"name\":\"Smoke Plan\",\"ownDeviceLimit\":5,\"invitedDeviceLimit\":5,\"totalDeviceLimit\":10,\"relayMonthlyGb\":100,\"relayBandwidthMbps\":50,\"relayThrottleMbps\":5,\"p2pUnlimited\":true,\"customDomain\":true,\"acl\":true,\"dedicatedRelay\":false,\"auditLog\":true,\"apiAccess\":false,\"monthlyPrice\":9,\"yearlyPrice\":99,\"status\":\"active\"}" >/dev/null || fail "plan create failed"
auth_curl -X PATCH "${BASE_URL}/api/ops/plans/${PLAN_CODE}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Plan Updated\",\"ownDeviceLimit\":6,\"invitedDeviceLimit\":6,\"totalDeviceLimit\":12,\"relayMonthlyGb\":120,\"relayBandwidthMbps\":60,\"relayThrottleMbps\":6,\"p2pUnlimited\":true,\"customDomain\":true,\"acl\":true,\"dedicatedRelay\":false,\"auditLog\":true,\"apiAccess\":true,\"monthlyPrice\":10,\"yearlyPrice\":100,\"status\":\"active\"}" >/dev/null || fail "plan update failed"
auth_curl "${BASE_URL}/api/ops/plans" >/dev/null || fail "plans list failed"

PRODUCT="$(auth_curl -X POST "${BASE_URL}/api/ops/products" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Product\",\"type\":\"plan\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"monthly\",\"validDays\":31,\"relayTrafficGb\":100,\"relayBandwidthMbps\":50,\"listPrice\":10,\"salePrice\":8,\"currency\":\"CNY\",\"autoRenew\":false,\"status\":\"active\",\"description\":\"smoke\"}")" || fail "product create failed"
PRODUCT_ID="$(printf '%s' "${PRODUCT}" | json_value productId)"
[[ -n "${PRODUCT_ID}" ]] || fail "missing product id"
auth_curl -X PATCH "${BASE_URL}/api/ops/products/${PRODUCT_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Product Updated\",\"type\":\"plan\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"monthly\",\"validDays\":31,\"relayTrafficGb\":120,\"relayBandwidthMbps\":60,\"listPrice\":12,\"salePrice\":9,\"currency\":\"CNY\",\"autoRenew\":true,\"status\":\"active\",\"description\":\"smoke updated\"}" >/dev/null || fail "product update failed"
auth_curl "${BASE_URL}/api/ops/products" >/dev/null || fail "products list failed"

RELAY_NODE="$(auth_curl -X POST "${BASE_URL}/api/ops/relay-nodes" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Relay","region":"smoke","transport":"relay_udp","publicAddr":"udp://127.0.0.1:39210","maxBandwidthMbps":1000,"monthlyTrafficGb":1024,"maxSessions":100,"status":"active"}')" || fail "relay node create failed"
RELAY_NODE_ID="$(printf '%s' "${RELAY_NODE}" | json_value nodeId)"
[[ -n "${RELAY_NODE_ID}" ]] || fail "missing relay node id"
auth_curl -X PATCH "${BASE_URL}/api/ops/relay-nodes/${RELAY_NODE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Relay Updated","region":"smoke","transport":"relay_udp","publicAddr":"udp://127.0.0.1:39210","maxBandwidthMbps":900,"monthlyTrafficGb":2048,"maxSessions":120,"status":"disabled"}' >/dev/null || fail "relay node update failed"
auth_curl "${BASE_URL}/api/ops/relay-nodes" >/dev/null || fail "relay nodes list failed"

USER_AUTH="$(curl --silent --fail -X POST "${BASE_URL}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops-smoke-user@staticlss.com","password":"password","name":"Ops Smoke User"}')" || fail "customer seed register failed"
USER_ID="$(printf '%s' "${USER_AUTH}" | json_value userId)"
[[ -n "${USER_ID}" ]] || fail "missing seeded user id"

auth_curl -X PATCH "${BASE_URL}/api/ops/customers/${USER_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops-smoke-user@staticlss.com","name":"Ops Smoke User Updated","country":"CN","province":"Guangdong","city":"Shenzhen","ipRegion":"South China","status":"active"}' >/dev/null || fail "customer update failed"
ASSIGN_RESPONSE="$(auth_curl -X POST "${BASE_URL}/api/ops/customers/${USER_ID}/assign-plan" \
  -H 'Content-Type: application/json' \
  -d "{\"planCode\":\"${PLAN_CODE}\",\"expiresAt\":1821264000,\"amount\":100,\"period\":\"yearly\"}")" || fail "customer assign plan failed"
RENEWAL_ID="$(printf '%s' "${ASSIGN_RESPONSE}" | json_value renewalId)"
[[ -n "${RENEWAL_ID}" ]] || fail "missing renewal id"
auth_curl "${BASE_URL}/api/ops/customers" >/dev/null || fail "customers list failed"

DEVICE_ID="ops-smoke-device"
DELETE_DEVICE_ID="ops-smoke-delete-device"
curl --silent --fail -X POST "${BASE_URL}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DEVICE_ID}\",\"name\":\"Ops Smoke Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"15.3\",\"alias\":\"Ops Smoke Mac\",\"publicKey\":\"ops-smoke-public-key\"}" >/dev/null || fail "device seed register failed"
curl --silent --fail -X POST "${BASE_URL}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DELETE_DEVICE_ID}\",\"name\":\"Ops Smoke Delete\",\"platform\":\"linux\",\"osName\":\"Linux\",\"osVersion\":\"6.8\",\"alias\":\"Ops Smoke Delete\",\"publicKey\":\"ops-smoke-delete-public-key\"}" >/dev/null || fail "delete device seed register failed"
auth_curl -X PATCH "${BASE_URL}/api/ops/devices/${DEVICE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"alias":"Ops Smoke Mac Updated","status":"active","enabled":true}' >/dev/null || fail "device update failed"
auth_curl -X DELETE "${BASE_URL}/api/ops/devices/${DELETE_DEVICE_ID}" >/dev/null || fail "device delete failed"
auth_curl "${BASE_URL}/api/ops/devices" >/dev/null || fail "devices list failed"

ORDER="$(auth_curl -X POST "${BASE_URL}/api/ops/orders" \
  -H 'Content-Type: application/json' \
  -d "{\"customerEmail\":\"ops-smoke-user@staticlss.com\",\"productId\":\"${PRODUCT_ID}\",\"amount\":9,\"currency\":\"CNY\",\"payStatus\":\"pending\",\"provisionStatus\":\"pending\",\"channel\":\"manual\"}")" || fail "order create failed"
ORDER_ID="$(printf '%s' "${ORDER}" | json_value orderId)"
[[ -n "${ORDER_ID}" ]] || fail "missing order id"
auth_curl -X PATCH "${BASE_URL}/api/ops/orders/${ORDER_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"customerEmail\":\"ops-smoke-user@staticlss.com\",\"productId\":\"${PRODUCT_ID}\",\"amount\":9,\"currency\":\"CNY\",\"payStatus\":\"paid\",\"provisionStatus\":\"provisioned\",\"channel\":\"manual\",\"paidAt\":1783267200,\"validUntil\":1821264000}" >/dev/null || fail "order update failed"
auth_curl "${BASE_URL}/api/ops/orders" >/dev/null || fail "orders list failed"

auth_curl -X PATCH "${BASE_URL}/api/ops/renewals/${RENEWAL_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"customerEmail\":\"ops-smoke-user@staticlss.com\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"yearly\",\"amount\":100,\"currency\":\"CNY\",\"paidAt\":1783267200,\"validUntil\":1821264000,\"source\":\"manual\",\"operator\":\"admin@slan.local\"}" >/dev/null || fail "renewal update failed"
auth_curl "${BASE_URL}/api/ops/renewals" >/dev/null || fail "renewals list failed"

printf 'smoke-client-package\n' >"${TMP_DIR}/SLAN-Client-Smoke.pkg"
DOWNLOAD="$(curl --silent --fail -X POST "${BASE_URL}/api/ops/client-downloads" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -F platform=macos \
  -F version=9.9.9-smoke \
  -F arch=universal \
  -F channel=stable \
  -F status=active \
  -F releaseNotes=smoke \
  -F "file=@${TMP_DIR}/SLAN-Client-Smoke.pkg")" || fail "client download upload failed"
DOWNLOAD_ID="$(printf '%s' "${DOWNLOAD}" | json_value downloadId)"
[[ -n "${DOWNLOAD_ID}" ]] || fail "missing download id"
auth_curl "${BASE_URL}/api/ops/client-downloads" >/dev/null || fail "client downloads ops list failed"
curl --silent --fail "${BASE_URL}/api/client-downloads" >/dev/null || fail "client downloads public list failed"
auth_curl -X DELETE "${BASE_URL}/api/ops/client-downloads/${DOWNLOAD_ID}" >/dev/null || fail "client download delete failed"

AUDIT_EVENTS="$(auth_curl "${BASE_URL}/api/ops/audit-events?limit=20")" || fail "audit events list failed"
printf '%s' "${AUDIT_EVENTS}" | grep -q '"items"' || fail "audit events response missing items"

echo "ops api smoke passed on ${BASE_URL}"
