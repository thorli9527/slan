#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
FALLBACK_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../../.." && pwd)
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
if [ ! -e "$ROOT_DIR/.git" ]; then
  ROOT_DIR="$FALLBACK_ROOT"
fi
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_NAME:-slan-linux-runtime-check}"
WORK_DIR="${SLAN_LINUX_DOCKER_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-linux-runtime.XXXXXX")}"
LOCAL_PACKAGE_PATH="${SLAN_LINUX_CLIENT_PACKAGE:-}"
KEEP_CONTAINER="${SLAN_LINUX_DOCKER_KEEP_CONTAINER:-0}"
CONTAINER_PRIVILEGED="${SLAN_LINUX_DOCKER_CONTAINER_PRIVILEGED:-0}"
RESULT_JSON_PATH="${SLAN_LINUX_DOCKER_RESULT_JSON_PATH:-}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
WEB_API_BASE_URL="${SLAN_WEB_API_BASE_URL:-${SLAN_BIZ_WEB_BASE_URL:-http://47.245.40.231:28081}}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
CONTAINER_BIZ_URL="${SLAN_LINUX_DOCKER_CONTAINER_BIZ_URL:-$BIZ_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
EMAIL="${SLAN_LINUX_DOCKER_EMAIL:-linux-runtime-$(date +%s%N)@example.test}"
DEVICE_ALIAS="${SLAN_LINUX_DOCKER_DEVICE_ALIAS:-Docker Linux Runtime}"
TTL_SECONDS="${SLAN_LINUX_DOCKER_TTL_SECONDS:-1800}"
SERVICE_HOST="${SLAN_LINUX_SERVICE_HOST:-127.0.0.1:46392}"
LOGIN_TIMEOUT_SECONDS="${SLAN_LINUX_RUNTIME_TIMEOUT_SECONDS:-90}"
LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-1}"
BIZ_READY_TIMEOUT_SECONDS="${SLAN_LINUX_DOCKER_BIZ_READY_TIMEOUT_SECONDS:-60}"

BOOTSTRAP_ID=""
BOOTSTRAP_KEY=""
USER_ID=""
NETWORK_ID=""
USER_TOKEN=""

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

json_value() {
  local key="$1"
  jq -r --arg key "$key" '.[$key] // empty'
}

curl_retry() {
  local attempt
  local delay=1
  local max_attempts="${SLAN_LINUX_DOCKER_CURL_RETRY_ATTEMPTS:-8}"
  for ((attempt = 1; attempt <= max_attempts; attempt += 1)); do
    if curl "$@"; then
      return 0
    fi
    if [[ "$attempt" -lt "$max_attempts" ]]; then
      sleep "$delay"
      if [[ "$delay" -lt 8 ]]; then
        delay=$((delay * 2))
      fi
    fi
  done
  return 1
}

wait_for_biz_ready() {
  log "wait for biz api ready"
  local deadline=$((SECONDS + BIZ_READY_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    if curl --silent --show-error --max-time 5 \
      -o /dev/null \
      -X POST "${BIZ_URL}/api/app/auth/register" \
      -H 'Content-Type: application/json' \
      -d '{"email":"biz-ready-probe@example.test","password":"Password123!"}'; then
      return 0
    fi
    sleep 2
  done
  fail "biz api did not become ready within ${BIZ_READY_TIMEOUT_SECONDS}s: ${BIZ_URL}"
}

cleanup() {
  if [[ "$KEEP_CONTAINER" != "1" ]]; then
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BOOTSTRAP_ID" && -n "$USER_TOKEN" && -n "$USER_ID" ]]; then
    curl --silent --show-error --fail \
      -X POST "${WEB_API_BASE_URL}/api/web/device-bootstrap-keys/${BOOTSTRAP_ID}/revoke" \
      -H "Authorization: Bearer ${USER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"userId\":\"${USER_ID}\"}" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_CONTAINER" != "1" ]]; then
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT

dump_container_debug() {
  docker_exec "echo '--- ps ---'; ps -ef || true" || true
  docker_exec "echo '--- service log ---'; tail -n 200 /var/lib/SLAN/client-core-service.log 2>/dev/null || tail -n 200 /root/.local/share/SLAN/client-core-service.log 2>/dev/null || true" || true
  docker_exec "echo '--- console env ---'; cat /etc/slan/client-v2-console.env 2>/dev/null || true" || true
  docker_exec "echo '--- bootstrap env ---'; cat /etc/slan/bootstrap.env 2>/dev/null || true" || true
  docker_exec "echo '--- session json ---'; cat /var/lib/SLAN/client-v2-session.json 2>/dev/null || cat /root/.local/share/SLAN/client-v2-session.json 2>/dev/null || true" || true
  docker_exec "echo '--- console stdout ---'; cat /tmp/slan-console.out 2>/dev/null || true" || true
  docker_exec "echo '--- console stderr ---'; cat /tmp/slan-console.err 2>/dev/null || true" || true
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

docker_exec() {
  docker exec "$CONTAINER_NAME" bash -lc "$1"
}

request_json() {
  local method="$1"
  local args="${2:-{}}"
  docker_exec "printf '%s\n' '{\"method\":\"${method}\",\"args\":${args}}' | nc -w 3 ${SERVICE_HOST%:*} ${SERVICE_HOST##*:}"
}

need curl
need jq
need docker

wait_for_biz_ready
log "register/login Linux Docker runtime test user"
curl_retry --silent --show-error --fail \
  -X POST "${BIZ_URL}/api/app/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true

auth_json="$(curl_retry --silent --show-error --fail \
  -X POST "${BIZ_URL}/api/app/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"

USER_ID="$(printf '%s' "$auth_json" | jq -r '.userId // .auth.userId // .auth.session.userId // empty')"
USER_TOKEN="$(printf '%s' "$auth_json" | jq -r '.accessToken // .token // .auth.accessToken // .auth.session.token // empty')"
[[ -n "$USER_ID" && -n "$USER_TOKEN" ]] || fail "failed to login test user"

log "resolve default network"
networks_json="$(curl_retry --silent --show-error --fail \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  "${WEB_API_BASE_URL}/api/web/networks?userId=${USER_ID}")"
NETWORK_ID="$(printf '%s' "$networks_json" | jq -r '.items[0].networkId // .[0].networkId // empty')"
[[ -n "$NETWORK_ID" ]] || fail "failed to resolve test network"

log "create bootstrap key"
bootstrap_json="$(curl_retry --silent --show-error --fail \
  -X POST "${WEB_API_BASE_URL}/api/web/device-bootstrap-keys" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"${DEVICE_ALIAS}\",\"ttlSeconds\":${TTL_SECONDS}}")"

BOOTSTRAP_ID="$(printf '%s' "$bootstrap_json" | json_value id)"
BOOTSTRAP_KEY="$(printf '%s' "$bootstrap_json" | json_value key)"
[[ -n "$BOOTSTRAP_ID" && -n "$BOOTSTRAP_KEY" ]] || fail "failed to create bootstrap key"

log "start Linux Docker container: $IMAGE"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker_args=(
  -d
  --name "$CONTAINER_NAME"
)
if [[ "$CONTAINER_PRIVILEGED" == "1" ]]; then
  docker_args+=(--privileged)
else
  docker_args+=(--cap-add NET_ADMIN --device /dev/net/tun:/dev/net/tun)
fi
if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  [[ -f "$LOCAL_PACKAGE_PATH" ]] || fail "local package not found: $LOCAL_PACKAGE_PATH"
  docker_args+=(-v "$ROOT_DIR:/workspace/slan")
  docker run "${docker_args[@]}" "$IMAGE" sleep infinity >/dev/null
else
  docker run "${docker_args[@]}" "$IMAGE" sleep infinity >/dev/null
fi

docker_exec '
set -euo pipefail
mkdir -p /dev/net
if [ ! -e /dev/net/tun ] && [ -e /dev/tun ]; then
  ln -sf /dev/tun /dev/net/tun
fi
'

log "install runtime dependencies inside container"
docker_exec '
set -e
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update >/dev/null
  apt-get install -y bash curl ca-certificates tar jq netcat-openbsd procps iproute2 iputils-ping dnsutils python3 >/dev/null
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache bash curl ca-certificates tar jq netcat-openbsd procps iproute2 iputils bind-tools python3 >/dev/null
else
  echo "unsupported base image package manager" >&2
  exit 1
fi
'

if [[ -n "$LOCAL_PACKAGE_PATH" ]]; then
  log "install local package inside container"
  local_package_in_container="/workspace/slan/${LOCAL_PACKAGE_PATH#$ROOT_DIR/}"
  docker_exec "
set -euo pipefail
curl -fsSL '${CONTAINER_BIZ_URL}/downloads/clients/install.sh' -o /tmp/slan-install.sh
bash /tmp/slan-install.sh \
  --server='${CONTAINER_BIZ_URL}' \
  --installation-key='${BOOTSTRAP_KEY}' \
  --tray=disabled \
  --package-url='file://${local_package_in_container}'
"
else
  log "download and execute install command inside container"
  docker_exec "
set -euo pipefail
curl -fsSL '${CONTAINER_BIZ_URL}/downloads/clients/install.sh' -o /tmp/slan-install.sh
bash /tmp/slan-install.sh --server='${CONTAINER_BIZ_URL}' --installation-key='${BOOTSTRAP_KEY}' --tray=disabled
"
fi

log "start client-core-service in foreground via console bootstrap"
docker_exec "pkill -x client-core-service >/dev/null 2>&1 || true"
docker exec -d "$CONTAINER_NAME" bash -lc "
set -euo pipefail
export SLAN_CLIENT_CORE_SERVICE_HOST='${SERVICE_HOST}'
export SLAN_LINUX_NETWORK_MOCK='${LINUX_NETWORK_MOCK}'
exec /usr/bin/slan-client-v2-console \
  --server-url '${CONTAINER_BIZ_URL}' \
  --email '${EMAIL}' \
  --password '${PASSWORD}' \
  --device-name '${DEVICE_ALIAS}' \
  --foreground >/tmp/slan-console.out 2>/tmp/slan-console.err
" >/dev/null

log "wait for local API to become ready"
deadline=$(( $(date +%s) + LOGIN_TIMEOUT_SECONDS ))
while (( $(date +%s) < deadline )); do
  if request_json localStatus >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! request_json localStatus >/dev/null 2>&1; then
  dump_container_debug
  fail "client-core-service local API did not become ready"
fi

log "wait for signed-in local status"
signed_in_json=''
while (( $(date +%s) < deadline )); do
  signed_in_json="$(request_json localSession || true)"
  if [[ -n "$signed_in_json" ]] && jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$signed_in_json"; then
    break
  fi
  sleep 1
done
if [[ -z "$signed_in_json" ]] || ! jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$signed_in_json"; then
  printf '%s\n' "$signed_in_json"
  dump_container_debug
  fail "local status did not reach signed-in state"
fi
device_id="$(jq -r '.deviceId // empty' <<<"$signed_in_json")"

log "observe client runtime state"
runtime_state_json=''
runtime_state_deadline=$(( $(date +%s) + 30 ))
while (( $(date +%s) < runtime_state_deadline )); do
  runtime_state_json="$(request_json localState || true)"
  if [[ -n "$runtime_state_json" ]] && jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$runtime_state_json"; then
    break
  fi
  sleep 1
done
if [[ -n "$runtime_state_json" ]] && jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$runtime_state_json"; then
  log "client runtime state reflected signed-in session"
else
  log "client runtime state still converging; continue with persisted session + control-ready checks"
fi

log "wait for control transport ready"
control_json=''
while (( $(date +%s) < deadline )); do
  control_json="$(request_json localControlStatus || true)"
  if [[ -n "$control_json" ]] && jq -e '.ready == true' >/dev/null <<<"$control_json"; then
    break
  fi
  sleep 1
done
if [[ -z "$control_json" ]] || ! jq -e '.ready == true' >/dev/null <<<"$control_json"; then
  printf '%s\n' "$control_json"
  dump_container_debug
  fail "local control transport did not become ready"
fi

log "inspect runtime files"
docker_exec '
set -euo pipefail
test -x /opt/slan-client-v2/bin/client-core-service
test -f /etc/slan/bootstrap.env
if [ -f /etc/slan/client-v2-console.env ]; then
  echo "console bootstrap env preserved"
else
  echo "console bootstrap env consumed"
fi
test -f /var/lib/SLAN/client-v2-session.json
find /opt/slan-client-v2 -maxdepth 3 -type f | sort | head -50
'

echo "linuxDockerRuntimeLoginCheck: ok email=${EMAIL} networkId=${NETWORK_ID} bootstrapId=${BOOTSTRAP_ID} deviceId=${device_id} image=${IMAGE}"
if [[ -n "$RESULT_JSON_PATH" ]]; then
  mkdir -p "$(dirname "$RESULT_JSON_PATH")"
  jq -cn \
    --arg email "$EMAIL" \
    --arg networkId "$NETWORK_ID" \
    --arg bootstrapId "$BOOTSTRAP_ID" \
    --arg deviceId "$device_id" \
    --arg image "$IMAGE" \
    --arg containerName "$CONTAINER_NAME" \
    --arg serviceHost "$SERVICE_HOST" \
    --arg userId "$USER_ID" \
    --arg userToken "$USER_TOKEN" \
    '{email:$email,networkId:$networkId,bootstrapId:$bootstrapId,deviceId:$deviceId,image:$image,containerName:$containerName,serviceHost:$serviceHost,userId:$userId,userToken:$userToken}' \
    >"$RESULT_JSON_PATH"
fi
