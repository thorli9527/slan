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
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"
PORT="${SLAN_BIZ_SMOKE_PORT:-39080}"
START_LOCAL_BIZ="${SLAN_BIZ_SMOKE_START:-1}"
if [[ -n "${SLAN_APP_BASE_URL:-}" ]]; then
  APP_BASE_URL="$SLAN_APP_BASE_URL"
elif [[ -n "${SLAN_BIZ_BASE_URL:-}" ]]; then
  APP_BASE_URL="$SLAN_BIZ_BASE_URL"
elif [[ "$START_LOCAL_BIZ" == "1" ]]; then
  APP_BASE_URL="http://127.0.0.1:${PORT}"
else
  APP_BASE_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
fi
if [[ -n "${SLAN_WEB_BASE_URL:-}" ]]; then
  WEB_BASE_URL="$SLAN_WEB_BASE_URL"
elif [[ -n "${SLAN_BIZ_WEB_BASE_URL:-}" ]]; then
  WEB_BASE_URL="$SLAN_BIZ_WEB_BASE_URL"
elif [[ "$START_LOCAL_BIZ" == "1" ]]; then
  WEB_BASE_URL="${APP_BASE_URL}"
else
  WEB_BASE_URL="$SLAN_DEFAULT_WEB_BASE_URL"
fi
if [[ -n "${SLAN_OPS_BASE_URL:-}" ]]; then
  OPS_BASE_URL="$SLAN_OPS_BASE_URL"
elif [[ "$START_LOCAL_BIZ" == "1" ]]; then
  OPS_BASE_URL="${APP_BASE_URL}"
else
  OPS_BASE_URL="$SLAN_DEFAULT_OPS_BASE_URL"
fi
WIRE_TOKEN="${SLAN_INTERNAL_WIRE_TOKEN:-wire-token}"
RUN_ID="$(date +%s%N)"
SEED_WIRE_NODES="${SLAN_SERVICE_BIZ_SMOKE_SEED_WIRE_NODES:-${START_LOCAL_BIZ}}"
HEALTH_RETRIES="${SLAN_SERVICE_BIZ_SMOKE_HEALTH_RETRIES:-60}"
HEALTH_SLEEP_SECS="${SLAN_SERVICE_BIZ_SMOKE_HEALTH_SLEEP_SECS:-0.5}"
HTTP_TIMEOUT_SECS="${SLAN_SERVICE_BIZ_SMOKE_HTTP_TIMEOUT_SECS:-10}"
TRACE_HTTP="${SLAN_SERVICE_BIZ_SMOKE_TRACE_HTTP:-0}"

log() {
  printf '[service_biz_smoke] %s\n' "$*"
}

fail() {
  log "ERROR: $*"
  if [[ -f /tmp/slan-service-biz-smoke.log ]]; then
    log "last service log lines:"
    tail -n 60 /tmp/slan-service-biz-smoke.log >&2 || true
  fi
  exit 1
}

http_status() {
  curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
    --max-time "${HTTP_TIMEOUT_SECS}" "$@"
}

allow_remote_internal_skip() {
  local status="${1:-}"
  [[ "${START_LOCAL_BIZ}" == "0" && ( "${status}" == "401" || "${status}" == "403" ) ]]
}

http_call() {
  local capture_file="${1:-}"
  shift

  local -a args=(
    --silent
    --show-error
    --fail
    --max-time "${HTTP_TIMEOUT_SECS}"
  )
  if [[ "${TRACE_HTTP}" == "1" ]]; then
    args+=(--verbose)
  fi
  if [[ -n "${capture_file}" ]]; then
    args+=(--output "${capture_file}")
  fi

  curl "${args[@]}" "$@"
}

http_json() {
  local body
  if ! body="$(http_call "" "$@")"; then
    fail "request failed: curl $*"
  fi
  printf '%s' "${body}"
}

wait_for_http() {
  local name="$1"
  shift

  local attempt=1
  while (( attempt <= HEALTH_RETRIES )); do
    if http_call "" "$@" >/dev/null 2>&1; then
      log "${name} is ready after ${attempt} attempt(s)"
      return 0
    fi
    sleep "${HEALTH_SLEEP_SECS}"
    attempt=$((attempt + 1))
  done

  fail "${name} did not become ready after ${HEALTH_RETRIES} attempts: curl $*"
}

cleanup() {
  if [[ -n "${OPS_TOKEN:-}" ]]; then
    slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "${NETWORK_ID:-}" 2>/dev/null || true
    for group_id in "${DEVICE_GROUP_ID:-}" "${MEMBER_GROUP_ID:-}"; do
      [[ -n "$group_id" ]] && curl --silent --show-error -X DELETE \
        "$OPS_BASE_URL/api/ops/device-groups/$group_id" \
        -H "Authorization: Bearer $OPS_TOKEN" >/dev/null 2>&1 || true
    done
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "${CREDENTIAL_ID:-}" 2>/dev/null || true
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "${DST_CREDENTIAL_ID:-}" 2>/dev/null || true
    for device_id in "${DEVICE_ID:-}" "${DST_DEVICE_ID:-}"; do
      [[ -n "$device_id" ]] && curl --silent --show-error -X DELETE \
        "$OPS_BASE_URL/api/ops/devices/$device_id" \
        -H "Authorization: Bearer $OPS_TOKEN" >/dev/null 2>&1 || true
    done
  fi
  if [[ -n "${BIZ_PID:-}" ]]; then
    kill "${BIZ_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ "${START_LOCAL_BIZ}" != "0" ]]; then
  cd "${ROOT_DIR}/server/service-biz"

  log "starting local service-biz on 127.0.0.1:${PORT}"
  SLAN_BIZ_ADDR="127.0.0.1:${PORT}" \
  SLAN_INTERNAL_WIRE_TOKEN="${WIRE_TOKEN}" \
  go run ./cmd/service-biz >/tmp/slan-service-biz-smoke.log 2>&1 &
  BIZ_PID=$!

  wait_for_http \
    "service-biz internal wire admin" \
    "${APP_BASE_URL}/internal/wire/admin/relay-nodes" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}"
fi

wait_for_http "service-biz healthz" "${APP_BASE_URL}/healthz"

if [[ "${SEED_WIRE_NODES}" == "1" ]]; then
  log "seeding smoke relay and derp nodes"
  http_call "" -X PUT "${APP_BASE_URL}/internal/wire/admin/relay-nodes" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"regionId":"smoke","nodeId":"relay-smoke","host":"127.0.0.1","udpPort":29110,"adminPort":29111,"enabled":true,"healthy":true,"priority":10}' >/dev/null || fail "failed to seed relay node"
  http_call "" -X POST "${APP_BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/heartbeat" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null || fail "failed to heartbeat relay node"
  http_call "" -X PATCH "${APP_BASE_URL}/internal/wire/admin/relay-nodes/smoke/relay-smoke/status" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"enabled":true,"healthy":true}' >/dev/null || fail "failed to update relay node status"

  http_call "" -X PUT "${APP_BASE_URL}/internal/wire/admin/derp-nodes" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"regionId":"smoke","nodeId":"derp-smoke","name":"Smoke DERP","host":"127.0.0.1","port":29120,"enabled":true,"healthy":true,"priority":10}' >/dev/null || fail "failed to seed derp node"
  http_call "" -X POST "${APP_BASE_URL}/internal/wire/admin/derp-nodes/smoke/derp-smoke/heartbeat" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"healthy":true,"ticketKeyRotation":{"source":"smoke","signingConfigured":true,"keyRingConfigured":true,"effectiveKeyCount":1,"rotationReady":true}}' >/dev/null || fail "failed to heartbeat derp node"
fi
DERP_MAP_STATUS="$(http_status "${APP_BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if [[ "${DERP_MAP_STATUS}" == "200" ]]; then
  DERP_MAP="$(http_json "${APP_BASE_URL}/internal/wire/derp-map" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
  if [[ "${SEED_WIRE_NODES}" == "1" ]]; then
    DERP_EXPECTATION='"nodeId":"derp-smoke"'
  else
    DERP_EXPECTATION='"nodeId":"'
  fi
  if ! printf '%s' "${DERP_MAP}" | grep -q "${DERP_EXPECTATION}"; then
    fail "derp map missing expected node: ${DERP_MAP}"
  fi
elif [[ "${START_LOCAL_BIZ}" == "0" && ( "${DERP_MAP_STATUS}" == "401" || "${DERP_MAP_STATUS}" == "403" ) ]]; then
  log "skipping derp map assertion for remote edge status=${DERP_MAP_STATUS}"
else
  fail "derp map request failed with status ${DERP_MAP_STATUS}"
fi

log "provisioning Ops-managed app-plane resources"
OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
SOURCE_DEVICE="$(http_json -X POST "${OPS_BASE_URL}/api/ops/devices" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Mac ${RUN_ID}\",\"alias\":\"Smoke Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"smoke\",\"publicKey\":\"smoke-mac-key-${RUN_ID}\"}")"
DEVICE_ID="$(printf '%s' "$SOURCE_DEVICE" | jq -er '.deviceId')"
DST_DEVICE="$(http_json -X POST "${OPS_BASE_URL}/api/ops/devices" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke iPhone ${RUN_ID}\",\"alias\":\"Smoke iPhone\",\"platform\":\"ios\",\"osName\":\"iOS\",\"osVersion\":\"smoke\",\"publicKey\":\"smoke-ios-key-${RUN_ID}\"}")"
DST_DEVICE_ID="$(printf '%s' "$DST_DEVICE" | jq -er '.deviceId')"
CREDENTIAL="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Smoke Mac" "$DEVICE_ID")"
CREDENTIAL_ID="$(printf '%s' "$CREDENTIAL" | jq -er '.credentialId')"
AUTHORIZATION_KEY="$(printf '%s' "$CREDENTIAL" | jq -er '.key')"
DST_CREDENTIAL="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Smoke iPhone" "$DST_DEVICE_ID")"
DST_CREDENTIAL_ID="$(printf '%s' "$DST_CREDENTIAL" | jq -er '.credentialId')"
DST_AUTHORIZATION_KEY="$(printf '%s' "$DST_CREDENTIAL" | jq -er '.key')"

SOURCE_SESSION="$(http_json -X POST "${APP_BASE_URL}/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"${AUTHORIZATION_KEY}\",\"deviceId\":\"${DEVICE_ID}\"}")"
DEVICE_TOKEN="$(printf '%s' "$SOURCE_SESSION" | jq -er '.deviceSession.deviceToken')"
DST_SESSION="$(http_json -X POST "${APP_BASE_URL}/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"${DST_AUTHORIZATION_KEY}\",\"deviceId\":\"${DST_DEVICE_ID}\"}")"
DST_DEVICE_TOKEN="$(printf '%s' "$DST_SESSION" | jq -er '.deviceSession.deviceToken')"

NETWORK="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "Smoke Network ${RUN_ID}")"
NETWORK_ID="$(printf '%s' "$NETWORK" | jq -er '.networkId')"
MEMBER_GROUP="$(http_json -X POST "${OPS_BASE_URL}/api/ops/device-groups" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Network Members","description":"network membership smoke"}')"
MEMBER_GROUP_ID="$(printf '%s' "$MEMBER_GROUP" | jq -er '.groupId')"
for member_device_id in "${DEVICE_ID}" "${DST_DEVICE_ID}"; do
  http_call "" -X POST "${OPS_BASE_URL}/api/ops/device-groups/${MEMBER_GROUP_ID}/devices" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"deviceId\":\"${member_device_id}\"}" >/dev/null || fail "failed to assign network member group"
done
http_call "" -X POST "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"groupId\":\"${MEMBER_GROUP_ID}\"}" >/dev/null || fail "failed to reference network member group"

http_call "" "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/mqtt-credential" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" >/dev/null || fail "mqtt credential lookup failed"
http_call "" -X POST "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/renew" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"networkEnabled":true,"rxBytesTotal":1024,"txBytesTotal":2048}' >/dev/null || fail "source device renew failed"
http_call "" -X POST "${APP_BASE_URL}/api/app/devices/${DST_DEVICE_ID}/renew" \
  -H "Authorization: Bearer ${DST_DEVICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"networkEnabled":true,"rxBytesTotal":256,"txBytesTotal":512}' >/dev/null || fail "destination device renew failed"
http_call "" "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/network-configs" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" >/dev/null || fail "device network configs failed"
http_call "" "${APP_BASE_URL}/api/app/networks/${NETWORK_ID}/relay-candidates?deviceId=${DEVICE_ID}" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" >/dev/null || fail "relay candidates failed"
PEER_ID="${NETWORK_ID}:${DEVICE_ID}"
PEER_AUTHZ_STATUS="$(http_status "${APP_BASE_URL}/internal/wire/peers/${PEER_ID}/authz" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if [[ "${PEER_AUTHZ_STATUS}" == "200" ]]; then
  :
elif allow_remote_internal_skip "${PEER_AUTHZ_STATUS}"; then
  log "skipping peer authz assertion for remote edge status=${PEER_AUTHZ_STATUS}"
else
  fail "peer authz lookup failed with status ${PEER_AUTHZ_STATUS}"
fi
RUNTIME_CONFIG_STATUS="$(http_status "${APP_BASE_URL}/internal/wire/peers/node-${DEVICE_ID}/runtime-config" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if [[ "${RUNTIME_CONFIG_STATUS}" == "200" ]]; then
  :
elif allow_remote_internal_skip "${RUNTIME_CONFIG_STATUS}"; then
  log "skipping runtime config assertion for remote edge status=${RUNTIME_CONFIG_STATUS}"
else
  fail "runtime config lookup failed with status ${RUNTIME_CONFIG_STATUS}"
fi
TOPOLOGY_STATUS="$(http_status "${APP_BASE_URL}/internal/wire/networks/${NETWORK_ID}/topology" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if [[ "${TOPOLOGY_STATUS}" == "200" ]]; then
  TOPOLOGY="$(http_json "${APP_BASE_URL}/internal/wire/networks/${NETWORK_ID}/topology" \
    -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
  if ! printf '%s' "${TOPOLOGY}" | grep -q "\"peerId\":\"${NETWORK_ID}:${DST_DEVICE_ID}\""; then
    fail "network topology missing destination peer: ${TOPOLOGY}"
  fi
elif allow_remote_internal_skip "${TOPOLOGY_STATUS}"; then
  log "skipping network topology assertion for remote edge status=${TOPOLOGY_STATUS}"
else
  fail "network topology lookup failed with status ${TOPOLOGY_STATUS}"
fi
RELAY_TICKET="$(http_json -X POST "${APP_BASE_URL}/api/app/relay/tickets" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke\"}")"
RELAY_TICKET_ID="$(printf '%s' "${RELAY_TICKET}" | sed -n 's/.*"ticketId":"\([^"]*\)".*/\1/p')"
if [[ -z "${RELAY_TICKET_ID}" ]]; then
  fail "relay ticket was not issued: ${RELAY_TICKET}"
fi

log "running Ops policy checks"
SECURITY_GROUPS="$(http_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/security-groups" \
  -H "Authorization: Bearer ${OPS_TOKEN}")"
SECURITY_GROUP_ID="$(printf '%s' "${SECURITY_GROUPS}" | sed -n 's/.*"securityGroupId":"\([^"]*\)".*/\1/p')"
if [[ -z "${SECURITY_GROUP_ID}" ]]; then
  SECURITY_GROUP="$(http_json -X POST "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/security-groups" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d '{"name":"Smoke Security Group","description":"service smoke policy"}')"
  SECURITY_GROUP_ID="$(printf '%s' "$SECURITY_GROUP" | jq -er '.securityGroupId')"
fi
DEVICE_GROUP_JSON="$(http_json -X POST "${OPS_BASE_URL}/api/ops/device-groups" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke ACL Group","description":"device group smoke"}')"
DEVICE_GROUP_ID="$(printf '%s' "${DEVICE_GROUP_JSON}" | sed -n 's/.*"groupId":"\([^"]*\)".*/\1/p')"
if [[ -z "${DEVICE_GROUP_ID}" ]]; then
  fail "missing device group id: ${DEVICE_GROUP_JSON}"
fi
http_call "" -X POST "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}/devices" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"deviceId\":\"${DEVICE_ID}\"}" >/dev/null || fail "failed to assign device group"
http_call "" -X POST "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"groupId\":\"${DEVICE_GROUP_ID}\"}" >/dev/null || fail "failed to reference ACL device group"
GROUP_RULE="$(http_json -X POST "${OPS_BASE_URL}/api/ops/security-groups/${SECURITY_GROUP_ID}/rules" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":4,\"action\":\"allow\",\"protocol\":\"tcp\",\"portRange\":\"443\",\"peerType\":\"device_group\",\"peerValue\":\"${DEVICE_GROUP_ID}\",\"description\":\"allow smoke group ingress\",\"enabled\":true}")"
GROUP_RULE_ID="$(printf '%s' "${GROUP_RULE}" | sed -n 's/.*"ruleId":"\([^"]*\)".*/\1/p')"
if [[ -z "${GROUP_RULE_ID}" ]]; then
  fail "missing device_group security rule id: ${GROUP_RULE}"
fi
GROUP_RULE_UPDATED="$(http_json -X PATCH "${OPS_BASE_URL}/api/ops/security-rules/${GROUP_RULE_ID}" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":5,\"action\":\"allow\",\"protocol\":\"tcp\",\"portRange\":\"443\",\"peerType\":\"device_group\",\"peerValue\":\"${DEVICE_GROUP_ID}\",\"description\":\"updated smoke group ingress\",\"enabled\":true}")"
if ! printf '%s' "${GROUP_RULE_UPDATED}" | grep -q '"priority":5'; then
  fail "security rule update did not persist priority: ${GROUP_RULE_UPDATED}"
fi
if ! printf '%s' "${GROUP_RULE_UPDATED}" | grep -q '"description":"updated smoke group ingress"'; then
  fail "security rule update did not persist description: ${GROUP_RULE_UPDATED}"
fi
GROUP_RULE_CONFIG="$(http_json "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/network-configs" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}")"
if ! printf '%s' "${GROUP_RULE_CONFIG}" | grep -q "\"ruleId\":\"${GROUP_RULE_ID}\""; then
  fail "device_group ACL rule missing from network config: ${GROUP_RULE_CONFIG}"
fi
if ! printf '%s' "${GROUP_RULE_CONFIG}" | grep -q "\"resolvedPeerNodeId\":\"node-${DEVICE_ID}\""; then
  fail "device_group ACL rule missing resolved peer node: ${GROUP_RULE_CONFIG}"
fi
http_call "" -X DELETE "${OPS_BASE_URL}/api/ops/security-rules/${GROUP_RULE_ID}" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null || fail "failed to delete device_group rule"

INGRESS_DENY_RULE="$(http_json -X POST "${OPS_BASE_URL}/api/ops/security-groups/${SECURITY_GROUP_ID}/rules" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":5,\"action\":\"deny\",\"protocol\":\"all\",\"portRange\":\"\",\"peerType\":\"device\",\"peerValue\":\"${DEVICE_ID}\",\"description\":\"deny smoke peer ingress\",\"enabled\":true}")"
INGRESS_DENY_RULE_ID="$(printf '%s' "${INGRESS_DENY_RULE}" | sed -n 's/.*"ruleId":"\([^"]*\)".*/\1/p')"
if [[ -z "${INGRESS_DENY_RULE_ID}" ]]; then
  fail "missing ingress deny security rule id: ${INGRESS_DENY_RULE}"
fi
INGRESS_DENIED_CONFIG="$(http_json "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/network-configs" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}")"
if ! printf '%s' "${INGRESS_DENIED_CONFIG}" | grep -q "\"ruleId\":\"${INGRESS_DENY_RULE_ID}\""; then
  fail "ingress ACL deny rule missing from network config: ${INGRESS_DENIED_CONFIG}"
fi
if ! printf '%s' "${INGRESS_DENIED_CONFIG}" | grep -q "\"resolvedPeerNodeId\":\"node-${DEVICE_ID}\""; then
  fail "ingress ACL deny rule missing resolved peer node: ${INGRESS_DENIED_CONFIG}"
fi
INGRESS_DENIED_TICKET_STATUS="$(curl --silent --show-error --output /tmp/slan-service-biz-ingress-denied-ticket.json --write-out '%{http_code}' --max-time "${HTTP_TIMEOUT_SECS}" -X POST "${APP_BASE_URL}/api/app/relay/tickets" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke-ingress-denied\"}")"
if [[ "${INGRESS_DENIED_TICKET_STATUS}" == "200" || "${INGRESS_DENIED_TICKET_STATUS}" == "201" ]]; then
  fail "ingress ACL deny still allowed relay ticket: $(cat /tmp/slan-service-biz-ingress-denied-ticket.json)"
fi
http_call "" -X DELETE "${OPS_BASE_URL}/api/ops/security-rules/${INGRESS_DENY_RULE_ID}" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null || fail "failed to delete ingress deny rule"

http_call "" -X POST "${OPS_BASE_URL}/api/ops/security-groups/${SECURITY_GROUP_ID}/rules" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"direction\":\"egress\",\"priority\":10,\"action\":\"deny\",\"protocol\":\"all\",\"portRange\":\"\",\"peerType\":\"device\",\"peerValue\":\"${DST_DEVICE_ID}\",\"description\":\"deny smoke peer\",\"enabled\":true}" >/dev/null || fail "failed to create egress deny rule"
DENIED_CONFIG="$(http_json "${APP_BASE_URL}/api/app/devices/${DEVICE_ID}/network-configs" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}")"
if ! printf '%s' "${DENIED_CONFIG}" | grep -q '"securityRuleCount":1'; then
  fail "ACL deny rule count missing from network config: ${DENIED_CONFIG}"
fi
if ! printf '%s' "${DENIED_CONFIG}" | grep -q "\"resolvedPeerNodeId\":\"node-${DST_DEVICE_ID}\""; then
  fail "ACL deny rule missing resolved peer node: ${DENIED_CONFIG}"
fi
DENIED_TICKET_STATUS="$(curl --silent --show-error --output /tmp/slan-service-biz-denied-ticket.json --write-out '%{http_code}' --max-time "${HTTP_TIMEOUT_SECS}" -X POST "${APP_BASE_URL}/api/app/relay/tickets" \
  -H "Authorization: Bearer ${DEVICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"srcNodeId\":\"node-${DEVICE_ID}\",\"dstNodeId\":\"node-${DST_DEVICE_ID}\",\"reason\":\"smoke-denied\"}")"
if [[ "${DENIED_TICKET_STATUS}" == "200" || "${DENIED_TICKET_STATUS}" == "201" ]]; then
  fail "ACL deny still allowed relay ticket: $(cat /tmp/slan-service-biz-denied-ticket.json)"
fi

log "running ops-plane checks"
http_call "" "${OPS_BASE_URL}/api/ops/operators" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null || fail "ops operators list failed"
http_call "" "${OPS_BASE_URL}/api/ops/relay-nodes" \
  -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null || fail "ops relay nodes list failed"
FINAL_DERP_MAP_STATUS="$(http_status "${APP_BASE_URL}/internal/wire/derp-map" \
  -H "X-Slan-Internal-Token: ${WIRE_TOKEN}")"
if [[ "${FINAL_DERP_MAP_STATUS}" == "200" ]]; then
  :
elif [[ "${START_LOCAL_BIZ}" == "0" && ( "${FINAL_DERP_MAP_STATUS}" == "401" || "${FINAL_DERP_MAP_STATUS}" == "403" ) ]]; then
  log "skipping final derp map assertion for remote edge status=${FINAL_DERP_MAP_STATUS}"
else
  fail "final derp map lookup failed with status ${FINAL_DERP_MAP_STATUS}"
fi

echo "service-biz smoke passed app=${APP_BASE_URL} web=${WEB_BASE_URL} ops=${OPS_BASE_URL}"
