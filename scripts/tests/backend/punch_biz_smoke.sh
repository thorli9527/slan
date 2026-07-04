#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

BASE_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
RUN_ID="$(date +%s%N)"
EMAIL="${SLAN_TEST_EMAIL:-punch-smoke-${RUN_ID}@example.test}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
DEVICE_A="${SLAN_PUNCH_SMOKE_DEVICE_A:-punch-smoke-mac-${RUN_ID}}"
DEVICE_B="${SLAN_PUNCH_SMOKE_DEVICE_B:-punch-smoke-android-${RUN_ID}}"
DENY_FILE="${TMPDIR:-/tmp}/slan-punch-smoke-deny-${RUN_ID}.json"
USER_ID=""
DEVICE_A_CREATED=0
DEVICE_B_CREATED=0

best_effort_curl() {
  command curl --silent --show-error --connect-timeout 5 --max-time 20 "$@" >/dev/null 2>&1 || true
}

cleanup_device() {
  local device_id="$1"
  case "${device_id}" in
    punch-smoke-*${RUN_ID}*)
      [[ -n "${USER_ID}" ]] || return 0
      best_effort_curl -X DELETE "${BASE_URL}/api/web/devices/${device_id}?actorUserId=${USER_ID}"
      ;;
  esac
}

cleanup() {
  if [[ "${DEVICE_A_CREATED}" == "1" ]]; then
    cleanup_device "${DEVICE_A}"
  fi
  if [[ "${DEVICE_B_CREATED}" == "1" ]]; then
    cleanup_device "${DEVICE_B}"
  fi
  rm -f "${DENY_FILE}"
}
trap cleanup EXIT

json_value() {
  local key="$1"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p"
}

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
    printf '%s' "${value}" | md5
    return
  fi
  if command -v md5sum >/dev/null 2>&1; then
    printf '%s' "${value}" | md5sum | awk '{print $1}'
    return
  fi
  printf '%s' "${value}" | openssl dgst -md5 -r | awk '{print $1}'
}

AUTH="$(curl_json -X POST "${BASE_URL}/api/app/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\",\"name\":\"Punch Smoke\"}")"
USER_ID="$(printf '%s' "${AUTH}" | json_value userId)"
NETWORK_ID="$(printf '%s' "${AUTH}" | json_value networkId)"
USER_TOKEN="$(printf '%s' "${AUTH}" | json_value token)"
if [[ -z "${USER_ID}" || -z "${NETWORK_ID}" || -z "${USER_TOKEN}" ]]; then
  echo "missing userId, networkId, or user token from register response" >&2
  exit 1
fi

BIND_A="$(curl_json -X POST "${BASE_URL}/api/app/device/session/bind" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -d "{\"deviceId\":\"${DEVICE_A}\",\"name\":\"Punch Mac\",\"platform\":\"macos\",\"osName\":\"macOS\",\"osVersion\":\"15.0\",\"alias\":\"Punch Mac\",\"publicKey\":\"punch-smoke-pub-a-${RUN_ID}\"}")"
DEVICE_A_CREATED=1
DEVICE_TOKEN_A="$(printf '%s' "${BIND_A}" | json_value deviceToken)"
MQTT_USERNAME_A="$(printf '%s' "${BIND_A}" | json_value username)"
MQTT_PASSWORD_A="$(printf '%s' "${BIND_A}" | json_value password)"
if [[ -z "${DEVICE_TOKEN_A}" || -z "${MQTT_USERNAME_A}" || -z "${MQTT_PASSWORD_A}" ]]; then
  echo "missing deviceToken or mqtt credential from requester bind response" >&2
  exit 1
fi
PUNCH_SIGNATURE_A="$(md5_hex "${DEVICE_A}${MQTT_PASSWORD_A}")"
curl_json -X POST "${BASE_URL}/api/app/device/session/bind" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -d "{\"userId\":\"${USER_ID}\",\"deviceId\":\"${DEVICE_B}\",\"name\":\"Punch Android\",\"platform\":\"android\",\"osName\":\"Android\",\"osVersion\":\"15\",\"alias\":\"Punch Android\",\"publicKey\":\"punch-smoke-pub-b-${RUN_ID}\"}" >/dev/null
DEVICE_B_CREATED=1

SESSION="$(curl_json -X POST "${BASE_URL}/api/app/networks/${NETWORK_ID}/punch/connect-sessions" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${DEVICE_TOKEN_A}" \
  -H "X-Slan-Device-ID: ${DEVICE_A}" \
  -H "X-Slan-MQTT-Username: ${MQTT_USERNAME_A}" \
  -H "X-Slan-Punch-Signature: ${PUNCH_SIGNATURE_A}" \
  -d "{\"requesterNodeId\":\"node-${DEVICE_A}\",\"peerNodeId\":\"node-${DEVICE_B}\",\"ttlSeconds\":30}")"
if ! printf '%s' "${SESSION}" | grep -q '"sessionId":"'; then
  echo "punch connect-session response missing sessionId: ${SESSION}" >&2
  exit 1
fi
if ! printf '%s' "${SESSION}" | grep -q "\"networkId\":\"${NETWORK_ID}\""; then
  echo "punch connect-session response has unexpected networkId: ${SESSION}" >&2
  exit 1
fi

if curl_json -X POST "${BASE_URL}/api/app/networks/${NETWORK_ID}/punch/connect-sessions" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${DEVICE_TOKEN_A}" \
  -H "X-Slan-Device-ID: ${DEVICE_A}" \
  -H "X-Slan-MQTT-Username: ${MQTT_USERNAME_A}" \
  -H "X-Slan-Punch-Signature: ${PUNCH_SIGNATURE_A}" \
  -d "{\"requesterNodeId\":\"node-${DEVICE_A}\",\"peerNodeId\":\"node-missing-${RUN_ID}\",\"ttlSeconds\":30}" >"${DENY_FILE}" 2>/dev/null; then
  echo "punch connect-session unexpectedly allowed missing peer" >&2
  exit 1
fi

echo "punch biz smoke passed base=${BASE_URL} network=${NETWORK_ID} session=$(printf '%s' "${SESSION}" | json_value sessionId)"
