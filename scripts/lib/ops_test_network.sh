#!/usr/bin/env bash

slan_ops_login() {
  local ops_base_url="$1"
  local email="${SLAN_OPS_EMAIL:-admin1}"
  local password="${SLAN_OPS_PASSWORD:-admin1}"
  local response
  response="$(curl --silent --show-error --fail \
    -X POST "${ops_base_url%/}/api/ops/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\"}")" || return
  SLAN_OPS_TEST_TOKEN="$(printf '%s' "$response" | jq -er '.token')"
  export SLAN_OPS_TEST_TOKEN
}

slan_ops_provision_test_user() {
  local ops_base_url="$1"
  local app_base_url="$2"
  local email="$3"
  local password="$4"
  local name="${5:-Integration Test User}"
  local response_file status

  slan_ops_login "$ops_base_url" || return
  response_file="$(mktemp)" || return
  status="$(curl --silent --show-error \
    --output "$response_file" \
    --write-out '%{http_code}' \
    -X POST "${ops_base_url%/}/api/ops/users" \
    -H "Authorization: Bearer ${SLAN_OPS_TEST_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\",\"name\":\"${name}\"}")" || {
      rm -f "$response_file"
      return 1
    }
  if [[ "$status" != "201" && "$status" != "409" ]]; then
    cat "$response_file" >&2
    rm -f "$response_file"
    return 1
  fi
  rm -f "$response_file"

  curl --silent --show-error --fail \
    -X POST "${app_base_url%/}/api/app/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\",\"sessionMode\":\"long\"}"
}

slan_ops_create_test_network() {
  local ops_base_url="$1"
  local owner_id="$2"
  local name="$3"
  local response network_id security_group
  slan_ops_login "$ops_base_url" || return
  response="$(curl --silent --show-error --fail \
    -X POST "${ops_base_url%/}/api/ops/networks" \
    -H "Authorization: Bearer ${SLAN_OPS_TEST_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"ownerId\":\"${owner_id}\",\"name\":\"${name}\",\"cidr\":\"10.0.0.0/8\",\"intraGroupPolicy\":\"allow\"}")" || return
  network_id="$(printf '%s' "$response" | jq -er '.network.networkId // .networkId')" || return
  security_group="$(curl --silent --show-error --fail \
    -X POST "${ops_base_url%/}/api/ops/networks/${network_id}/security-groups" \
    -H "Authorization: Bearer ${SLAN_OPS_TEST_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"ownerId\":\"${owner_id}\",\"name\":\"test-default\",\"description\":\"integration test security group\"}")" || return
  SLAN_OPS_TEST_SECURITY_GROUP_ID="$(printf '%s' "$security_group" | jq -er '.securityGroupId')" || return
  SLAN_OPS_TEST_NETWORK_ID="$network_id"
  export SLAN_OPS_TEST_NETWORK_ID
  export SLAN_OPS_TEST_SECURITY_GROUP_ID
}

slan_ops_delete_test_network() {
  local ops_base_url="$1"
  local owner_id="$2"
  local network_id="$3"
  [[ -n "$owner_id" && -n "$network_id" ]] || return 0
  slan_ops_login "$ops_base_url" >/dev/null 2>&1 || return 0
  curl --silent --show-error \
    -X DELETE "${ops_base_url%/}/api/ops/networks/${network_id}" \
    -H "Authorization: Bearer ${SLAN_OPS_TEST_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"ownerId\":\"${owner_id}\"}" >/dev/null 2>&1 || true
}
