#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${SLAN_BIZ_SMOKE_PORT:-39080}"
APP_BASE_URL="${SLAN_APP_BASE_URL:-${SLAN_BIZ_BASE_URL:-http://127.0.0.1:${PORT}}}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-${SLAN_BIZ_WEB_BASE_URL:-${APP_BASE_URL}}}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-${APP_BASE_URL}}"
START_LOCAL_BIZ="${SLAN_BIZ_SMOKE_START:-1}"
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"
RUN_ID="$(date +%s%N)"

cleanup() {
  if [[ -n "${BIZ_PID:-}" ]]; then
    kill "${BIZ_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ "${START_LOCAL_BIZ}" != "0" ]]; then
  cd "${ROOT_DIR}/server/service-biz"

  SLAN_BIZ_ADDR="127.0.0.1:${PORT}" \
  SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  go run ./cmd/service-biz >/tmp/slan-service-biz-smoke.log 2>&1 &
  BIZ_PID=$!

  for _ in {1..30}; do
    if curl --silent --fail "${APP_BASE_URL}/internal/wire/admin/relay-nodes" -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
fi

for _ in {1..30}; do
  if curl --silent --fail "${APP_BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

curl --silent --fail -X PUT "${APP_BASE_URL}/internal/wire/admin/relay-nodes" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"regionId":"smoke","nodeId":"relay-smoke","host":"127.0.0.1","udpPort":29110,"adminPort":29111,"enabled":true,"healthy":true,"priority":10}' >/dev/null
curl --silent --fail -X POST "${APP_BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/heartbeat" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null
curl --silent --fail -X PATCH "${APP_BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/status" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"enabled":true,"healthy":true}' >/dev/null

curl --silent --fail -X PUT "${APP_BASE_URL}/internal/wire/admin/derp-nodes" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"regionId":"smoke","nodeId":"derp-smoke","name":"Smoke DERP","host":"127.0.0.1","port":29120,"enabled":true,"healthy":true,"priority":10}' >/dev/null
curl --silent --fail -X POST "${APP_BASE_URL}/internal/wire/admin/derp-nodes/smoke/derp-smoke/heartbeat" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null
DERP_MAP="$(curl --silent --fail "${APP_BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if ! printf '%s' "${DERP_MAP}" | grep -q '"nodeId":"derp-smoke"'; then
  echo "derp smoke node missing from derp map" >&2
  exit 1
fi

USER_EMAIL="smoke-${RUN_ID}@staticlss.com"
USER_AUTH="$(curl --silent --fail -X POST "${APP_BASE_URL}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${USER_EMAIL}\",\"password\":\"password\",\"name\":\"Smoke\"}")"
USER_ID="$(printf '%s' "${USER_AUTH}" | sed -n 's/.*"userId":"\([^"]*\)".*/\1/p')"
NETWORK_ID="$(printf '%s' "${USER_AUTH}" | sed -n 's/.*"networkId":"\([^"]*\)".*/\1/p')"
USER_TOKEN="$(printf '%s' "${USER_AUTH}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [[ -z "${USER_ID}" || -z "${NETWORK_ID}" || -z "${USER_TOKEN}" ]]; then
  echo "missing registered user, token, or default network" >&2
  exit 1
fi

DEVICE_ID="smoke-mac-${RUN_ID}"
DEVICE_REGISTER="$(curl --silent --fail -X POST "${APP_BASE_URL}/api/devices/register" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DEVICE_ID}\",\"name\":\"Smoke Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"15.3\",\"alias\":\"Smoke Mac\",\"publicKey\":\"smoke-public-key\"}")"
if ! printf '%s' "${DEVICE_REGISTER}" | grep -q '"globalIp":"10\.'; then
  echo "registered device did not receive global IP" >&2
  exit 1
fi
DST_DEVICE_ID="smoke-ios-${RUN_ID}"
curl --silent --fail -X POST "${APP_BASE_URL}/api/devices/register" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DST_DEVICE_ID}\",\"name\":\"Smoke iPhone\",\"platform\":\"ios\",\"osName\":\"iOS\",\"osVersion\":\"18.3\",\"alias\":\"Smoke iPhone\",\"publicKey\":\"smoke-ios-public-key\"}" >/dev/null

curl --silent --fail "${APP_BASE_URL}/api/devices/${DEVICE_ID}/mqtt-credential" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null
curl --silent --fail -X POST "${APP_BASE_URL}/api/devices/${DEVICE_ID}/renew" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkEnabled\":true,\"rxBytesTotal\":1024,\"txBytesTotal\":2048}" >/dev/null
curl --silent --fail -X POST "${APP_BASE_URL}/api/devices/${DST_DEVICE_ID}/renew" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkEnabled\":true,\"rxBytesTotal\":256,\"txBytesTotal\":512}" >/dev/null
curl --silent --fail "${APP_BASE_URL}/api/devices/${DEVICE_ID}/network-configs" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null
curl --silent --fail "${APP_BASE_URL}/api/networks/${NETWORK_ID}/network-config?deviceId=${DEVICE_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null
curl --silent --fail "${APP_BASE_URL}/api/networks/${NETWORK_ID}/relay-candidates?deviceId=${DEVICE_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null
PEER_ID="${NETWORK_ID}:${DEVICE_ID}"
curl --silent --fail "${APP_BASE_URL}/internal/wire/peers/${PEER_ID}/authz" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null
curl --silent --fail "${APP_BASE_URL}/internal/wire/peers/node-${DEVICE_ID}/runtime-config" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null
TOPOLOGY="$(curl --silent --fail "${APP_BASE_URL}/internal/wire/networks/${NETWORK_ID}/topology" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if ! printf '%s' "${TOPOLOGY}" | grep -q "\"peerId\":\"${NETWORK_ID}:${DST_DEVICE_ID}\""; then
  echo "network topology missing destination peer" >&2
  exit 1
fi
RELAY_TICKET="$(curl --silent --fail -X POST "${APP_BASE_URL}/api/relay/tickets" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke\"}")"
RELAY_TICKET_ID="$(printf '%s' "${RELAY_TICKET}" | sed -n 's/.*"ticketId":"\([^"]*\)".*/\1/p')"
if [[ -z "${RELAY_TICKET_ID}" ]]; then
  echo "relay ticket was not issued: ${RELAY_TICKET}" >&2
  exit 1
fi

SECURITY_GROUPS="$(curl --silent --fail "${WEB_BASE_URL}/api/networks/${NETWORK_ID}/security-groups" \
  -H "Authorization: Bearer ${USER_TOKEN}")"
SECURITY_GROUP_ID="$(printf '%s' "${SECURITY_GROUPS}" | sed -n 's/.*"securityGroupId":"\([^"]*\)".*/\1/p')"
if [[ -z "${SECURITY_GROUP_ID}" ]]; then
  echo "missing security group" >&2
  exit 1
fi
INGRESS_DENY_RULE="$(curl --silent --fail -X POST "${WEB_BASE_URL}/api/security-groups/${SECURITY_GROUP_ID}/rules" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":5,\"action\":\"deny\",\"protocol\":\"all\",\"portFrom\":0,\"portTo\":0,\"peerType\":\"device\",\"peerValue\":\"${DST_DEVICE_ID}\",\"description\":\"deny smoke peer ingress\",\"enabled\":true}")"
INGRESS_DENY_RULE_ID="$(printf '%s' "${INGRESS_DENY_RULE}" | sed -n 's/.*"ruleId":"\([^"]*\)".*/\1/p')"
if [[ -z "${INGRESS_DENY_RULE_ID}" ]]; then
  echo "missing ingress deny security rule id" >&2
  exit 1
fi
INGRESS_DENIED_CONFIG="$(curl --silent --fail "${APP_BASE_URL}/api/networks/${NETWORK_ID}/network-config?deviceId=${DEVICE_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}")"
if ! printf '%s' "${INGRESS_DENIED_CONFIG}" | grep -q "\"ruleId\":\"${INGRESS_DENY_RULE_ID}\""; then
  echo "Ingress ACL deny rule missing from network config: ${INGRESS_DENIED_CONFIG}" >&2
  exit 1
fi
if ! printf '%s' "${INGRESS_DENIED_CONFIG}" | grep -q "\"resolvedPeerNodeId\":\"node-${DST_DEVICE_ID}\""; then
  echo "Ingress ACL deny rule missing resolved peer node: ${INGRESS_DENIED_CONFIG}" >&2
  exit 1
fi
INGRESS_DENIED_TICKET_STATUS="$(curl --silent --output /tmp/slan-service-biz-ingress-denied-ticket.json --write-out '%{http_code}' -X POST "${APP_BASE_URL}/api/relay/tickets" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke-ingress-denied\"}")"
if [[ "${INGRESS_DENIED_TICKET_STATUS}" == "200" || "${INGRESS_DENIED_TICKET_STATUS}" == "201" ]]; then
  echo "Ingress ACL deny still allowed relay ticket: $(cat /tmp/slan-service-biz-ingress-denied-ticket.json)" >&2
  exit 1
fi
curl --silent --fail -X DELETE "${WEB_BASE_URL}/api/security-groups/rules/${INGRESS_DENY_RULE_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null

curl --silent --fail -X POST "${WEB_BASE_URL}/api/security-groups/${SECURITY_GROUP_ID}/rules" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"egress\",\"priority\":10,\"action\":\"deny\",\"protocol\":\"all\",\"portFrom\":0,\"portTo\":0,\"peerType\":\"device\",\"peerValue\":\"${DST_DEVICE_ID}\",\"description\":\"deny smoke peer\",\"enabled\":true}" >/dev/null
DENIED_CONFIG="$(curl --silent --fail "${APP_BASE_URL}/api/networks/${NETWORK_ID}/network-config?deviceId=${DEVICE_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}")"
if ! printf '%s' "${DENIED_CONFIG}" | grep -q '"securityRuleCount":1'; then
  echo "ACL deny rule count missing from network config: ${DENIED_CONFIG}" >&2
  exit 1
fi
if ! printf '%s' "${DENIED_CONFIG}" | grep -q "\"resolvedPeerNodeId\":\"node-${DST_DEVICE_ID}\""; then
  echo "ACL deny rule missing resolved peer node: ${DENIED_CONFIG}" >&2
  exit 1
fi
DENIED_TICKET_STATUS="$(curl --silent --output /tmp/slan-service-biz-denied-ticket.json --write-out '%{http_code}' -X POST "${APP_BASE_URL}/api/relay/tickets" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke-denied\"}")"
if [[ "${DENIED_TICKET_STATUS}" == "200" || "${DENIED_TICKET_STATUS}" == "201" ]]; then
  echo "ACL deny still allowed relay ticket: $(cat /tmp/slan-service-biz-denied-ticket.json)" >&2
  exit 1
fi

OPS_AUTH="$(curl --silent --fail -X POST "${OPS_BASE_URL}/api/ops/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin1","password":"admin1"}')"
OPS_TOKEN="$(printf '%s' "${OPS_AUTH}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [[ -z "${OPS_TOKEN}" ]]; then
  echo "missing ops token" >&2
  exit 1
fi

curl --silent --fail "${OPS_BASE_URL}/api/ops/operators" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null
curl --silent --fail "${OPS_BASE_URL}/api/ops/relay-nodes" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null
curl --silent --fail "${APP_BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" >/dev/null

echo "service-biz smoke passed app=${APP_BASE_URL} web=${WEB_BASE_URL} ops=${OPS_BASE_URL}"
