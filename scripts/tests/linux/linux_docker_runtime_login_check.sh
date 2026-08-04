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
. "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"
IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_NAME:-slan-linux-runtime-check}"
WORK_DIR="${SLAN_LINUX_DOCKER_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-linux-runtime.XXXXXX")}"
LOCAL_PACKAGE_PATH="${SLAN_LINUX_CLIENT_PACKAGE:-}"
KEEP_CONTAINER="${SLAN_LINUX_DOCKER_KEEP_CONTAINER:-0}"
CONTAINER_PRIVILEGED="${SLAN_LINUX_DOCKER_CONTAINER_PRIVILEGED:-0}"
RESULT_JSON_PATH="${SLAN_LINUX_DOCKER_RESULT_JSON_PATH:-}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
CONTAINER_BIZ_URL="${SLAN_LINUX_DOCKER_CONTAINER_BIZ_URL:-$BIZ_URL}"
DEVICE_ALIAS="${SLAN_LINUX_DOCKER_DEVICE_ALIAS:-Docker Linux Runtime}"
TTL_SECONDS="${SLAN_LINUX_DOCKER_TTL_SECONDS:-1800}"
SERVICE_HOST="${SLAN_LINUX_SERVICE_HOST:-127.0.0.1:46392}"
ACTIVATION_TIMEOUT_SECONDS="${SLAN_LINUX_RUNTIME_TIMEOUT_SECONDS:-90}"
LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-1}"
BIZ_READY_TIMEOUT_SECONDS="${SLAN_LINUX_DOCKER_BIZ_READY_TIMEOUT_SECONDS:-60}"

BOOTSTRAP_ID=""
BOOTSTRAP_KEY=""
OPS_TOKEN=""

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
    if curl --silent --show-error --max-time 5 -o /dev/null "${BIZ_URL}/healthz"; then
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
  if [[ -n "$BOOTSTRAP_ID" && -n "$OPS_TOKEN" ]]; then
    slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$BOOTSTRAP_ID" >/dev/null 2>&1 || true
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
  docker_exec "echo '--- encrypted client config ---'; jq '{version,deviceId,algorithm:.encrypted.algorithm,encrypted:(.encrypted.ciphertext != null)}' /var/lib/SLAN/config.json 2>/dev/null || true" || true
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

log "create Ops device authorization key"
OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL")"
bootstrap_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$DEVICE_ALIAS")"

BOOTSTRAP_ID="$(printf '%s' "$bootstrap_json" | json_value credentialId)"
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
bash /workspace/slan/client/install/linux/install.sh \
  --server='${CONTAINER_BIZ_URL}' \
  --authorization-key='${BOOTSTRAP_KEY}' \
  --tray=disabled \
  --package-url='file://${local_package_in_container}'
"
else
  fail "SLAN_LINUX_LOCAL_PACKAGE_PATH is required"
fi

log "start client-core-service in foreground via console bootstrap"
docker_exec "pkill -x client-core-service >/dev/null 2>&1 || true"
docker exec -d "$CONTAINER_NAME" bash -lc "
set -euo pipefail
export SLAN_CLIENT_CORE_SERVICE_HOST='${SERVICE_HOST}'
export SLAN_LINUX_NETWORK_MOCK='${LINUX_NETWORK_MOCK}'
exec /usr/bin/slan-client-v2-console \
  --server-url '${CONTAINER_BIZ_URL}' \
  --authorization-key '${BOOTSTRAP_KEY}' \
  --foreground >/tmp/slan-console.out 2>/tmp/slan-console.err
" >/dev/null

log "wait for local API to become ready"
deadline=$(( $(date +%s) + ACTIVATION_TIMEOUT_SECONDS ))
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

log "wait for activated local status"
activated_json=''
while (( $(date +%s) < deadline )); do
  activated_json="$(request_json localSession || true)"
  if [[ -n "$activated_json" ]] && jq -e '.activated == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$activated_json"; then
    break
  fi
  sleep 1
done
if [[ -z "$activated_json" ]] || ! jq -e '.activated == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$activated_json"; then
  printf '%s\n' "$activated_json"
  dump_container_debug
  fail "local status did not reach activated state"
fi
device_id="$(jq -r '.deviceId // empty' <<<"$activated_json")"

log "observe client runtime state"
runtime_state_json=''
runtime_state_deadline=$(( $(date +%s) + 30 ))
while (( $(date +%s) < runtime_state_deadline )); do
  runtime_state_json="$(request_json localState || true)"
  if [[ -n "$runtime_state_json" ]] && jq -e '.activated == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$runtime_state_json"; then
    break
  fi
  sleep 1
done
if [[ -n "$runtime_state_json" ]] && jq -e '.activated == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$runtime_state_json"; then
  log "client runtime state reflected activated session"
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
if [ -f /etc/slan/client-v2-console.env ]; then
  echo "console bootstrap env preserved"
else
  echo "console bootstrap env consumed"
fi
jq -e ".deviceId != \"\" and .encrypted.algorithm == \"AES-256-GCM\" and (.encrypted.ciphertext | length) > 0" /var/lib/SLAN/config.json >/dev/null
find /opt/slan-client-v2 -maxdepth 3 -type f | sort | head -50
'

echo "linuxDockerRuntimeActivationCheck: ok credentialId=${BOOTSTRAP_ID} deviceId=${device_id} image=${IMAGE}"
if [[ -n "$RESULT_JSON_PATH" ]]; then
  mkdir -p "$(dirname "$RESULT_JSON_PATH")"
  jq -cn \
    --arg bootstrapId "$BOOTSTRAP_ID" \
    --arg deviceId "$device_id" \
    --arg image "$IMAGE" \
    --arg containerName "$CONTAINER_NAME" \
    --arg serviceHost "$SERVICE_HOST" \
    '{bootstrapId:$bootstrapId,deviceId:$deviceId,image:$image,containerName:$containerName,serviceHost:$serviceHost}' \
    >"$RESULT_JSON_PATH"
fi
