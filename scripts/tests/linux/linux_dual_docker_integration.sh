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
IMAGE="${SLAN_LINUX_DUAL_IMAGE:-ubuntu:24.04}"
BUILD_PACKAGE="${SLAN_LINUX_DUAL_BUILD_PACKAGE:-0}"
CONTAINER_PREFIX="${SLAN_LINUX_DUAL_PREFIX:-slan-linux-dual}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
WEB_API_BASE_URL="${SLAN_WEB_API_BASE_URL:-${SLAN_BIZ_WEB_BASE_URL:-http://47.245.40.231:28081}}"
WEB_API_PREFIX="${SLAN_WEB_API_PREFIX:-/api/web}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
CONTAINER_BIZ_URL="${SLAN_LINUX_DUAL_CONTAINER_BIZ_URL:-$BIZ_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
EMAIL="${SLAN_LINUX_DUAL_EMAIL:-linux-dual-$(date +%s%N)@example.test}"
TIMEOUT_SECONDS="${SLAN_LINUX_DUAL_TIMEOUT_SECONDS:-120}"
BOOTSTRAP_TTL_SECONDS="${SLAN_LINUX_DUAL_BOOTSTRAP_TTL_SECONDS:-1800}"
ENABLE_NETWORK="${SLAN_LINUX_DUAL_ENABLE_NETWORK:-0}"
LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-1}"
UDP_PORT="${SLAN_LINUX_DUAL_UDP_PORT:-19090}"
TCP_PORT="${SLAN_LINUX_DUAL_TCP_PORT:-19091}"
RUN_PACKET_TESTS="${SLAN_LINUX_DUAL_PACKET_TESTS:-0}"
KEEP_CONTAINERS="${SLAN_LINUX_DUAL_KEEP_CONTAINERS:-0}"
KEEP_REMOTE_STATE="${SLAN_LINUX_DUAL_KEEP_REMOTE_STATE:-0}"
CONTAINER_PRIVILEGED="${SLAN_LINUX_DUAL_CONTAINER_PRIVILEGED:-0}"
CONTAINER_A="${CONTAINER_PREFIX}-a"
CONTAINER_B="${CONTAINER_PREFIX}-b"
SERVICE_HOST_A="127.0.0.1:46392"
SERVICE_HOST_B="127.0.0.1:46393"
RESULT_DIR="${SLAN_LINUX_DUAL_RESULT_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-linux-dual-result.XXXXXX")}"
RESULT_JSON_A="$RESULT_DIR/a.json"
RESULT_JSON_B="$RESULT_DIR/b.json"
LOCAL_API_TIMEOUT_SECONDS="${SLAN_LINUX_DUAL_LOCAL_API_TIMEOUT_SECONDS:-8}"
BIZ_READY_TIMEOUT_SECONDS="${SLAN_LINUX_DUAL_BIZ_READY_TIMEOUT_SECONDS:-60}"

BOOTSTRAP_ID_A=""
BOOTSTRAP_KEY_A=""
BOOTSTRAP_ID_B=""
BOOTSTRAP_KEY_B=""
USER_ID=""
USER_TOKEN=""
NETWORK_ID=""
ZONE_ID=""
ZONE_NAME=""
RULE_IDS=()
RECORD_IDS=()

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

resolve_linux_package_path() {
  if [[ -n "${SLAN_LINUX_CLIENT_PACKAGE:-}" ]]; then
    printf '%s\n' "$SLAN_LINUX_CLIENT_PACKAGE"
    return
  fi

  local installer_dir="$ROOT_DIR/client_v2/.tmp/installer/linux"
  local host_arch
  host_arch="$(uname -m 2>/dev/null || true)"
  local preferred=()
  case "$host_arch" in
    x86_64|amd64)
      preferred+=(
        "$installer_dir/SLAN-Client-V2-linux-amd64.tar.gz"
        "$installer_dir/SLAN-Client-V2-linux-arm64.tar.gz"
      )
      ;;
    arm64|aarch64)
      preferred+=(
        "$installer_dir/SLAN-Client-V2-linux-arm64.tar.gz"
        "$installer_dir/SLAN-Client-V2-linux-amd64.tar.gz"
      )
      ;;
    *)
      preferred+=(
        "$installer_dir/SLAN-Client-V2-linux-amd64.tar.gz"
        "$installer_dir/SLAN-Client-V2-linux-arm64.tar.gz"
      )
      ;;
  esac

  local candidate
  for candidate in "${preferred[@]}"; do
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done

  printf '%s\n' "${preferred[0]}"
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

log "linux dual docker defaults: account=${EMAIL} biz=${BIZ_URL} containerBiz=${CONTAINER_BIZ_URL} web=${WEB_BASE_URL} webApi=${WEB_API_BASE_URL}"

if [[ "$RUN_PACKET_TESTS" == "1" && "$LINUX_NETWORK_MOCK" == "1" ]]; then
  fail "packet tests require SLAN_LINUX_NETWORK_MOCK=0 so the client uses a real TUN data plane"
fi

if [[ "$RUN_PACKET_TESTS" == "1" && ! -e /dev/net/tun && ! -e /dev/tun ]]; then
  fail "packet tests require a Linux Docker host with /dev/net/tun; Docker Desktop on macOS cannot provide a real TUN data plane here"
fi

json_value() {
  local key="$1"
  jq -r --arg key "$key" '.[$key] // empty'
}

curl_retry() {
  local attempt
  local delay=1
  local max_attempts="${SLAN_LINUX_DUAL_CURL_RETRY_ATTEMPTS:-8}"
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

cleanup_container() {
  local name="$1"
  docker rm -f "$name" >/dev/null 2>&1 || true
}

container_exec() {
  local name="$1"
  shift
  docker exec "$name" bash -lc "$*"
}

request_json() {
  local name="$1"
  local service_host="$2"
  local method="$3"
  local args="${4:-{}}"
  local payload
  payload="$(printf '{"method":"%s","args":%s}\n' "$method" "$args")"
  printf '%s' "$payload" | docker exec -i "$name" python3 -c '
import socket
import sys

try:
    host = sys.argv[1]
    port = int(sys.argv[2])
    timeout = float(sys.argv[3])
    payload = sys.stdin.buffer.read()
    if not payload.endswith(b"\n"):
        payload += b"\n"

    sock = socket.create_connection((host, port), timeout=timeout)
    sock.settimeout(timeout)
    try:
        sock.sendall(payload)
        sock.shutdown(socket.SHUT_WR)
        chunks = []
        while True:
            chunk = sock.recv(65536)
            if not chunk:
                break
            chunks.append(chunk)
            if b"\n" in chunk:
                break
        sys.stdout.buffer.write(b"".join(chunks))
    finally:
        sock.close()
except Exception:
    sys.exit(1)
' "${service_host%:*}" "${service_host##*:}" "${LOCAL_API_TIMEOUT_SECONDS}"
}

revoke_bootstrap_key() {
  local bootstrap_id="$1"
  [[ -n "$bootstrap_id" && -n "$USER_TOKEN" && -n "$USER_ID" ]] || return 0
  curl --silent --show-error --fail \
    -X POST "${WEB_API_BASE_URL}${WEB_API_PREFIX}/device-bootstrap-keys/${bootstrap_id}/revoke" \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"${USER_ID}\"}" >/dev/null 2>&1 || true
}

cleanup() {
  if [[ "$KEEP_CONTAINERS" != "1" ]]; then
    cleanup_container "$CONTAINER_A"
    cleanup_container "$CONTAINER_B"
  fi
  revoke_bootstrap_key "$BOOTSTRAP_ID_A"
  revoke_bootstrap_key "$BOOTSTRAP_ID_B"
  if [[ "$KEEP_REMOTE_STATE" == "1" ]]; then
    rm -rf "$RESULT_DIR"
    return
  fi
  if [[ -n "$USER_ID" ]]; then
    for rule_id in "${RULE_IDS[@]:-}"; do
      curl --silent --show-error -X DELETE \
        "${WEB_API_BASE_URL}${WEB_API_PREFIX}/security-groups/rules/${rule_id}" >/dev/null 2>&1 || true
    done
    if [[ -n "$NETWORK_ID" ]]; then
      for record_id in "${RECORD_IDS[@]:-}"; do
        curl --silent --show-error -X DELETE \
          "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks/${NETWORK_ID}/dns/records/${record_id}" >/dev/null 2>&1 || true
      done
      if [[ -n "$ZONE_ID" ]]; then
        curl --silent --show-error -X DELETE \
          "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks/${NETWORK_ID}/dns/zones/${ZONE_ID}" >/dev/null 2>&1 || true
      fi
    fi
    local device_ids
    device_ids="$(curl --silent --show-error \
      "${WEB_API_BASE_URL}${WEB_API_PREFIX}/devices?userId=${USER_ID}" | jq -r '.items[]?.deviceId // empty' 2>/dev/null || true)"
    while IFS= read -r device_id; do
      [[ -n "$device_id" ]] || continue
      curl --silent --show-error -X DELETE \
        "${WEB_API_BASE_URL}${WEB_API_PREFIX}/devices/${device_id}?actorUserId=${USER_ID}" >/dev/null 2>&1 || true
    done <<<"$device_ids"
  fi
  rm -rf "$RESULT_DIR"
}
trap cleanup EXIT

PACKAGE_PATH="$(resolve_linux_package_path)"

dump_container_debug() {
  local name="$1"
  log "debug dump for $name"
  container_exec "$name" "echo '--- ps ---'; ps -ef || true" || true
  container_exec "$name" "echo '--- console stdout ---'; tail -n 200 /tmp/slan-console.out 2>/dev/null || true" || true
  container_exec "$name" "echo '--- console stderr ---'; tail -n 200 /tmp/slan-console.err 2>/dev/null || true" || true
  container_exec "$name" "echo '--- service log ---'; tail -n 200 /var/lib/SLAN/client-core-service.log 2>/dev/null || tail -n 200 /root/.local/share/SLAN/client-core-service.log 2>/dev/null || true" || true
  container_exec "$name" "echo '--- encrypted client config ---'; jq '{version,deviceId,algorithm:.encrypted.algorithm,encrypted:(.encrypted.ciphertext != null)}' /var/lib/SLAN/config.json 2>/dev/null || true" || true
}

register_and_login() {
  wait_for_biz_ready
  log "register/login test user"
  curl_retry --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true

  local auth_json
  auth_json="$(curl_retry --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
  USER_ID="$(printf '%s' "$auth_json" | jq -r '.userId // .auth.userId // .auth.session.userId // .auth.user.userId // empty')"
  USER_TOKEN="$(printf '%s' "$auth_json" | jq -r '.accessToken // .token // .auth.accessToken // .auth.session.token // empty')"
  [[ -n "$USER_ID" && -n "$USER_TOKEN" ]] || fail "failed to login test user"
}

resolve_network() {
  log "resolve default network"
  local networks_json
  networks_json="$(curl_retry --silent --show-error --fail \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks?userId=${USER_ID}")"
  NETWORK_ID="$(printf '%s' "$networks_json" | jq -r '.items[0].networkId // .[0].networkId // empty')"
  [[ -n "$NETWORK_ID" ]] || fail "failed to resolve default network"
}

create_bootstrap_key() {
  local alias="$1"
  local bootstrap_json
  bootstrap_json="$(curl_retry --silent --show-error --fail \
    -X POST "${WEB_API_BASE_URL}${WEB_API_PREFIX}/device-bootstrap-keys" \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"${alias}\",\"ttlSeconds\":${BOOTSTRAP_TTL_SECONDS}}")"
  local bootstrap_id bootstrap_key
  bootstrap_id="$(printf '%s' "$bootstrap_json" | json_value id)"
  bootstrap_key="$(printf '%s' "$bootstrap_json" | json_value key)"
  [[ -n "$bootstrap_id" && -n "$bootstrap_key" ]] || fail "failed to create bootstrap key for ${alias}"
  printf '%s;%s\n' "$bootstrap_id" "$bootstrap_key"
}

ensure_package() {
  if [[ "$BUILD_PACKAGE" == "1" || ! -f "$PACKAGE_PATH" ]]; then
    log "build Linux package in docker"
    bash "$ROOT_DIR/scripts/build_linux_client_docker.sh"
  fi
  [[ -f "$PACKAGE_PATH" ]] || fail "Linux client package not found: $PACKAGE_PATH"
}

start_container() {
  local name="$1"
  cleanup_container "$name"
  local docker_args=(
    -d
    --name "$name"
    -v "$ROOT_DIR:/workspace/slan"
  )
  if [[ "$CONTAINER_PRIVILEGED" == "1" ]]; then
    docker_args+=(--privileged)
  else
    docker_args+=(--cap-add NET_ADMIN --device /dev/net/tun:/dev/net/tun)
  fi
  docker run "${docker_args[@]}" "$IMAGE" sleep infinity >/dev/null
  container_exec "$name" '
set -euo pipefail
mkdir -p /dev/net
if [ ! -e /dev/net/tun ] && [ -e /dev/tun ]; then
  ln -sf /dev/tun /dev/net/tun
fi
'
}

install_container_dependencies() {
  local name="$1"
  container_exec "$name" '
set -euo pipefail
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
}

install_client() {
  local name="$1"
  local bootstrap_key="$2"
  local local_package_in_container
  local_package_in_container="/workspace/slan/${PACKAGE_PATH#$ROOT_DIR/}"
  container_exec "$name" "
set -euo pipefail
curl -fsSL '${CONTAINER_BIZ_URL}/downloads/clients/install.sh' -o /tmp/slan-install.sh
bash /tmp/slan-install.sh \
  --server='${CONTAINER_BIZ_URL}' \
  --installation-key='${bootstrap_key}' \
  --tray=disabled \
  --package-url='file://${local_package_in_container}'
"
}

provision_logged_in_container() {
  local container_name="$1"
  local service_host="$2"
  local device_alias="$3"
  local result_json_path="$4"
  SLAN_LINUX_CLIENT_PACKAGE="$PACKAGE_PATH" \
  SLAN_LINUX_DOCKER_IMAGE="$IMAGE" \
  SLAN_LINUX_RUNTIME_TIMEOUT_SECONDS="$TIMEOUT_SECONDS" \
  SLAN_LINUX_DOCKER_NAME="$container_name" \
  SLAN_LINUX_SERVICE_HOST="$service_host" \
  SLAN_LINUX_DOCKER_CONTAINER_BIZ_URL="$CONTAINER_BIZ_URL" \
  SLAN_LINUX_DOCKER_EMAIL="$EMAIL" \
  SLAN_LINUX_DOCKER_DEVICE_ALIAS="$device_alias" \
  SLAN_LINUX_DOCKER_TTL_SECONDS="$BOOTSTRAP_TTL_SECONDS" \
  SLAN_LINUX_DOCKER_CONTAINER_PRIVILEGED="$CONTAINER_PRIVILEGED" \
  SLAN_LINUX_DOCKER_KEEP_CONTAINER=1 \
  SLAN_LINUX_DOCKER_RESULT_JSON_PATH="$result_json_path" \
  SLAN_LINUX_NETWORK_MOCK="$LINUX_NETWORK_MOCK" \
  bash "$ROOT_DIR/scripts/linux_docker_runtime_login_check.sh"
}

start_console() {
  local name="$1"
  local service_host="$2"
  local device_alias="$3"
  docker exec -d "$name" bash -lc "
set -euo pipefail
export SLAN_CLIENT_CORE_SERVICE_HOST='${service_host}'
export SLAN_LINUX_NETWORK_MOCK='${LINUX_NETWORK_MOCK}'
exec /usr/bin/slan-client-v2-console \
  --server-url '${CONTAINER_BIZ_URL}' \
  --email '${EMAIL}' \
  --password '${PASSWORD}' \
  --device-name '${device_alias}' \
  --foreground >/tmp/slan-console.out 2>/tmp/slan-console.err
" >/dev/null
}

wait_local_api() {
  local name="$1"
  local service_host="$2"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  while (( $(date +%s) < deadline )); do
    if request_json "$name" "$service_host" localStatus >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  dump_container_debug "$name"
  fail "${name} local API did not become ready"
}

wait_signed_in() {
  local name="$1"
  local service_host="$2"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local status_json=''
  while (( $(date +%s) < deadline )); do
    status_json="$(request_json "$name" "$service_host" localStatus || true)"
    if [[ -n "$status_json" ]] && jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$status_json"; then
      printf '%s\n' "$status_json"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$status_json"
  if [[ -n "$status_json" ]]; then
    request_json "$name" "$service_host" localControlStatus || true
  fi
  dump_container_debug "$name"
  fail "${name} did not reach signed-in state"
}

restart_console() {
  local name="$1"
  local service_host="$2"
  local device_alias="$3"
  container_exec "$name" "pkill -f slan-client-v2-console >/dev/null 2>&1 || true"
  start_console "$name" "$service_host" "$device_alias"
}

wait_control_ready() {
  local name="$1"
  local service_host="$2"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local status_json=''
  local mqtt_connect_attempted=0
  while (( $(date +%s) < deadline )); do
    status_json="$(request_json "$name" "$service_host" localControlStatus || true)"
    if [[ -n "$status_json" ]] && jq -e '.ready == true' >/dev/null <<<"$status_json"; then
      return 0
    fi
    if [[ "$mqtt_connect_attempted" != "1" ]] && [[ -n "$status_json" ]] && jq -e '
      (.missing // []) | index("mqtt") != null
    ' >/dev/null <<<"$status_json"; then
      request_json "$name" "$service_host" localEnsureDevice >/dev/null 2>&1 || true
      request_json "$name" "$service_host" localConnectControlMqtt >/dev/null 2>&1 || true
      mqtt_connect_attempted=1
      continue
    fi
    sleep 1
  done
  printf '%s\n' "$status_json"
  dump_container_debug "$name"
  fail "${name} control transport did not become ready"
}

create_dns_zone_and_records() {
  ZONE_NAME="linux-dual-$(date +%s).slan.test"
  log "create dns zone ${ZONE_NAME}"
  local zone_json
  zone_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks/${NETWORK_ID}/dns/zones" \
    -H 'Content-Type: application/json' \
    -d "{\"zoneName\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(printf '%s' "$zone_json" | jq -r '.zoneId // empty')"
  [[ -n "$ZONE_ID" ]] || fail "dns zone create returned empty zoneId"
}

create_dns_record() {
  local record_name="$1"
  local target_device_id="$2"
  local record_json record_id
  record_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks/${NETWORK_ID}/dns/records" \
    -H 'Content-Type: application/json' \
    -d "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"${record_name}\",\"recordType\":\"A\",\"targetDeviceId\":\"${target_device_id}\",\"port\":\"443\",\"ttl\":60}")"
  record_id="$(printf '%s' "$record_json" | jq -r '.recordId // empty')"
  [[ -n "$record_id" ]] || fail "dns record create returned empty recordId for ${record_name}"
  RECORD_IDS+=("$record_id")
}

security_group_id() {
  curl --silent --show-error --fail \
    "${WEB_API_BASE_URL}${WEB_API_PREFIX}/networks/${NETWORK_ID}/security-groups" | jq -r '.items[0].securityGroupId // empty'
}

add_rule() {
  local security_group_id="$1"
  local direction="$2"
  local protocol="$3"
  local port="$4"
  local peer_type="$5"
  local peer_value="$6"
  local rule_json rule_id
  rule_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_API_BASE_URL}${WEB_API_PREFIX}/security-groups/${security_group_id}/rules" \
    -H 'Content-Type: application/json' \
    -d "{\"direction\":\"${direction}\",\"priority\":100,\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"${peer_type}\",\"peerValue\":\"${peer_value}\",\"enabled\":true}")"
  rule_id="$(printf '%s' "$rule_json" | jq -r '.ruleId // empty')"
  [[ -n "$rule_id" ]] || fail "failed to create ${protocol}:${port} ${direction} rule for ${peer_value}"
  RULE_IDS+=("$rule_id")
}

wait_network_module() {
  local name="$1"
  local service_host="$2"
  local min_peers="$3"
  local min_dns="$4"
  local min_rules="$5"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local module_json=''
  local last_nonempty_module_json=''
  local peer_count=0
  local dns_count=0
  local rule_count=0
  log "wait ${name} network module peers>=${min_peers} dns>=${min_dns} rules>=${min_rules}"
  while (( $(date +%s) < deadline )); do
    module_json="$(request_json "$name" "$service_host" localNetworkModule || true)"
    if [[ -n "$module_json" ]]; then
      last_nonempty_module_json="$module_json"
      peer_count="$(jq -r '(.peerCount // ([.configs[]?.peers[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      dns_count="$(jq -r '(.resolverRecordCount // ([.configs[]?.resolverRecords[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      rule_count="$(jq -r '(.securityRuleCount // ([.configs[]?.rules[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
    fi
    if [[ -n "$module_json" ]] && \
      (( peer_count >= min_peers && dns_count >= min_dns && rule_count >= min_rules )); then
      return 0
    fi
    sleep 1
  done
  for _ in 1 2 3 4 5; do
    module_json="$(request_json "$name" "$service_host" localNetworkModule || true)"
    if [[ -n "$module_json" ]]; then
      last_nonempty_module_json="$module_json"
      peer_count="$(jq -r '(.peerCount // ([.configs[]?.peers[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      dns_count="$(jq -r '(.resolverRecordCount // ([.configs[]?.resolverRecords[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      rule_count="$(jq -r '(.securityRuleCount // ([.configs[]?.rules[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
    fi
    if [[ -n "$module_json" ]] && \
      (( peer_count >= min_peers && dns_count >= min_dns && rule_count >= min_rules )); then
      return 0
    fi
    sleep 2
  done
  printf '%s\n' "${last_nonempty_module_json:-$module_json}"
  dump_container_debug "$name"
  fail "${name} network module did not receive expected dns/acl config"
}

wait_network_settled() {
  local name="$1"
  local service_host="$2"
  local min_peers="${3:-1}"
  local min_dns="${4:-0}"
  local min_rules="${5:-0}"
  local consecutive_target="${6:-8}"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local consecutive=0
  local status_json=''
  local control_json=''
  local module_json=''
  local peer_count=0
  local dns_count=0
  local rule_count=0
  log "wait ${name} network settled peers>=${min_peers} dns>=${min_dns} rules>=${min_rules} stable=${consecutive_target}"
  while (( $(date +%s) < deadline )); do
    status_json="$(request_json "$name" "$service_host" localStatus || true)"
    control_json="$(request_json "$name" "$service_host" localControlStatus || true)"
    module_json="$(request_json "$name" "$service_host" localNetworkModule || true)"
    if [[ -n "$module_json" ]]; then
      peer_count="$(jq -r '(.peerCount // ([.configs[]?.peers[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      dns_count="$(jq -r '(.resolverRecordCount // ([.configs[]?.resolverRecords[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      rule_count="$(jq -r '(.securityRuleCount // ([.configs[]?.rules[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
    fi
    if [[ -n "$status_json" && -n "$control_json" && -n "$module_json" ]] && \
      jq -e '.signedIn == true and .networkEnabled == true and .syncing == false and (.virtualIp // "" | length > 0)' >/dev/null <<<"$status_json" && \
      jq -e '.ready == true' >/dev/null <<<"$control_json" && \
      (( peer_count >= min_peers && dns_count >= min_dns && rule_count >= min_rules )); then
      consecutive=$((consecutive + 1))
      if (( consecutive >= consecutive_target )); then
        return 0
      fi
    else
      consecutive=0
    fi
    sleep 2
  done
  printf '%s\n' "${status_json:-$control_json}"
  printf '%s\n' "$module_json"
  dump_container_debug "$name"
  fail "${name} network did not settle after config updates"
}

ensure_network_ready() {
  local name="$1"
  local service_host="$2"
  local response status_json
  status_json="$(request_json "$name" "$service_host" localStatus || true)"
  if [[ -n "$status_json" ]] && jq -e '.networkEnabled == true and (.virtualIp // "" | length > 0)' >/dev/null <<<"$status_json"; then
    printf '%s\n' "$status_json"
    return 0
  fi
  response="$(request_json "$name" "$service_host" localNetworkActivate || true)"
  if [[ -n "$response" ]] && jq -e '.networkEnabled == true and (.virtualIp // "" | length > 0)' >/dev/null <<<"$response"; then
    printf '%s\n' "$response"
    return 0
  fi
  wait_packet_tunnel_ready "$name" "$service_host"
}

send_client_message() {
  local from_name="$1"
  local from_service_host="$2"
  local target_device_id="$3"
  local body="$4"
  local response args_json payload
  args_json="$(jq -cn \
    --arg target_device_id "$target_device_id" \
    --arg body "$body" \
    '{targetDeviceId:$target_device_id,body:$body,metadata:{smoke:"linux-dual-docker"}}')"
  payload="$(printf '{"method":"%s","args":%s}\n' "localSendClientMessage" "$args_json")"
  response="$(
    printf '%s' "$payload" | docker exec -i "$from_name" python3 -c '
import socket
import sys

try:
    host = sys.argv[1]
    port = int(sys.argv[2])
    timeout = float(sys.argv[3])
    payload = sys.stdin.buffer.read()
    if not payload.endswith(b"\n"):
        payload += b"\n"

    sock = socket.create_connection((host, port), timeout=timeout)
    sock.settimeout(timeout)
    try:
        sock.sendall(payload)
        sock.shutdown(socket.SHUT_WR)
        chunks = []
        while True:
            chunk = sock.recv(65536)
            if not chunk:
                break
            chunks.append(chunk)
            if b"\n" in chunk:
                break
        sys.stdout.buffer.write(b"".join(chunks))
    finally:
        sock.close()
except Exception:
    sys.exit(1)
' "${from_service_host%:*}" "${from_service_host##*:}" "${LOCAL_API_TIMEOUT_SECONDS}" || true
  )"
  jq -e '(.messageId // "" | length > 0)' >/dev/null <<<"$response" || {
    printf '%s\n' "$response"
    fail "${from_name} send client message failed"
  }
}

wait_client_message() {
  local name="$1"
  local service_host="$2"
  local from_device_id="$3"
  local body="$4"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local response=''
  local last_state=''
  local persisted_payload=''
  while (( $(date +%s) < deadline )); do
    persisted_payload="$(container_exec "$name" "cat /var/lib/SLAN/client-v2-last-client-message.json 2>/dev/null || cat /root/.local/share/SLAN/client-v2-last-client-message.json 2>/dev/null || true" || true)"
    if [[ -n "$persisted_payload" ]] && jq -e --arg from_device_id "$from_device_id" --arg body "$body" '
      .payload.fromDeviceId == $from_device_id and .payload.body == $body
    ' >/dev/null <<<"$persisted_payload"; then
      return 0
    fi

    response="$(request_json "$name" "$service_host" localState || true)"
    if [[ -n "$response" ]]; then
      last_state="$response"
      if jq -e --arg from_device_id "$from_device_id" --arg body "$body" \
        '.lastClientMessageFromDeviceId == $from_device_id and .lastClientMessageBody == $body' \
        >/dev/null <<<"$response"; then
        return 0
      fi
    fi
    sleep 1
  done
  printf '%s\n' "${persisted_payload:-${last_state:-$response}}"
  request_json "$name" "$service_host" localControlStatus || true
  dump_container_debug "$name"
  fail "${name} did not receive client message from ${from_device_id}"
}

run_packet_server() {
  local name="$1"
  docker exec -d "$name" bash -lc "
set -euo pipefail
exec python3 - '${UDP_PORT}' '${TCP_PORT}' >/tmp/socket-echo.out 2>/tmp/socket-echo.err <<'PY'
import socket
import sys
import threading
import time

udp_port = int(sys.argv[1])
tcp_port = int(sys.argv[2])
stop = False


def log(message: str) -> None:
    print(message, flush=True)


def run_udp() -> None:
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(('0.0.0.0', udp_port))
    sock.settimeout(0.5)
    log(f'SOCKET_ECHO_UDP_READY={udp_port}')
    while not stop:
      try:
        data, addr = sock.recvfrom(2048)
      except socket.timeout:
        continue
      body = data.decode()
      log(f'SOCKET_ECHO_UDP_RECEIVED={addr[0]}:{addr[1]} body={body}')
      payload = f'echo:{body}'.encode()
      sock.sendto(payload, addr)
      log(f'SOCKET_ECHO_UDP_SENT={addr[0]}:{addr[1]} bytes={len(payload)}')


def handle_tcp(conn: socket.socket, addr) -> None:
    try:
        conn.settimeout(10)
        data = b''
        while not data.endswith(b'\n'):
            chunk = conn.recv(2048)
            if not chunk:
                break
            data += chunk
        body = data.decode().rstrip('\n')
        log(f'SOCKET_ECHO_TCP_RECEIVED={addr[0]}:{addr[1]} body={body}')
        payload = f'echo:{body}\n'.encode()
        conn.sendall(payload)
        log(f'SOCKET_ECHO_TCP_SENT={addr[0]}:{addr[1]} bytes={len(payload)}')
        conn.shutdown(socket.SHUT_WR)
        time.sleep(0.5)
    except Exception as exc:
        log(f'SOCKET_ECHO_TCP_ERROR={addr[0]}:{addr[1]} error={exc!r}')
    finally:
        conn.close()


def run_tcp() -> None:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(('0.0.0.0', tcp_port))
    sock.listen()
    sock.settimeout(0.5)
    log(f'SOCKET_ECHO_TCP_READY={tcp_port}')
    while not stop:
        try:
            conn, addr = sock.accept()
        except socket.timeout:
            continue
        threading.Thread(target=handle_tcp, args=(conn, addr), daemon=True).start()


threading.Thread(target=run_udp, daemon=True).start()
threading.Thread(target=run_tcp, daemon=True).start()

while True:
    time.sleep(3600)
PY
" >/dev/null
}

resolve_record_from_module() {
  local name="$1"
  local service_host="$2"
  local record_name="$3"
  local fqdn="${record_name}.${ZONE_NAME}"
  local module_json ip
  module_json="$(request_json "$name" "$service_host" localNetworkModule || true)"
  ip="$(jq -r --arg fqdn "$fqdn" --arg name "$record_name" '
    .configs[]? as $config
    | $config.resolverRecords[]?
    | select((.fqdn // "" | ascii_downcase) == ($fqdn | ascii_downcase) or (.name // "" | ascii_downcase) == ($name | ascii_downcase))
    | if (.targetIp // "") != "" then
        .targetIp
      else
        (.targetDeviceId // "") as $targetDeviceId
        | (
            $config.peers[]?
            | select((.deviceId // "") == $targetDeviceId)
            | .globalIp // empty
          )
      end
  ' <<<"$module_json" | head -n 1)"
  [[ -n "$ip" ]] || fail "failed to resolve ${fqdn} from localNetworkModule"
  printf '%s\n' "$ip"
}

wait_packet_tunnel_ready() {
  local name="$1"
  local service_host="$2"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local status=''
  while (( $(date +%s) < deadline )); do
    status="$(request_json "$name" "$service_host" localStatus || true)"
    if jq -e '.networkEnabled == true and (.virtualIp // "" | length > 0)' >/dev/null <<<"$status"; then
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$status"
  fail "${name} packet tunnel did not become ready"
}

udp_echo_check() {
  local from_name="$1"
  local target_ip="$2"
  local body="$3"
  local output
  output="$(container_exec "$from_name" "
python3 - '${target_ip}' '${UDP_PORT}' '${body}' <<'PY'
import socket, sys
target_ip = sys.argv[1]
port = int(sys.argv[2])
body = sys.argv[3].encode()
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(8)
s.sendto(body, (target_ip, port))
data, _ = s.recvfrom(2048)
print(data.decode())
PY
" 2>/dev/null || true)"
  [[ "$output" == "echo:${body}" ]] || fail "UDP echo failed from ${from_name}: got=${output:-<empty>} want=echo:${body}"
}

tcp_echo_check() {
  local from_name="$1"
  local target_ip="$2"
  local body="$3"
  local output
  output="$(container_exec "$from_name" "
python3 - '${target_ip}' '${TCP_PORT}' '${body}' <<'PY'
import socket, sys
target_ip = sys.argv[1]
port = int(sys.argv[2])
body = (sys.argv[3] + '\\n').encode()
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(8)
try:
    s.connect((target_ip, port))
    s.sendall(body)
    chunks = []
    while True:
        chunk = s.recv(2048)
        if not chunk:
            break
        chunks.append(chunk)
        if b'\\n' in chunk:
            break
    print(b''.join(chunks).decode().strip())
except Exception as exc:
    print(f'ERROR:{exc!r}')
    raise
finally:
    s.close()
PY
" 2>/dev/null || true)"
  [[ "$output" == "echo:${body}" ]] || fail "TCP echo failed from ${from_name}: got=${output:-<empty>} want=echo:${body}"
}

main() {
  need curl
  need jq
  need docker

  ensure_package
  register_and_login
  resolve_network

  log "provision logged-in ${CONTAINER_A} via stable single-container flow"
  provision_logged_in_container "$CONTAINER_A" "$SERVICE_HOST_A" "Docker Linux A" "$RESULT_JSON_A"
  log "provision logged-in ${CONTAINER_B} via stable single-container flow"
  provision_logged_in_container "$CONTAINER_B" "$SERVICE_HOST_B" "Docker Linux B" "$RESULT_JSON_B"

  BOOTSTRAP_ID_A="$(jq -r '.bootstrapId // empty' "$RESULT_JSON_A")"
  BOOTSTRAP_ID_B="$(jq -r '.bootstrapId // empty' "$RESULT_JSON_B")"
  USER_ID="$(jq -r '.userId // empty' "$RESULT_JSON_A")"
  USER_TOKEN="$(jq -r '.userToken // empty' "$RESULT_JSON_A")"
  NETWORK_ID="$(jq -r '.networkId // empty' "$RESULT_JSON_A")"
  local device_id_a device_id_b
  device_id_a="$(jq -r '.deviceId // empty' "$RESULT_JSON_A")"
  device_id_b="$(jq -r '.deviceId // empty' "$RESULT_JSON_B")"
  [[ -n "$device_id_a" && -n "$device_id_b" ]] || fail "resolved empty device ids"

  create_dns_zone_and_records
  create_dns_record appa "$device_id_a"
  create_dns_record appb "$device_id_b"

  local sg_id
  sg_id="$(security_group_id)"
  [[ -n "$sg_id" ]] || fail "network ${NETWORK_ID} has no security group"
  add_rule "$sg_id" ingress tcp 443 device "$device_id_a"
  add_rule "$sg_id" egress tcp 443 device "$device_id_b"
  add_rule "$sg_id" ingress tcp 443 device "$device_id_b"
  add_rule "$sg_id" egress tcp 443 device "$device_id_a"
  add_rule "$sg_id" ingress udp "$UDP_PORT" device "$device_id_a"
  add_rule "$sg_id" egress udp "$UDP_PORT" device "$device_id_b"
  add_rule "$sg_id" ingress udp "$UDP_PORT" device "$device_id_b"
  add_rule "$sg_id" egress udp "$UDP_PORT" device "$device_id_a"
  add_rule "$sg_id" ingress tcp "$TCP_PORT" device "$device_id_a"
  add_rule "$sg_id" egress tcp "$TCP_PORT" device "$device_id_b"
  add_rule "$sg_id" ingress tcp "$TCP_PORT" device "$device_id_b"
  add_rule "$sg_id" egress tcp "$TCP_PORT" device "$device_id_a"

  wait_network_module "$CONTAINER_A" "$SERVICE_HOST_A" 1 2 4
  wait_network_module "$CONTAINER_B" "$SERVICE_HOST_B" 1 2 4

  if [[ "$RUN_PACKET_TESTS" == "1" ]]; then
    log "ensure dual client networks are ready"
    ensure_network_ready "$CONTAINER_A" "$SERVICE_HOST_A" >/dev/null
    ensure_network_ready "$CONTAINER_B" "$SERVICE_HOST_B" >/dev/null
  fi

  wait_network_settled "$CONTAINER_A" "$SERVICE_HOST_A" 1 2 4
  wait_network_settled "$CONTAINER_B" "$SERVICE_HOST_B" 1 2 4

  log "verify bidirectional client_message"
  local body_ab body_ba
  body_ab="linux-a-to-b-$(date +%s%N)"
  body_ba="linux-b-to-a-$(date +%s%N)"
  log "send ${CONTAINER_A} -> ${CONTAINER_B}"
  send_client_message "$CONTAINER_A" "$SERVICE_HOST_A" "$device_id_b" "$body_ab"
  log "wait ${CONTAINER_B} receive from ${CONTAINER_A}"
  wait_client_message "$CONTAINER_B" "$SERVICE_HOST_B" "$device_id_a" "$body_ab"
  log "send ${CONTAINER_B} -> ${CONTAINER_A}"
  send_client_message "$CONTAINER_B" "$SERVICE_HOST_B" "$device_id_a" "$body_ba"
  log "wait ${CONTAINER_A} receive from ${CONTAINER_B}"
  wait_client_message "$CONTAINER_A" "$SERVICE_HOST_A" "$device_id_b" "$body_ba"

  if [[ "$RUN_PACKET_TESTS" == "1" ]]; then
    log "run UDP/TCP packet tests between dual containers"
    run_packet_server "$CONTAINER_A"
    run_packet_server "$CONTAINER_B"
    sleep 5
    local ip_a ip_b
    ip_a="$(resolve_record_from_module "$CONTAINER_B" "$SERVICE_HOST_B" appa)"
    ip_b="$(resolve_record_from_module "$CONTAINER_A" "$SERVICE_HOST_A" appb)"
    udp_echo_check "$CONTAINER_A" "$ip_b" "udp-ab-$(date +%s%N)"
    udp_echo_check "$CONTAINER_B" "$ip_a" "udp-ba-$(date +%s%N)"
    tcp_echo_check "$CONTAINER_A" "$ip_b" "tcp-ab-$(date +%s%N)"
    tcp_echo_check "$CONTAINER_B" "$ip_a" "tcp-ba-$(date +%s%N)"
  fi

  printf 'linuxDualDockerIntegration: ok email=%s networkId=%s zone=%s deviceA=%s deviceB=%s packetTests=%s\n' \
    "$EMAIL" "$NETWORK_ID" "$ZONE_NAME" "$device_id_a" "$device_id_b" "$RUN_PACKET_TESTS"
}

main "$@"
