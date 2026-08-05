#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do ROOT_DIR=$(dirname "$ROOT_DIR"); done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"

HOST="${1:-${SLAN_REMOTE_HOST:-$SLAN_DEFAULT_MQTT_HOST}}"
OPS_BASE="${SLAN_REMOTE_OPS_BASE:-http://${HOST}:24201}"
BIZ_BASE="${SLAN_REMOTE_BIZ_BASE:-http://${HOST}:28080}"
RUN_ID="$(date +%s%N)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/slan-remote-ui-smoke.XXXXXX")"
OPS_TOKEN=""
CREDENTIAL_ID=""
DEVICE_ID="remote-ui-${RUN_ID}-mac"
NETWORK_ID=""
DEVICE_GROUP_ID=""
SECURITY_GROUP_ID=""
RULE_ID=""
ZONE_ID=""
RECORD_ID=""

curl() { command curl --connect-timeout 5 --max-time 20 "$@"; }
fail() { echo "remote ui ops smoke failed: $*" >&2; exit 1; }
auth_curl() { curl --silent --show-error --fail -H "Authorization: Bearer ${OPS_TOKEN}" "$@"; }
best_effort() { command curl --silent --connect-timeout 5 --max-time 20 "$@" >/dev/null 2>&1 || true; }

cleanup() {
  if [[ -n "$OPS_TOKEN" ]]; then
    [[ -n "$RULE_ID" ]] && best_effort -X DELETE "$OPS_BASE/api/ops/security-rules/$RULE_ID" -H "Authorization: Bearer $OPS_TOKEN"
    [[ -n "$SECURITY_GROUP_ID" ]] && best_effort -X DELETE "$OPS_BASE/api/ops/security-groups/$SECURITY_GROUP_ID" -H "Authorization: Bearer $OPS_TOKEN"
    [[ -n "$RECORD_ID" ]] && best_effort -X DELETE "$OPS_BASE/api/ops/dns/records/$RECORD_ID" -H "Authorization: Bearer $OPS_TOKEN"
    [[ -n "$ZONE_ID" ]] && best_effort -X DELETE "$OPS_BASE/api/ops/dns/zones/$ZONE_ID" -H "Authorization: Bearer $OPS_TOKEN"
    slan_ops_delete_network "$OPS_BASE" "$OPS_TOKEN" "$NETWORK_ID" 2>/dev/null || true
    [[ -n "$DEVICE_GROUP_ID" ]] && best_effort -X DELETE "$OPS_BASE/api/ops/device-groups/$DEVICE_GROUP_ID" -H "Authorization: Bearer $OPS_TOKEN"
    slan_ops_revoke_device_credential "$OPS_BASE" "$OPS_TOKEN" "$CREDENTIAL_ID" 2>/dev/null || true
    best_effort -X DELETE "$OPS_BASE/api/ops/devices/$DEVICE_ID" -H "Authorization: Bearer $OPS_TOKEN"
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

command -v jq >/dev/null 2>&1 || fail "jq is required"

echo "==> UI shell checks"
curl --silent --fail "$OPS_BASE/" | grep -q '<ops-root' || fail "ops console root did not render ops-root"
curl --silent --fail "$BIZ_BASE/healthz" | grep -q '"status":"ok"' || fail "biz healthz failed"

echo "==> Ops resource checks"
OPS_TOKEN="$(slan_ops_login "$OPS_BASE")"
for endpoint in dashboard operators relay-nodes punch-nodes customers devices device-credentials networks device-groups audit-events; do
  auth_curl "$OPS_BASE/api/ops/$endpoint" >/dev/null || fail "Ops list failed: $endpoint"
done

CREDENTIAL="$(slan_ops_create_device_credential "$OPS_BASE" "$OPS_TOKEN" "Remote UI Device")"
CREDENTIAL_ID="$(printf '%s' "$CREDENTIAL" | jq -er '.credentialId')"
AUTHORIZATION_KEY="$(printf '%s' "$CREDENTIAL" | jq -er '.key')"
SESSION="$(curl --silent --show-error --fail -X POST "$BIZ_BASE/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"$AUTHORIZATION_KEY\",\"deviceId\":\"$DEVICE_ID\"}")" || fail "authorization key exchange failed"
DEVICE_TOKEN="$(printf '%s' "$SESSION" | jq -er '.deviceSession.deviceToken')"

NETWORK="$(slan_ops_create_network "$OPS_BASE" "$OPS_TOKEN" "Remote Smoke Network $RUN_ID")"
NETWORK_ID="$(printf '%s' "$NETWORK" | jq -er '.networkId')"
auth_curl -X PATCH "$OPS_BASE/api/ops/networks/$NETWORK_ID" -H 'Content-Type: application/json' \
  -d '{"name":"Remote Smoke Network Updated","status":"active"}' >/dev/null || fail "network update failed"

DEVICE_GROUP="$(auth_curl -X POST "$OPS_BASE/api/ops/device-groups" -H 'Content-Type: application/json' \
  -d '{"name":"Remote ACL Group","description":"remote smoke device group"}')" || fail "device group create failed"
DEVICE_GROUP_ID="$(printf '%s' "$DEVICE_GROUP" | jq -er '.groupId')"
auth_curl -X POST "$OPS_BASE/api/ops/device-groups/$DEVICE_GROUP_ID/devices" -H 'Content-Type: application/json' \
  -d "{\"deviceId\":\"$DEVICE_ID\"}" >/dev/null || fail "device group member add failed"
auth_curl -X POST "$OPS_BASE/api/ops/networks/$NETWORK_ID/device-groups" -H 'Content-Type: application/json' \
  -d "{\"groupId\":\"$DEVICE_GROUP_ID\"}" >/dev/null || fail "network device group add failed"

ZONE="$(auth_curl -X POST "$OPS_BASE/api/ops/networks/$NETWORK_ID/dns/zones" -H 'Content-Type: application/json' \
  -d "{\"name\":\"remote-${RUN_ID}.staticlss.com\"}")" || fail "dns zone create failed"
ZONE_ID="$(printf '%s' "$ZONE" | jq -er '.zoneId')"
RECORD="$(auth_curl -X POST "$OPS_BASE/api/ops/networks/$NETWORK_ID/dns/records" -H 'Content-Type: application/json' \
  -d "{\"zoneId\":\"$ZONE_ID\",\"name\":\"app\",\"type\":\"A\",\"value\":\"$DEVICE_ID\",\"ttl\":60}")" || fail "dns record create failed"
RECORD_ID="$(printf '%s' "$RECORD" | jq -er '.recordId')"
auth_curl -X PATCH "$OPS_BASE/api/ops/dns/records/$RECORD_ID" -H 'Content-Type: application/json' \
  -d "{\"zoneId\":\"$ZONE_ID\",\"name\":\"app2\",\"type\":\"A\",\"value\":\"$DEVICE_ID\",\"ttl\":120}" >/dev/null || fail "dns record update failed"

SECURITY_GROUP="$(auth_curl -X POST "$OPS_BASE/api/ops/networks/$NETWORK_ID/security-groups" -H 'Content-Type: application/json' \
  -d '{"name":"Remote Smoke ACL","description":"remote smoke"}')" || fail "security group create failed"
SECURITY_GROUP_ID="$(printf '%s' "$SECURITY_GROUP" | jq -er '.securityGroupId')"
RULE="$(auth_curl -X POST "$OPS_BASE/api/ops/security-groups/$SECURITY_GROUP_ID/rules" -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":100,\"action\":\"allow\",\"protocol\":\"tcp\",\"portRange\":\"443\",\"peerType\":\"device_group\",\"peerValue\":\"$DEVICE_GROUP_ID\",\"description\":\"remote smoke\",\"enabled\":true}")" || fail "security rule create failed"
RULE_ID="$(printf '%s' "$RULE" | jq -er '.ruleId')"
auth_curl -X PATCH "$OPS_BASE/api/ops/security-rules/$RULE_ID" -H 'Content-Type: application/json' \
  -d "{\"direction\":\"ingress\",\"priority\":110,\"action\":\"allow\",\"protocol\":\"tcp\",\"portRange\":\"8443\",\"peerType\":\"device_group\",\"peerValue\":\"$DEVICE_GROUP_ID\",\"description\":\"remote smoke updated\",\"enabled\":true}" >/dev/null || fail "security rule update failed"

CONFIG="$(curl --silent --show-error --fail "$BIZ_BASE/api/app/devices/$DEVICE_ID/network-configs" \
  -H "Authorization: Bearer $DEVICE_TOKEN")" || fail "device network configs failed"
printf '%s' "$CONFIG" | grep -q "\"ruleId\":\"$RULE_ID\"" || fail "network config missing security rule"
printf '%s' "$CONFIG" | grep -q "\"resolvedPeerNodeId\":\"node-$DEVICE_ID\"" || fail "network config missing device-group expansion"

echo "remote ui ops smoke passed"
echo "ops=$OPS_BASE"
echo "biz=$BIZ_BASE"
