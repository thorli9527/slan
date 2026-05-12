#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${SLAN_BIZ_NEW_SMOKE_PORT:-39080}"
BASE_URL="http://127.0.0.1:${PORT}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"

cleanup() {
  if [[ -n "${BIZ_PID:-}" ]]; then
    kill "${BIZ_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

cd "${ROOT_DIR}/server/service-biz-new"

SLAN_BIZ_NEW_ADDR="127.0.0.1:${PORT}" \
SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
go run ./cmd/service-biz-new >/tmp/slan-service-biz-new-smoke.log 2>&1 &
BIZ_PID=$!

for _ in {1..30}; do
  if curl --silent --fail "${BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

curl --silent --fail -X PUT "${BASE_URL}/internal/wire/admin/relay-nodes" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"regionId":"smoke","nodeId":"relay-smoke","host":"127.0.0.1","udpPort":29110,"adminPort":29111,"enabled":true,"healthy":true,"priority":10}' >/dev/null
curl --silent --fail -X POST "${BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/heartbeat" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null
curl --silent --fail -X PATCH "${BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/status" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"enabled":true,"healthy":true}' >/dev/null

curl --silent --fail -X PUT "${BASE_URL}/internal/wire/admin/derp-nodes" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"regionId":"smoke","nodeId":"derp-smoke","name":"Smoke DERP","host":"127.0.0.1","port":29120,"enabled":true,"healthy":true,"priority":10}' >/dev/null
curl --silent --fail -X POST "${BASE_URL}/internal/wire/admin/derp-nodes/smoke/derp-smoke/heartbeat" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null
DERP_MAP="$(curl --silent --fail "${BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if ! printf '%s' "${DERP_MAP}" | grep -q '"nodeId":"derp-smoke"'; then
  echo "derp smoke node missing from derp map" >&2
  exit 1
fi

USER_AUTH="$(curl --silent --fail -X POST "${BASE_URL}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke@vlan.com","password":"password","name":"Smoke"}')"
USER_ID="$(printf '%s' "${USER_AUTH}" | sed -n 's/.*"userId":"\([^"]*\)".*/\1/p')"
NETWORK_ID="$(printf '%s' "${USER_AUTH}" | sed -n 's/.*"networkId":"\([^"]*\)".*/\1/p')"
if [[ -z "${USER_ID}" || -z "${NETWORK_ID}" ]]; then
  echo "missing registered user or default network" >&2
  exit 1
fi

DEVICE_ID="smoke-mac-001"
DEVICE_REGISTER="$(curl --silent --fail -X POST "${BASE_URL}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DEVICE_ID}\",\"name\":\"Smoke Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"15.3\",\"alias\":\"Smoke Mac\",\"publicKey\":\"smoke-public-key\"}")"
if ! printf '%s' "${DEVICE_REGISTER}" | grep -q '"globalIp":"10\.'; then
  echo "registered device did not receive global IP" >&2
  exit 1
fi
DST_DEVICE_ID="smoke-ios-001"
curl --silent --fail -X POST "${BASE_URL}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DST_DEVICE_ID}\",\"name\":\"Smoke iPhone\",\"platform\":\"ios\",\"osName\":\"iOS\",\"osVersion\":\"18.3\",\"alias\":\"Smoke iPhone\",\"publicKey\":\"smoke-ios-public-key\"}" >/dev/null

curl --silent --fail "${BASE_URL}/api/devices/${DEVICE_ID}/mqtt-credential" >/dev/null
curl --silent --fail -X POST "${BASE_URL}/api/devices/${DEVICE_ID}/renew" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkEnabled\":true,\"rxBytesTotal\":1024,\"txBytesTotal\":2048}" >/dev/null
curl --silent --fail -X POST "${BASE_URL}/api/devices/${DST_DEVICE_ID}/renew" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkEnabled\":true,\"rxBytesTotal\":256,\"txBytesTotal\":512}" >/dev/null
curl --silent --fail "${BASE_URL}/api/devices/${DEVICE_ID}/network-configs" >/dev/null
curl --silent --fail "${BASE_URL}/api/networks/${NETWORK_ID}/network-config?deviceId=${DEVICE_ID}" >/dev/null
curl --silent --fail "${BASE_URL}/api/networks/${NETWORK_ID}/relay-candidates?deviceId=${DEVICE_ID}" >/dev/null
PEER_ID="${NETWORK_ID}:${DEVICE_ID}"
curl --silent --fail "${BASE_URL}/internal/wire/peers/${PEER_ID}/authz" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null
curl --silent --fail "${BASE_URL}/internal/wire/peers/node-${DEVICE_ID}/runtime-config" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null
TOPOLOGY="$(curl --silent --fail "${BASE_URL}/internal/wire/networks/${NETWORK_ID}/topology" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if ! printf '%s' "${TOPOLOGY}" | grep -q "\"peerId\":\"${NETWORK_ID}:${DST_DEVICE_ID}\""; then
  echo "network topology missing destination peer" >&2
  exit 1
fi
RELAY_TICKET="$(curl --silent --fail -X POST "${BASE_URL}/api/relay/tickets" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke\"}")"
if ! printf '%s' "${RELAY_TICKET}" | grep -q '"ticketId":"rt-'; then
  echo "relay ticket was not issued" >&2
  exit 1
fi

OPS_AUTH="$(curl --silent --fail -X POST "${BASE_URL}/api/ops/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@slan.local","password":"admin123456"}')"
OPS_TOKEN="$(printf '%s' "${OPS_AUTH}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [[ -z "${OPS_TOKEN}" ]]; then
  echo "missing ops token" >&2
  exit 1
fi

curl --silent --fail "${BASE_URL}/api/ops/operators" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null
curl --silent --fail "${BASE_URL}/api/ops/relay-nodes" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null
curl --silent --fail "${BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null

echo "service-biz-new smoke passed on ${BASE_URL}"
