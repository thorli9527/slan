#!/usr/bin/env bash

slan_ops_login() {
  local ops_base_url="$1"
  local email="${2:-${SLAN_OPS_EMAIL:-admin1}}"
  local password="${3:-${SLAN_OPS_PASSWORD:-admin1}}"
  curl --silent --show-error --fail \
    -X POST "${ops_base_url}/api/ops/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\"}" \
    | jq -er '.auth.session.token // .token'
}

slan_ops_create_device_credential() {
  local ops_base_url="$1"
  local ops_token="$2"
  local name="$3"
  local device_id="${4:-}"
  jq -cn --arg deviceId "$device_id" --arg name "$name" \
    '{deviceId: $deviceId, name: $name, scopes: "standard_device", expiresAt: 0}' \
    | curl --silent --show-error --fail \
      -X POST "${ops_base_url}/api/ops/device-credentials" \
      -H "Authorization: Bearer ${ops_token}" \
      -H 'Content-Type: application/json' \
      --data-binary @-
}

slan_ops_revoke_device_credential() {
  local ops_base_url="$1"
  local ops_token="$2"
  local credential_id="$3"
  [[ -n "$credential_id" ]] || return 0
  curl --silent --show-error --fail \
    -X POST "${ops_base_url}/api/ops/device-credentials/${credential_id}/revoke" \
    -H "Authorization: Bearer ${ops_token}" >/dev/null
}

slan_ops_create_network() {
  local ops_base_url="$1"
  local ops_token="$2"
  local name="$3"
  jq -cn --arg name "$name" '{name: $name, status: "active"}' \
    | curl --silent --show-error --fail \
      -X POST "${ops_base_url}/api/ops/networks" \
      -H "Authorization: Bearer ${ops_token}" \
      -H 'Content-Type: application/json' \
      --data-binary @-
}

slan_ops_add_network_device() {
  local ops_base_url="$1"
  local ops_token="$2"
  local network_id="$3"
  local device_id="$4"
  jq -cn --arg deviceId "$device_id" '{deviceId: $deviceId}' \
    | curl --silent --show-error --fail \
      -X POST "${ops_base_url}/api/ops/networks/${network_id}/devices" \
      -H "Authorization: Bearer ${ops_token}" \
      -H 'Content-Type: application/json' \
      --data-binary @- >/dev/null
}

slan_ops_delete_network() {
  local ops_base_url="$1"
  local ops_token="$2"
  local network_id="$3"
  [[ -n "$network_id" ]] || return 0
  curl --silent --show-error --fail \
    -X DELETE "${ops_base_url}/api/ops/networks/${network_id}" \
    -H "Authorization: Bearer ${ops_token}" >/dev/null
}
