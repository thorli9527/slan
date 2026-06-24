#!/usr/bin/env bash
set -euo pipefail

HOST="${1:-${SLAN_REMOTE_HOST:-47.245.40.231}}"
WEB_BASE="${SLAN_REMOTE_WEB_BASE:-http://${HOST}:24200}"
OPS_BASE="${SLAN_REMOTE_OPS_BASE:-http://${HOST}:24201}"
BIZ_BASE="${SLAN_REMOTE_BIZ_BASE:-http://${HOST}:28080}"
RUN_ID="$(date +%s)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/slan-remote-ui-smoke.XXXXXX")"
USER_ID=""
USER_EMAIL=""
USER_TOKEN=""
DEVICE_ID=""
SECOND_USER_ID=""
SECOND_USER_EMAIL=""
SECOND_DEVICE_ID=""
BOOTSTRAP_ID=""
WORKSPACE_ID=""
ZONE_ID=""
RECORD_ID=""
MAPPING_ID=""
GROUP_ID=""
RULE_ID=""
OPS_TOKEN=""
OPERATOR_ID=""
PLAN_CODE=""
PRODUCT_ID=""
RELAY_NODE_ID=""
DOWNLOAD_ID=""

best_effort_curl() {
  command curl --silent --show-error --connect-timeout 5 --max-time 20 "$@" >/dev/null 2>&1 || true
}

best_effort_ops_json() {
  local method="$1"
  local url="$2"
  local payload="$3"
  [[ -n "${OPS_TOKEN}" ]] || return 0
  best_effort_curl -X "${method}" "${url}" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${payload}"
}

cleanup() {
  if [[ -n "${OPS_TOKEN}" ]]; then
    [[ -n "${DOWNLOAD_ID}" ]] && best_effort_curl -X DELETE "${OPS_BASE}/api/ops/client-downloads/${DOWNLOAD_ID}" -H "Authorization: Bearer ${OPS_TOKEN}"
    [[ -n "${RELAY_NODE_ID}" ]] && best_effort_curl -X DELETE "${OPS_BASE}/api/ops/relay-nodes/${RELAY_NODE_ID}" -H "Authorization: Bearer ${OPS_TOKEN}"
    [[ -n "${PRODUCT_ID}" ]] && best_effort_ops_json PATCH "${OPS_BASE}/api/ops/products/${PRODUCT_ID}" "{\"name\":\"Remote Smoke Product Disabled\",\"type\":\"plan\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"monthly\",\"validDays\":31,\"relayTrafficGb\":0,\"relayBandwidthMbps\":0,\"listPrice\":0,\"salePrice\":0,\"currency\":\"CNY\",\"autoRenew\":false,\"status\":\"disabled\",\"description\":\"remote smoke cleanup\"}"
    [[ -n "${PLAN_CODE}" ]] && best_effort_ops_json PATCH "${OPS_BASE}/api/ops/plans/${PLAN_CODE}" '{"name":"Remote Smoke Plan Disabled","ownDeviceLimit":0,"invitedDeviceLimit":0,"totalDeviceLimit":0,"relayMonthlyGb":0,"relayBandwidthMbps":0,"relayThrottleMbps":0,"p2pUnlimited":false,"customDomain":false,"acl":false,"dedicatedRelay":false,"auditLog":false,"apiAccess":false,"monthlyPrice":0,"yearlyPrice":0,"status":"disabled"}'
    [[ -n "${OPERATOR_ID}" ]] && best_effort_ops_json PATCH "${OPS_BASE}/api/ops/operators/${OPERATOR_ID}" "{\"name\":\"Remote Smoke Ops Disabled\",\"email\":\"remote-ops-${RUN_ID}@staticlss.com\",\"role\":\"ops\",\"status\":\"disabled\"}"
    [[ -n "${USER_ID}" && -n "${USER_EMAIL}" ]] && best_effort_ops_json PATCH "${OPS_BASE}/api/ops/customers/${USER_ID}" "{\"email\":\"${USER_EMAIL}\",\"name\":\"Remote UI Smoke Disabled\",\"status\":\"disabled\"}"
    [[ -n "${SECOND_USER_ID}" && -n "${SECOND_USER_EMAIL}" ]] && best_effort_ops_json PATCH "${OPS_BASE}/api/ops/customers/${SECOND_USER_ID}" "{\"email\":\"${SECOND_USER_EMAIL}\",\"name\":\"Remote UI Peer Disabled\",\"status\":\"disabled\"}"
  fi
  [[ -n "${RULE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/security-groups/rules/${RULE_ID}"
  [[ -n "${GROUP_ID}" && -n "${WORKSPACE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/security-groups/${GROUP_ID}"
  [[ -n "${MAPPING_ID}" && -n "${WORKSPACE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/public-mappings/${MAPPING_ID}"
  [[ -n "${RECORD_ID}" && -n "${WORKSPACE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/records/${RECORD_ID}"
  [[ -n "${ZONE_ID}" && -n "${WORKSPACE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/zones/${ZONE_ID}"
  [[ -n "${DEVICE_ID}" && -n "${WORKSPACE_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/devices/${DEVICE_ID}"
  [[ -n "${WORKSPACE_ID}" ]] && best_effort_curl -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}" -H 'Content-Type: application/json' -d "{\"name\":\"Remote Smoke Network Disabled\",\"code\":\"rsmoke-${RUN_ID}\",\"status\":\"disabled\"}"
  if [[ -n "${BOOTSTRAP_ID}" && -n "${USER_TOKEN}" && -n "${USER_ID}" ]]; then
    best_effort_curl -X POST "${WEB_BASE}/api/web/device-bootstrap-keys/${BOOTSTRAP_ID}/revoke" \
      -H "Authorization: Bearer ${USER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"userId\":\"${USER_ID}\"}"
  fi
  [[ -n "${DEVICE_ID}" && -n "${USER_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/devices/${DEVICE_ID}?actorUserId=${USER_ID}"
  [[ -n "${SECOND_DEVICE_ID}" && -n "${SECOND_USER_ID}" ]] && best_effort_curl -X DELETE "${WEB_BASE}/api/devices/${SECOND_DEVICE_ID}?actorUserId=${SECOND_USER_ID}"
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

fail() {
  echo "remote ui ops smoke failed: $*" >&2
  exit 1
}

curl() {
  command curl --connect-timeout 5 --max-time 20 "$@"
}

json_value() {
  local key="$1"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p"
}

json_number() {
  local key="$1"
  sed -n "s/.*\"${key}\":\\([0-9][0-9]*\\).*/\\1/p"
}

auth_curl() {
  curl --silent --fail -H "Authorization: Bearer ${OPS_TOKEN}" "$@"
}

echo "==> UI shell checks"
curl --silent --fail "${WEB_BASE}/" | grep -q '<app-root' || fail "web console root did not render app-root"
curl --silent --fail "${OPS_BASE}/" | grep -q '<ops-root' || fail "ops console root did not render ops-root"
curl --silent --fail "${BIZ_BASE}/healthz" | grep -q '"status":"ok"' || fail "biz healthz failed"

echo "==> Customer web API through web UI proxy"
USER_EMAIL="remote-ui-${RUN_ID}@staticlss.com"
USER_AUTH="$(curl --silent --fail -X POST "${WEB_BASE}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${USER_EMAIL}\",\"password\":\"password123\",\"name\":\"Remote UI Smoke\"}")" || fail "web register failed"
USER_ID="$(printf '%s' "${USER_AUTH}" | json_value userId)"
USER_TOKEN="$(printf '%s' "${USER_AUTH}" | json_value token)"
NETWORK_ID="$(printf '%s' "${USER_AUTH}" | json_value networkId)"
[[ -n "${USER_ID}" && -n "${USER_TOKEN}" && -n "${NETWORK_ID}" ]] || fail "web register missing user/token/network"

curl --silent --fail -X POST "${WEB_BASE}/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${USER_EMAIL}\",\"password\":\"password123\"}" >/dev/null || fail "web login failed"

DEVICE_ID="remote-ui-${RUN_ID}-mac"
DEVICE_REGISTER="$(curl --silent --fail -X POST "${WEB_BASE}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DEVICE_ID}\",\"name\":\"Remote UI Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"15.3\",\"alias\":\"Remote UI Mac\",\"publicKey\":\"remote-ui-public-key-${RUN_ID}\"}")" || fail "web device register failed"
printf '%s' "${DEVICE_REGISTER}" | grep -q '"globalIp":"10\.' || fail "device register missing global IP"
curl --silent --fail "${BIZ_BASE}/api/devices/${DEVICE_ID}/mqtt-credential" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null || fail "mqtt credential failed"
curl --silent --fail -X POST "${BIZ_BASE}/api/devices/${DEVICE_ID}/renew" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkEnabled\":true,\"rxBytesTotal\":1024,\"txBytesTotal\":2048}" >/dev/null || fail "device renew failed"
curl --silent --fail "${BIZ_BASE}/api/devices/${DEVICE_ID}/network-configs" >/dev/null || fail "device network configs failed"
curl --silent --fail "${BIZ_BASE}/api/networks/${NETWORK_ID}/network-config?deviceId=${DEVICE_ID}" >/dev/null || fail "network config failed"
curl --silent --fail "${BIZ_BASE}/api/networks/${NETWORK_ID}/relay-candidates?deviceId=${DEVICE_ID}" >/dev/null || fail "relay candidates failed"
curl --silent --fail "${WEB_BASE}/api/client-downloads" >/dev/null || fail "public client downloads failed"

curl --silent --fail "${WEB_BASE}/api/devices/visible?userId=${USER_ID}" >/dev/null || fail "visible devices failed"
curl --silent --fail "${WEB_BASE}/api/networks?userId=${USER_ID}" >/dev/null || fail "networks list failed"
curl --silent --fail "${WEB_BASE}/api/user-aliases?ownerUserId=${USER_ID}" >/dev/null || fail "user aliases list failed"
curl --silent --fail "${WEB_BASE}/api/device-invites?userId=${USER_ID}" >/dev/null || fail "device invites list failed"
curl --silent --fail "${WEB_BASE}/api/users/${USER_ID}/entitlement" >/dev/null || fail "user entitlement failed"
curl --silent --fail -X POST "${WEB_BASE}/api/auth/renew" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{}' >/dev/null || fail "user session renew failed"
curl --silent --fail -X PATCH "${WEB_BASE}/api/user-aliases" \
  -H 'Content-Type: application/json' \
  -d "{\"ownerUserId\":\"${USER_ID}\",\"email\":\"alias-${USER_EMAIL}\",\"alias\":\"Remote Alias\"}" >/dev/null || fail "user alias upsert failed"
curl --silent --fail -X PATCH "${WEB_BASE}/api/devices/${DEVICE_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"actorUserId\":\"${USER_ID}\",\"alias\":\"Remote UI Alias\"}" >/dev/null || fail "device alias update failed"

BOOTSTRAP="$(curl --silent --fail -X POST "${WEB_BASE}/api/web/device-bootstrap-keys" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"Remote Bootstrap\",\"ttlSeconds\":600}")" || fail "device bootstrap key create failed"
BOOTSTRAP_ID="$(printf '%s' "${BOOTSTRAP}" | json_value id)"
[[ -n "${BOOTSTRAP_ID}" ]] || fail "missing bootstrap key id"
curl --silent --fail "${WEB_BASE}/api/web/device-bootstrap-keys?userId=${USER_ID}" \
  -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null || fail "device bootstrap key list failed"
curl --silent --fail -X POST "${WEB_BASE}/api/web/device-bootstrap-keys/${BOOTSTRAP_ID}/revoke" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\"}" >/dev/null || fail "device bootstrap key revoke failed"

SECOND_USER_EMAIL="remote-ui-peer-${RUN_ID}@staticlss.com"
SECOND_AUTH="$(curl --silent --fail -X POST "${WEB_BASE}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${SECOND_USER_EMAIL}\",\"password\":\"password123\",\"name\":\"Remote UI Peer\"}")" || fail "second user register failed"
SECOND_USER_ID="$(printf '%s' "${SECOND_AUTH}" | json_value userId)"
SECOND_DEVICE_ID="remote-ui-${RUN_ID}-ios"
curl --silent --fail -X POST "${WEB_BASE}/api/devices/register" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${SECOND_USER_ID}\",\"deviceId\":\"${SECOND_DEVICE_ID}\",\"name\":\"Remote UI iOS\",\"platform\":\"ios\",\"osName\":\"iOS\",\"osVersion\":\"18.3\",\"alias\":\"Remote UI iOS\",\"publicKey\":\"remote-ui-second-public-key-${RUN_ID}\"}" >/dev/null || fail "second device register failed"
INVITE="$(curl --silent --fail -X POST "${WEB_BASE}/api/device-invites" \
  -H 'Content-Type: application/json' \
  -d "{\"networkId\":\"${NETWORK_ID}\",\"inviterUserId\":\"${USER_ID}\",\"userId\":\"${SECOND_USER_ID}\",\"ttlSeconds\":600}")" || fail "device invite create failed"
INVITE_CODE="$(printf '%s' "${INVITE}" | json_value inviteCode)"
[[ -n "${INVITE_CODE}" ]] || fail "missing invite code"
curl --silent --fail -X POST "${WEB_BASE}/api/device-invites/accept" \
  -H 'Content-Type: application/json' \
  -d "{\"inviteCode\":\"${INVITE_CODE}\",\"actorUserId\":\"${SECOND_USER_ID}\",\"deviceId\":\"${SECOND_DEVICE_ID}\",\"alias\":\"Remote UI iOS\"}" >/dev/null || fail "device invite accept failed"

WORKSPACE="$(curl --silent --fail -X POST "${WEB_BASE}/api/networks" \
  -H 'Content-Type: application/json' \
  -d "{\"ownerUserId\":\"${USER_ID}\",\"name\":\"Remote Smoke Network\",\"code\":\"rsmoke-${RUN_ID}\",\"templateKey\":\"team\"}")" || fail "network create failed"
WORKSPACE_ID="$(printf '%s' "${WORKSPACE}" | json_value networkId)"
[[ -n "${WORKSPACE_ID}" ]] || fail "missing network id"
curl --silent --fail -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Network Updated\",\"code\":\"rsmoke-${RUN_ID}\",\"status\":\"enabled\"}" >/dev/null || fail "network update failed"
curl --silent --fail -X POST "${WEB_BASE}/api/networks/${WORKSPACE_ID}/devices" \
  -H 'Content-Type: application/json' \
  -d "{\"deviceId\":\"${DEVICE_ID}\",\"actorUserId\":\"${USER_ID}\",\"alias\":\"Remote Network Device\",\"enabled\":true}" >/dev/null || fail "network device add failed"
curl --silent --fail "${WEB_BASE}/api/networks/${WORKSPACE_ID}/devices" >/dev/null || fail "network devices list failed"
curl --silent --fail -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}/devices/${DEVICE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"alias":"Remote Network Device Updated","enabled":false}' >/dev/null || fail "network device update failed"

ZONE="$(curl --silent --fail -X POST "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/zones" \
  -H 'Content-Type: application/json' \
  -d "{\"zoneName\":\"remote-${RUN_ID}.staticlss.com\",\"exposeGlobal\":true}")" || fail "dns zone create failed"
ZONE_ID="$(printf '%s' "${ZONE}" | json_value zoneId)"
[[ -n "${ZONE_ID}" ]] || fail "missing dns zone id"
curl --silent --fail -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/zones/${ZONE_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"zoneName\":\"remote-${RUN_ID}.staticlss.com\",\"exposeGlobal\":false}" >/dev/null || fail "dns zone update failed"
RECORD="$(curl --silent --fail -X POST "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/records" \
  -H 'Content-Type: application/json' \
  -d "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"app\",\"recordType\":\"A\",\"targetDeviceId\":\"${DEVICE_ID}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"\",\"ttl\":60}")" || fail "dns record create failed"
RECORD_ID="$(printf '%s' "${RECORD}" | json_value recordId)"
[[ -n "${RECORD_ID}" ]] || fail "missing dns record id"
curl --silent --fail -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/records/${RECORD_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"app2\",\"recordType\":\"A\",\"targetDeviceId\":\"${DEVICE_ID}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"\",\"ttl\":120}" >/dev/null || fail "dns record update failed"

MAPPING="$(curl --silent --fail -X POST "${WEB_BASE}/api/networks/${WORKSPACE_ID}/public-mappings" \
  -H 'Content-Type: application/json' \
  -d "{\"alias\":\"Remote Mapping\",\"publicDomain\":\"remote-${RUN_ID}.example.com\",\"sourceRecord\":\"app2.remote-${RUN_ID}.staticlss.com\",\"deviceId\":\"${DEVICE_ID}\",\"protocol\":\"tcp\",\"port\":\"443\",\"externalPort\":\"443\",\"status\":\"active\"}")" || fail "public mapping create failed"
MAPPING_ID="$(printf '%s' "${MAPPING}" | json_value mappingId)"
[[ -n "${MAPPING_ID}" ]] || fail "missing public mapping id"
curl --silent --fail -X PATCH "${WEB_BASE}/api/networks/${WORKSPACE_ID}/public-mappings/${MAPPING_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"alias\":\"Remote Mapping Updated\",\"publicDomain\":\"remote-${RUN_ID}.example.com\",\"sourceRecord\":\"app2.remote-${RUN_ID}.staticlss.com\",\"deviceId\":\"${DEVICE_ID}\",\"protocol\":\"tcp\",\"port\":\"8443\",\"externalPort\":\"443\",\"status\":\"disabled\"}" >/dev/null || fail "public mapping update failed"

GROUP="$(curl --silent --fail -X POST "${WEB_BASE}/api/networks/${WORKSPACE_ID}/security-groups" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Remote Smoke ACL","description":"remote smoke"}')" || fail "security group create failed"
GROUP_ID="$(printf '%s' "${GROUP}" | json_value securityGroupId)"
[[ -n "${GROUP_ID}" ]] || fail "missing security group id"
RULE="$(curl --silent --fail -X POST "${WEB_BASE}/api/security-groups/${GROUP_ID}/rules" \
  -H 'Content-Type: application/json' \
  -d '{"direction":"ingress","priority":100,"action":"allow","protocol":"tcp","portFrom":443,"portTo":443,"peerType":"cidr","peerValue":"10.0.0.0/8","description":"remote smoke","enabled":true}')" || fail "security rule create failed"
RULE_ID="$(printf '%s' "${RULE}" | json_value ruleId)"
[[ -n "${RULE_ID}" ]] || fail "missing security rule id"
curl --silent --fail -X PATCH "${WEB_BASE}/api/security-groups/rules/${RULE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"direction":"ingress","priority":110,"action":"allow","protocol":"tcp","portFrom":8443,"portTo":8443,"peerType":"cidr","peerValue":"10.0.0.0/8","description":"remote smoke updated","enabled":false}' >/dev/null || fail "security rule update failed"
curl --silent --fail "${WEB_BASE}/api/security-groups/${GROUP_ID}/rules" >/dev/null || fail "security rules list failed"

curl --silent --fail -X DELETE "${WEB_BASE}/api/security-groups/rules/${RULE_ID}" >/dev/null || fail "security rule delete failed"
curl --silent --fail -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/security-groups/${GROUP_ID}" >/dev/null || fail "security group delete failed"
curl --silent --fail -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/public-mappings/${MAPPING_ID}" >/dev/null || fail "public mapping delete failed"
curl --silent --fail -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/records/${RECORD_ID}" >/dev/null || fail "dns record delete failed"
curl --silent --fail -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/dns/zones/${ZONE_ID}" >/dev/null || fail "dns zone delete failed"
curl --silent --fail -X DELETE "${WEB_BASE}/api/networks/${WORKSPACE_ID}/devices/${DEVICE_ID}" >/dev/null || fail "network device remove failed"
curl --silent --fail -X PATCH "${WEB_BASE}/api/users/${USER_ID}/password" \
  -H 'Content-Type: application/json' \
  -d '{"oldPassword":"password123","newPassword":"password456"}' >/dev/null || fail "user password change failed"
curl --silent --fail -X POST "${WEB_BASE}/api/auth/logout" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{}' >/dev/null || fail "user logout failed"

echo "==> Ops UI API through ops UI proxy"
OPS_AUTH="$(curl --silent --fail -X POST "${OPS_BASE}/api/ops/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin1","password":"admin1"}')" || fail "ops login failed"
OPS_TOKEN="$(printf '%s' "${OPS_AUTH}" | json_value token)"
[[ -n "${OPS_TOKEN}" ]] || fail "missing ops token"

auth_curl "${OPS_BASE}/api/ops/dashboard" >/dev/null || fail "ops dashboard failed"
auth_curl "${OPS_BASE}/api/ops/operators" >/dev/null || fail "ops operators list failed"
auth_curl "${OPS_BASE}/api/ops/relay-nodes" >/dev/null || fail "ops relay nodes list failed"
auth_curl "${OPS_BASE}/api/ops/customers" >/dev/null || fail "ops customers list failed"
auth_curl "${OPS_BASE}/api/ops/devices" >/dev/null || fail "ops devices list failed"
auth_curl "${OPS_BASE}/api/ops/client-downloads" >/dev/null || fail "ops client downloads list failed"
auth_curl "${OPS_BASE}/api/ops/plans" >/dev/null || fail "ops plans list failed"
auth_curl "${OPS_BASE}/api/ops/products" >/dev/null || fail "ops products list failed"
auth_curl "${OPS_BASE}/api/ops/orders" >/dev/null || fail "ops orders list failed"
auth_curl "${OPS_BASE}/api/ops/renewals" >/dev/null || fail "ops renewals list failed"
auth_curl "${OPS_BASE}/api/ops/audit-events?limit=20" >/dev/null || fail "ops audit events list failed"

OPERATOR="$(auth_curl -X POST "${OPS_BASE}/api/ops/operators" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Ops\",\"email\":\"remote-ops-${RUN_ID}@staticlss.com\",\"role\":\"ops\",\"password\":\"remote-smoke-password-123\",\"status\":\"active\"}")" || fail "operator create failed"
OPERATOR_ID="$(printf '%s' "${OPERATOR}" | json_value operatorId)"
[[ -n "${OPERATOR_ID}" ]] || fail "missing operator id"
auth_curl -X PATCH "${OPS_BASE}/api/ops/operators/${OPERATOR_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Ops Updated\",\"email\":\"remote-ops-${RUN_ID}@staticlss.com\",\"role\":\"admin\",\"status\":\"active\"}" >/dev/null || fail "operator update failed"
auth_curl -X POST "${OPS_BASE}/api/ops/operators/${OPERATOR_ID}/password" \
  -H 'Content-Type: application/json' \
  -d '{"password":"remote-smoke-password-123"}' >/dev/null || fail "operator password set failed"

PLAN_CODE="remote-smoke-${RUN_ID}"
auth_curl -X POST "${OPS_BASE}/api/ops/plans" \
  -H 'Content-Type: application/json' \
  -d "{\"planCode\":\"${PLAN_CODE}\",\"name\":\"Remote Smoke Plan\",\"ownDeviceLimit\":5,\"invitedDeviceLimit\":5,\"totalDeviceLimit\":10,\"relayMonthlyGb\":100,\"relayBandwidthMbps\":50,\"relayThrottleMbps\":5,\"p2pUnlimited\":true,\"customDomain\":true,\"acl\":true,\"dedicatedRelay\":false,\"auditLog\":true,\"apiAccess\":true,\"monthlyPrice\":10,\"yearlyPrice\":100,\"status\":\"active\"}" >/dev/null || fail "plan create failed"
auth_curl -X PATCH "${OPS_BASE}/api/ops/plans/${PLAN_CODE}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Plan Updated\",\"ownDeviceLimit\":6,\"invitedDeviceLimit\":6,\"totalDeviceLimit\":12,\"relayMonthlyGb\":120,\"relayBandwidthMbps\":60,\"relayThrottleMbps\":6,\"p2pUnlimited\":true,\"customDomain\":true,\"acl\":true,\"dedicatedRelay\":false,\"auditLog\":true,\"apiAccess\":true,\"monthlyPrice\":12,\"yearlyPrice\":120,\"status\":\"active\"}" >/dev/null || fail "plan update failed"

PRODUCT="$(auth_curl -X POST "${OPS_BASE}/api/ops/products" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Product\",\"type\":\"plan\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"monthly\",\"validDays\":31,\"relayTrafficGb\":100,\"relayBandwidthMbps\":50,\"listPrice\":10,\"salePrice\":9,\"currency\":\"CNY\",\"autoRenew\":false,\"status\":\"active\",\"description\":\"remote smoke\"}")" || fail "product create failed"
PRODUCT_ID="$(printf '%s' "${PRODUCT}" | json_value productId)"
[[ -n "${PRODUCT_ID}" ]] || fail "missing product id"
auth_curl -X PATCH "${OPS_BASE}/api/ops/products/${PRODUCT_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Product Updated\",\"type\":\"plan\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"monthly\",\"validDays\":31,\"relayTrafficGb\":120,\"relayBandwidthMbps\":60,\"listPrice\":12,\"salePrice\":10,\"currency\":\"CNY\",\"autoRenew\":true,\"status\":\"active\",\"description\":\"remote smoke updated\"}" >/dev/null || fail "product update failed"

RELAY_NODE="$(auth_curl -X POST "${OPS_BASE}/api/ops/relay-nodes" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Relay\",\"region\":\"remote-${RUN_ID}\",\"transport\":\"relay_udp\",\"publicAddr\":\"udp://127.0.0.1:${RUN_ID: -4}\",\"maxBandwidthMbps\":1000,\"monthlyTrafficGb\":1024,\"maxSessions\":100,\"status\":\"active\"}")" || fail "relay node create failed"
RELAY_NODE_ID="$(printf '%s' "${RELAY_NODE}" | json_value nodeId)"
[[ -n "${RELAY_NODE_ID}" ]] || fail "missing relay node id"
auth_curl -X PATCH "${OPS_BASE}/api/ops/relay-nodes/${RELAY_NODE_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Remote Smoke Relay Updated\",\"region\":\"remote-${RUN_ID}\",\"transport\":\"relay_udp\",\"publicAddr\":\"udp://127.0.0.1:${RUN_ID: -4}\",\"maxBandwidthMbps\":900,\"monthlyTrafficGb\":2048,\"maxSessions\":120,\"status\":\"disabled\"}" >/dev/null || fail "relay node update failed"
auth_curl -X DELETE "${OPS_BASE}/api/ops/relay-nodes/${RELAY_NODE_ID}" >/dev/null || fail "relay node delete failed"

auth_curl -X PATCH "${OPS_BASE}/api/ops/customers/${USER_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${USER_EMAIL}\",\"name\":\"Remote UI Smoke Updated\",\"country\":\"CN\",\"province\":\"Guangdong\",\"city\":\"Shenzhen\",\"ipRegion\":\"South China\",\"status\":\"active\"}" >/dev/null || fail "customer update failed"
ASSIGN_RESPONSE="$(auth_curl -X POST "${OPS_BASE}/api/ops/customers/${USER_ID}/assign-plan" \
  -H 'Content-Type: application/json' \
  -d "{\"planCode\":\"${PLAN_CODE}\",\"expiresAt\":1821264000,\"amount\":100,\"period\":\"yearly\"}")" || fail "assign plan failed"
RENEWAL_ID="$(printf '%s' "${ASSIGN_RESPONSE}" | json_value renewalId)"
[[ -n "${RENEWAL_ID}" ]] || fail "missing renewal id"

auth_curl -X PATCH "${OPS_BASE}/api/ops/devices/${DEVICE_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"alias":"Remote UI Mac Updated","status":"active","enabled":true}' >/dev/null || fail "device update failed"

ORDER="$(auth_curl -X POST "${OPS_BASE}/api/ops/orders" \
  -H 'Content-Type: application/json' \
  -d "{\"customerId\":\"${USER_ID}\",\"customerEmail\":\"${USER_EMAIL}\",\"productId\":\"${PRODUCT_ID}\",\"amount\":10,\"currency\":\"CNY\",\"payStatus\":\"pending\",\"provisionStatus\":\"pending\",\"channel\":\"manual\"}")" || fail "order create failed"
ORDER_ID="$(printf '%s' "${ORDER}" | json_value orderId)"
[[ -n "${ORDER_ID}" ]] || fail "missing order id"
auth_curl -X PATCH "${OPS_BASE}/api/ops/orders/${ORDER_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"customerEmail\":\"${USER_EMAIL}\",\"productId\":\"${PRODUCT_ID}\",\"amount\":10,\"currency\":\"CNY\",\"payStatus\":\"paid\",\"provisionStatus\":\"provisioned\",\"channel\":\"manual\",\"paidAt\":1783267200,\"validUntil\":1821264000}" >/dev/null || fail "order update failed"

auth_curl -X PATCH "${OPS_BASE}/api/ops/renewals/${RENEWAL_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"customerEmail\":\"${USER_EMAIL}\",\"planCode\":\"${PLAN_CODE}\",\"period\":\"yearly\",\"amount\":100,\"currency\":\"CNY\",\"paidAt\":1783267200,\"validUntil\":1821264000,\"source\":\"manual\",\"operator\":\"admin1\"}" >/dev/null || fail "renewal update failed"

printf 'remote-smoke-client\n' >"${TMP_DIR}/SLAN-Remote-Smoke.pkg"
DOWNLOAD="$(curl --silent --fail -X POST "${OPS_BASE}/api/ops/client-downloads" \
  -H "Authorization: Bearer ${OPS_TOKEN}" \
  -F platform=macos \
  -F "version=remote-${RUN_ID}" \
  -F arch=universal \
  -F channel=stable \
  -F status=active \
  -F releaseNotes=remote-smoke \
  -F "file=@${TMP_DIR}/SLAN-Remote-Smoke.pkg")" || fail "client download upload failed"
DOWNLOAD_ID="$(printf '%s' "${DOWNLOAD}" | json_value downloadId)"
[[ -n "${DOWNLOAD_ID}" ]] || fail "missing download id"
curl --silent --fail "${WEB_BASE}/api/client-downloads" | grep -q "remote-${RUN_ID}" || fail "uploaded download not visible to web UI"
auth_curl -X DELETE "${OPS_BASE}/api/ops/client-downloads/${DOWNLOAD_ID}" >/dev/null || fail "client download delete failed"

echo "remote ui ops smoke passed"
echo "web=${WEB_BASE}"
echo "ops=${OPS_BASE}"
echo "biz=${BIZ_BASE}"
