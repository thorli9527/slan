#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ ! -e "$ROOT_DIR/scripts/lib/client_default_endpoints.sh" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
if [ ! -e "$ROOT_DIR/scripts/lib/client_default_endpoints.sh" ]; then
  echo "unable to locate repository root from $SCRIPT_DIR" >&2
  exit 1
fi
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"

BASE_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}}"
RUN_ID="$(date +%s%N)"
DEVICE_A="${SLAN_PUNCH_SMOKE_DEVICE_A:-punch-smoke-mac-${RUN_ID}}"
DEVICE_B="${SLAN_PUNCH_SMOKE_DEVICE_B:-punch-smoke-android-${RUN_ID}}"
DENY_FILE="${TMPDIR:-/tmp}/slan-punch-smoke-deny-${RUN_ID}.json"
OPS_TOKEN=""
NETWORK_ID=""
CREDENTIAL_ID_A=""
CREDENTIAL_ID_B=""

best_effort_curl() {
  command curl --silent --show-error --connect-timeout 5 --max-time 20 "$@" >/dev/null 2>&1 || true
}

cleanup() {
  if [[ -n "$OPS_TOKEN" ]]; then
    slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" 2>/dev/null || true
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$CREDENTIAL_ID_A" 2>/dev/null || true
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$CREDENTIAL_ID_B" 2>/dev/null || true
    [[ -n "$DEVICE_A" ]] && best_effort_curl -X DELETE "$OPS_BASE_URL/api/ops/devices/$DEVICE_A" -H "Authorization: Bearer $OPS_TOKEN"
    [[ -n "$DEVICE_B" ]] && best_effort_curl -X DELETE "$OPS_BASE_URL/api/ops/devices/$DEVICE_B" -H "Authorization: Bearer $OPS_TOKEN"
  fi
  rm -f "$DENY_FILE"
}
trap cleanup EXIT

curl_json() {
  local attempt
  for attempt in 1 2 3 4 5 6 7 8; do
    if curl --silent --show-error --fail --connect-timeout 5 --max-time 20 "$@"; then
      return 0
    fi
    sleep 2
  done
  curl --silent --show-error --fail --connect-timeout 5 --max-time 20 "$@"
}

md5_hex() {
  local value="$1"
  if command -v md5 >/dev/null 2>&1; then
    printf '%s' "$value" | md5
    return
  fi
  if command -v md5sum >/dev/null 2>&1; then
    printf '%s' "$value" | md5sum | awk '{print $1}'
    return
  fi
  printf '%s' "$value" | openssl dgst -md5 -r | awk '{print $1}'
}

command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }
OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"

SOURCE_DEVICE_A="$(curl_json -X POST "$OPS_BASE_URL/api/ops/devices" \
  -H "Authorization: Bearer $OPS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Punch Smoke Mac $RUN_ID\",\"alias\":\"Punch Smoke Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"smoke\",\"publicKey\":\"punch-smoke-mac-key-$RUN_ID\"}")"
DEVICE_A="$(printf '%s' "$SOURCE_DEVICE_A" | jq -er '.deviceId')"
SOURCE_DEVICE_B="$(curl_json -X POST "$OPS_BASE_URL/api/ops/devices" \
  -H "Authorization: Bearer $OPS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Punch Smoke Android $RUN_ID\",\"alias\":\"Punch Smoke Android\",\"platform\":\"android\",\"osName\":\"Android\",\"osVersion\":\"smoke\",\"publicKey\":\"punch-smoke-android-key-$RUN_ID\"}")"
DEVICE_B="$(printf '%s' "$SOURCE_DEVICE_B" | jq -er '.deviceId')"
CREDENTIAL_A="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Punch Smoke Mac" "$DEVICE_A")"
CREDENTIAL_ID_A="$(printf '%s' "$CREDENTIAL_A" | jq -er '.credentialId')"
AUTHORIZATION_KEY_A="$(printf '%s' "$CREDENTIAL_A" | jq -er '.key')"
CREDENTIAL_B="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Punch Smoke Android" "$DEVICE_B")"
CREDENTIAL_ID_B="$(printf '%s' "$CREDENTIAL_B" | jq -er '.credentialId')"
AUTHORIZATION_KEY_B="$(printf '%s' "$CREDENTIAL_B" | jq -er '.key')"

SESSION_A="$(curl_json -X POST "$BASE_URL/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"$AUTHORIZATION_KEY_A\",\"deviceId\":\"$DEVICE_A\"}")"
DEVICE_TOKEN_A="$(printf '%s' "$SESSION_A" | jq -er '.deviceSession.deviceToken')"
MQTT_USERNAME_A="$(printf '%s' "$SESSION_A" | jq -er '.mqtt.username')"
MQTT_PASSWORD_A="$(printf '%s' "$SESSION_A" | jq -er '.mqtt.password')"
curl_json -X POST "$BASE_URL/api/device-auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"$AUTHORIZATION_KEY_B\",\"deviceId\":\"$DEVICE_B\"}" >/dev/null

NETWORK="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "Punch Smoke $RUN_ID")"
NETWORK_ID="$(printf '%s' "$NETWORK" | jq -er '.networkId')"
slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$DEVICE_A"
slan_ops_add_network_device "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" "$DEVICE_B"

PUNCH_SIGNATURE_A="$(md5_hex "${DEVICE_A}${MQTT_PASSWORD_A}")"
SESSION="$(curl_json -X POST "$BASE_URL/api/app/networks/$NETWORK_ID/punch/connect-sessions" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $DEVICE_TOKEN_A" \
  -H "X-Slan-Device-ID: $DEVICE_A" \
  -H "X-Slan-MQTT-Username: $MQTT_USERNAME_A" \
  -H "X-Slan-Punch-Signature: $PUNCH_SIGNATURE_A" \
  -d "{\"requesterNodeId\":\"node-$DEVICE_A\",\"peerNodeId\":\"node-$DEVICE_B\",\"ttlSeconds\":30}")"
printf '%s' "$SESSION" | jq -e --arg networkId "$NETWORK_ID" \
  '.sessionId | type == "string" and length > 0' >/dev/null || {
    echo "punch connect-session response missing sessionId: $SESSION" >&2
    exit 1
  }
printf '%s' "$SESSION" | jq -e --arg networkId "$NETWORK_ID" '.networkId == $networkId' >/dev/null || {
  echo "punch connect-session response has unexpected networkId: $SESSION" >&2
  exit 1
}

if curl_json -X POST "$BASE_URL/api/app/networks/$NETWORK_ID/punch/connect-sessions" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $DEVICE_TOKEN_A" \
  -H "X-Slan-Device-ID: $DEVICE_A" \
  -H "X-Slan-MQTT-Username: $MQTT_USERNAME_A" \
  -H "X-Slan-Punch-Signature: $PUNCH_SIGNATURE_A" \
  -d "{\"requesterNodeId\":\"node-$DEVICE_A\",\"peerNodeId\":\"node-missing-$RUN_ID\",\"ttlSeconds\":30}" >"$DENY_FILE" 2>/dev/null; then
  echo "punch connect-session unexpectedly allowed missing peer" >&2
  exit 1
fi

echo "punch biz smoke passed base=$BASE_URL network=$NETWORK_ID session=$(printf '%s' "$SESSION" | jq -r '.sessionId')"
