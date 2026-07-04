#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_NAME:-slan-linux-bootstrap-check}"
WORK_DIR="${SLAN_LINUX_DOCKER_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-linux-docker.XXXXXX")}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
EMAIL="${SLAN_LINUX_DOCKER_EMAIL:-linux-docker-$(date +%s%N)@example.test}"
DEVICE_ALIAS="${SLAN_LINUX_DOCKER_DEVICE_ALIAS:-Docker Linux Bootstrap}"
TTL_SECONDS="${SLAN_LINUX_DOCKER_TTL_SECONDS:-1800}"

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

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  if [[ -n "$BOOTSTRAP_ID" && -n "$USER_TOKEN" && -n "$USER_ID" ]]; then
    curl --silent --show-error --fail \
      -X POST "${WEB_BASE_URL}/api/web/device-bootstrap-keys/${BOOTSTRAP_ID}/revoke" \
      -H "Authorization: Bearer ${USER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"userId\":\"${USER_ID}\"}" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

need curl
need jq
need docker

log "register/login Linux Docker test user"
curl --silent --show-error --fail \
  -X POST "${BIZ_URL}/api/app/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true

auth_json="$(curl --silent --show-error --fail \
  -X POST "${BIZ_URL}/api/app/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"

USER_ID="$(printf '%s' "$auth_json" | jq -r '.userId // .auth.userId // .auth.session.userId // empty')"
USER_TOKEN="$(printf '%s' "$auth_json" | jq -r '.accessToken // .token // .auth.accessToken // .auth.session.token // empty')"
[[ -n "$USER_ID" && -n "$USER_TOKEN" ]] || fail "failed to login test user"

log "resolve default network"
networks_json="$(curl --silent --show-error --fail \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  "${WEB_BASE_URL}/api/web/networks?userId=${USER_ID}")"
NETWORK_ID="$(printf '%s' "$networks_json" | jq -r '.items[0].networkId // .[0].networkId // empty')"
[[ -n "$NETWORK_ID" ]] || fail "failed to resolve test network"

log "create bootstrap key"
bootstrap_json="$(curl --silent --show-error --fail \
  -X POST "${WEB_BASE_URL}/api/web/device-bootstrap-keys" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"${DEVICE_ALIAS}\",\"ttlSeconds\":${TTL_SECONDS}}")"

BOOTSTRAP_ID="$(printf '%s' "$bootstrap_json" | json_value id)"
BOOTSTRAP_KEY="$(printf '%s' "$bootstrap_json" | json_value key)"
[[ -n "$BOOTSTRAP_ID" && -n "$BOOTSTRAP_KEY" ]] || fail "failed to create bootstrap key"

log "bootstrap id: $BOOTSTRAP_ID"
log "start Linux Docker container: $IMAGE"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER_NAME" "$IMAGE" sleep infinity >/dev/null

log "install curl/tar inside container when needed"
docker exec "$CONTAINER_NAME" bash -lc '
set -e
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y curl ca-certificates tar
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache curl ca-certificates tar bash
else
  echo "unsupported base image package manager" >&2
  exit 1
fi
'

log "download and execute install command inside container"
docker exec "$CONTAINER_NAME" bash -lc "
set -euo pipefail
curl -fsSL '${BIZ_URL}/downloads/clients/install.sh' -o /tmp/slan-install.sh
bash /tmp/slan-install.sh --server='${BIZ_URL}' --session-key='${BOOTSTRAP_KEY}' --tray=disabled
"

log "verify bootstrap config written"
docker exec "$CONTAINER_NAME" bash -lc "
set -euo pipefail
test -f /etc/slan/bootstrap.env
grep -q '^SLAN_CONTROL_BASE_URL=${BIZ_URL}\$' /etc/slan/bootstrap.env
grep -q '^SLAN_SESSION_KEY=${BOOTSTRAP_KEY}\$' /etc/slan/bootstrap.env
test -f /etc/slan/client-v2-install.env
grep -q '^SLAN_LINUX_TRAY_MODE=disabled\$' /etc/slan/client-v2-install.env
"

log "inspect extracted client payload"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
find /opt/slan-client-v2 -maxdepth 3 -type f 2>/dev/null | sort | head -50
'

log "linux docker bootstrap install check finished"
echo "linuxDockerBootstrapInstallCheck: ok email=${EMAIL} networkId=${NETWORK_ID} bootstrapId=${BOOTSTRAP_ID} image=${IMAGE}"
