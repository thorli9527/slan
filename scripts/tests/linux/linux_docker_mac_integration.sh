#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"

IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
BUILD_PACKAGE="${SLAN_LINUX_DOCKER_MAC_BUILD_PACKAGE:-0}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_MAC_NAME:-slan-linux-mac-check}"
CONTAINER_PRIVILEGED="${SLAN_LINUX_DOCKER_CONTAINER_PRIVILEGED:-1}"
KEEP_CONTAINER="${SLAN_LINUX_DOCKER_KEEP_CONTAINER:-0}"
KEEP_WORK_DIR="${SLAN_KEEP_LINUX_DOCKER_MAC_WORK_DIR:-0}"
KEEP_REMOTE_RESOURCES="${SLAN_KEEP_LINUX_DOCKER_MAC_REMOTE_RESOURCES:-0}"
LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-0}"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
REGISTER_USER="${SLAN_TEST_REGISTER_USER:-true}"
TIMEOUT_SECONDS="${SLAN_LINUX_DOCKER_MAC_TIMEOUT_SECONDS:-120}"
BOOTSTRAP_TTL_SECONDS="${SLAN_LINUX_DOCKER_MAC_BOOTSTRAP_TTL_SECONDS:-1800}"
LOCAL_API_TIMEOUT_SECONDS="${SLAN_LINUX_DOCKER_MAC_LOCAL_API_TIMEOUT_SECONDS:-20}"

MAC_SERVICE_MODE="${SLAN_MAC_SERVICE_MODE:-existing}"
MAC_SERVICE_HOST="${SLAN_MAC_SERVICE_HOST:-127.0.0.1:46392}"
DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/rust/target/debug/client-core-service"
if [[ ! -x "$DEFAULT_MAC_SERVICE_BIN" ]]; then
  DEFAULT_MAC_SERVICE_BIN="$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi
MAC_SERVICE_BIN="${SLAN_CLIENT_CORE_SERVICE_BIN:-$DEFAULT_MAC_SERVICE_BIN}"
MACOS_APP_PATH="${SLAN_MACOS_APP_PATH:-$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
MAC_TEST_DEVICE_ID="${SLAN_MAC_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
RESET_EXISTING_MAC_SERVICE_IDENTITY="${SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY:-1}"
SUDO_PASSWORD="${SLAN_SUDO_PASSWORD:-}"

UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT:-19090}"
TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT:-19091}"
UDP_BODY_MAC_TO_LINUX="${SLAN_TEST_UDP_BODY_MAC_TO_LINUX:-mac-to-linux-udp-$(date +%s%N)}"
TCP_BODY_MAC_TO_LINUX="${SLAN_TEST_TCP_BODY_MAC_TO_LINUX:-mac-to-linux-tcp-$(date +%s%N)}"
UDP_BODY_LINUX_TO_MAC="${SLAN_TEST_UDP_BODY_LINUX_TO_MAC:-linux-to-mac-udp-$(date +%s%N)}"
TCP_BODY_LINUX_TO_MAC="${SLAN_TEST_TCP_BODY_LINUX_TO_MAC:-linux-to-mac-tcp-$(date +%s%N)}"

if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
  GENERATED_TEST_EMAIL=0
else
  EMAIL="linux-mac-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"

WORK_DIR="${SLAN_LINUX_DOCKER_MAC_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-linux-docker-mac.XXXXXX")}"
RESULT_JSON_PATH="$WORK_DIR/linux.json"
LINUX_ECHO_LOG="$WORK_DIR/linux-echo.log"
MAC_ECHO_LOG="$WORK_DIR/mac-echo.log"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"

BOOTSTRAP_ID=""
BOOTSTRAP_KEY=""
USER_ID=""
USER_TOKEN=""
NETWORK_ID=""
ZONE_ID=""
ZONE_NAME=""
SECURITY_GROUP_ID=""
RECORD_IDS=()
RULE_IDS=()
MAC_DEVICE_ID=""
MAC_IP=""
LINUX_DEVICE_ID=""
LINUX_IP=""

PIDS=()

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

PACKAGE_PATH="$(resolve_linux_package_path)"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

log "linux docker + mac defaults: account=${EMAIL} biz=${BIZ_URL} web=${WEB_BASE_URL}"

json_value() {
  local key="$1"
  jq -r --arg key "$key" '.[$key] // empty'
}

is_truthy() {
  local value="${1:-}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

best_effort_delete() {
  local url="$1"
  curl --silent --show-error --connect-timeout 5 --max-time 20 -X DELETE "$url" >/dev/null 2>&1 || true
}

create_json() {
  local url="$1"
  local payload="$2"
  curl --silent --show-error --fail --connect-timeout 5 --max-time 30 \
    -X POST "$url" \
    -H 'Content-Type: application/json' \
    -d "$payload"
}

sudo_run() {
  if [[ -n "$SUDO_PASSWORD" ]]; then
    printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

sha256_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

docker_exec() {
  docker exec "$CONTAINER_NAME" bash -lc "$1"
}

request_json() {
  local method="$1"
  local args="${2:-{}}"
  local payload
  payload="$(printf '{"method":"%s","args":%s}\n' "$method" "$args")"
  printf '%s' "$payload" | docker exec -i "$CONTAINER_NAME" python3 -c '
import socket
import sys

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
' "127.0.0.1" "46392" "${LOCAL_API_TIMEOUT_SECONDS}"
}

run_client_core_login_check() {
  local label="$1"
  shift
  local attempts="${SLAN_CONTROL_RETRY_ATTEMPTS:-3}"
  local attempt output status
  local args=("$@")
  for attempt in $(seq 1 "$attempts"); do
    set +e
    output="$(
      cd "$ROOT_DIR"
      /opt/homebrew/bin/go run scripts/client_core_service_login_check.go "${args[@]}" 2>&1
    )"
    status=$?
    set -e
    if [[ $status -eq 0 ]]; then
      echo "$output"
      return 0
    fi
    echo "$label attempt $attempt/$attempts failed: $output" >&2
    if [[ "$output" == *"HTTP 409"* ]]; then
      for index in "${!args[@]}"; do
        if [[ "${args[$index]}" == "-register=true" ]]; then
          args[$index]="-register=false"
        fi
      done
    fi
    if [[ "$attempt" != "$attempts" ]]; then
      sleep $((attempt * 5))
    fi
  done
  echo "$output"
  return "$status"
}

ensure_package() {
  if [[ "$BUILD_PACKAGE" == "1" || ! -f "$PACKAGE_PATH" ]]; then
    log "build Linux package in docker"
    bash "$ROOT_DIR/scripts/build_linux_client_docker.sh"
  fi
  [[ -f "$PACKAGE_PATH" ]] || fail "Linux client package not found: $PACKAGE_PATH"
}

register_and_login() {
  log "register/login test user"
  curl --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true

  local auth_json
  auth_json="$(curl --silent --show-error --fail \
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
  networks_json="$(curl --silent --show-error --fail \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    "${WEB_BASE_URL}/api/web/networks?userId=${USER_ID}")"
  NETWORK_ID="$(printf '%s' "$networks_json" | jq -r '.items[0].networkId // .[0].networkId // empty')"
  [[ -n "$NETWORK_ID" ]] || fail "failed to resolve default network"
}

create_bootstrap_key() {
  local bootstrap_json
  bootstrap_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_BASE_URL}/api/web/device-bootstrap-keys" \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"Docker Linux Mac Check\",\"ttlSeconds\":${BOOTSTRAP_TTL_SECONDS}}")"
  BOOTSTRAP_ID="$(printf '%s' "$bootstrap_json" | json_value id)"
  BOOTSTRAP_KEY="$(printf '%s' "$bootstrap_json" | json_value key)"
  [[ -n "$BOOTSTRAP_ID" && -n "$BOOTSTRAP_KEY" ]] || fail "failed to create bootstrap key"
}

start_container() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  local docker_args=(
    -d
    --name "$CONTAINER_NAME"
    -v "$ROOT_DIR:/workspace/slan"
  )
  case "$(basename "$PACKAGE_PATH")" in
    *-amd64.tar.gz|*_amd64.deb)
      docker_args+=(--platform linux/amd64)
      ;;
    *-arm64.tar.gz|*_arm64.deb)
      docker_args+=(--platform linux/arm64)
      ;;
  esac
  if [[ "$CONTAINER_PRIVILEGED" == "1" ]]; then
    docker_args+=(--privileged)
  else
    docker_args+=(--cap-add NET_ADMIN --device /dev/net/tun:/dev/net/tun)
  fi
  docker run "${docker_args[@]}" "$IMAGE" sleep infinity >/dev/null
  docker_exec '
set -euo pipefail
mkdir -p /dev/net
if [ ! -e /dev/net/tun ] && [ -e /dev/tun ]; then
  ln -sf /dev/tun /dev/net/tun
fi
'
}

install_container_dependencies() {
  docker_exec '
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update >/dev/null
  apt-get install -y bash curl ca-certificates tar jq netcat-openbsd procps iproute2 iputils-ping dnsutils python3 >/dev/null
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache bash curl ca-certificates tar jq netcat-openbsd procps iproute2 iputils bind-tools python3 >/dev/null
else
  echo unsupported base image package manager >&2
  exit 1
fi
'
}

install_client() {
  local local_package_in_container="/workspace/slan/${PACKAGE_PATH#$ROOT_DIR/}"
  docker_exec "
set -euo pipefail
curl -fsSL '${BIZ_URL}/downloads/clients/install.sh' -o /tmp/slan-install.sh
bash /tmp/slan-install.sh \
  --server='${BIZ_URL}' \
  --installation-key='${BOOTSTRAP_KEY}' \
  --tray=disabled \
  --package-url='file://${local_package_in_container}'
"
}

start_console() {
  docker_exec "pkill -x client-core-service >/dev/null 2>&1 || true"
  docker exec -d "$CONTAINER_NAME" bash -lc "
set -euo pipefail
export SLAN_CLIENT_CORE_SERVICE_HOST='127.0.0.1:46392'
export SLAN_LINUX_NETWORK_MOCK='${LINUX_NETWORK_MOCK}'
exec /usr/bin/slan-client-v2-console \
  --server-url '${BIZ_URL}' \
  --email '${EMAIL}' \
  --password '${PASSWORD}' \
  --device-name 'Docker Linux Mac Check' \
  --foreground >/tmp/slan-console.out 2>/tmp/slan-console.err
" >/dev/null
}

wait_local_api() {
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  while (( $(date +%s) < deadline )); do
    if request_json localStatus >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  fail "linux docker local API did not become ready"
}

wait_signed_in() {
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local status_json=''
  while (( $(date +%s) < deadline )); do
    status_json="$(request_json localStatus || true)"
    if [[ -n "$status_json" ]] && jq -e '.signedIn == true and (.deviceId // "" | length > 0)' >/dev/null <<<"$status_json"; then
      printf '%s\n' "$status_json"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$status_json"
  fail "linux docker did not reach signed-in state"
}

wait_control_ready() {
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local status_json=''
  local mqtt_connect_attempted=0
  while (( $(date +%s) < deadline )); do
    status_json="$(request_json localControlStatus || true)"
    if [[ -n "$status_json" ]] && jq -e '.ready == true' >/dev/null <<<"$status_json"; then
      return 0
    fi
    if [[ "$mqtt_connect_attempted" != "1" ]] && [[ -n "$status_json" ]] && jq -e '
      (.missing // []) | index("mqtt") != null
    ' >/dev/null <<<"$status_json"; then
      request_json localEnsureDevice >/dev/null 2>&1 || true
      request_json localConnectControlMqtt >/dev/null 2>&1 || true
      mqtt_connect_attempted=1
      continue
    fi
    sleep 1
  done
  printf '%s\n' "$status_json"
  fail "linux docker control transport did not become ready"
}

ensure_linux_network_ready() {
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local response=''
  while (( $(date +%s) < deadline )); do
    response="$(request_json localNetworkActivate || true)"
    if [[ -n "$response" ]] && jq -e '.networkEnabled == true and (.virtualIp // "" | length > 0)' >/dev/null <<<"$response"; then
      printf '%s\n' "$response"
      return 0
    fi
    response="$(request_json localStatus || true)"
    if [[ -n "$response" ]] && jq -e '.networkEnabled == true and (.virtualIp // "" | length > 0)' >/dev/null <<<"$response"; then
      local candidate_ip
      candidate_ip="$(jq -r '.virtualIp // empty' <<<"$response")"
      candidate_ip="${candidate_ip%%/*}"
      if [[ -n "$MAC_IP" && "$candidate_ip" == "$MAC_IP" ]]; then
        sleep 1
        continue
      fi
      printf '%s\n' "$response"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$response"
  fail "linux docker network did not become ready"
}

verify_existing_macos_service() {
  local expected_bin="$1"
  local expected_hash installed_hash health_output
  [[ -x "$expected_bin" ]] || fail "expected mac client-core-service binary is missing: $expected_bin"
  [[ -x "/Library/Application Support/SLAN/client-core-service" ]] || fail "installed mac client-core-service is missing"
  expected_hash="$(sha256_file "$expected_bin")"
  installed_hash="$(sha256_file "/Library/Application Support/SLAN/client-core-service")"
  [[ "$expected_hash" == "$installed_hash" ]] || fail "installed mac client-core-service is stale; reinstall with scripts/install_macos_service.sh"
  health_output="$(
    run_client_core_login_check "mac service health" \
      -address "$MAC_SERVICE_HOST" \
      -health-only=true \
      -timeout 10s
  )" || fail "installed mac client-core-service local API is unhealthy at $MAC_SERVICE_HOST"
  echo "$health_output"
}

ensure_macos_app_service() {
  [[ -d "$MACOS_APP_PATH" ]] || fail "macOS app bundle is missing: $MACOS_APP_PATH"
  open "$MACOS_APP_PATH"
  local health_output
  health_output="$(
    run_client_core_login_check "mac app service health" \
      -address "$MAC_SERVICE_HOST" \
      -health-only=true \
      -timeout 15s
  )" || fail "macOS app-hosted client-core-service local API is unhealthy at $MAC_SERVICE_HOST"
  echo "$health_output"
}

reset_existing_macos_service_identity() {
  local expected_bin="$1"
  local -a install_cmd=(
    env
    SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST"
    SLAN_CONTROL_BASE_URL="$BIZ_URL"
    SLAN_MACOS_NETWORK_MOCK="$MACOS_NETWORK_MOCK"
    SLAN_RESET_MACOS_IDENTITY=1
    "$ROOT_DIR/scripts/install_macos_service.sh"
    --binary "$expected_bin"
  )
  if [[ "$MAC_SERVICE_MODE" != "existing" ]]; then
    return 0
  fi
  if ! is_truthy "$RESET_EXISTING_MAC_SERVICE_IDENTITY"; then
    return 0
  fi
  log "reset existing mac client identity"
  if [[ $EUID -eq 0 ]]; then
    "${install_cmd[@]}"
  else
    sudo_run "${install_cmd[@]}"
  fi
}

start_mac_service_if_needed() {
  if [[ "$MAC_SERVICE_MODE" == "service" ]]; then
    mkdir -p "$WORK_DIR/state"
    log "start mac client-core-service on $MAC_SERVICE_HOST"
    SLAN_CLIENT_CORE_SERVICE_HOST="$MAC_SERVICE_HOST" \
      SLAN_CONTROL_BASE_URL="$BIZ_URL" \
      SLAN_CLIENT_DEVICE_ID="$MAC_TEST_DEVICE_ID" \
      SLAN_MACOS_NETWORK_MOCK="$MACOS_NETWORK_MOCK" \
      SLAN_STATE_DIR="$WORK_DIR/state" \
      "$MAC_SERVICE_BIN" >"$MAC_SERVICE_LOG" 2>&1 &
    PIDS+=("$!")
    return 0
  fi
  if [[ "$MAC_SERVICE_MODE" == "app" ]]; then
    ensure_macos_app_service
    return 0
  fi
  reset_existing_macos_service_identity "$MAC_SERVICE_BIN"
  verify_existing_macos_service "$MAC_SERVICE_BIN"
}

login_and_enable_mac() {
  local output
  output="$(
    run_client_core_login_check "mac docker login" \
      -biz-url "$BIZ_URL" \
      -address "$MAC_SERVICE_HOST" \
      -email "$EMAIL" \
      -password "$PASSWORD" \
      -register=false \
      -enable-network=true \
      -timeout 90s
  )" || {
    echo "$output" >&2
    fail "failed to login/enable mac service network"
  }
  echo "$output"
  MAC_DEVICE_ID="$(echo "$output" | sed -n 's/.*deviceId=\([^ ]*\).*/\1/p' | tail -n 1)"
  MAC_IP="$(echo "$output" | sed -n 's/.*clientCoreServiceNetwork: enabled virtualIp=\([^ ]*\).*/\1/p' | tail -n 1)"
  [[ -n "$MAC_DEVICE_ID" ]] || fail "failed to parse Mac device id"
  [[ -n "$MAC_IP" ]] || fail "failed to parse Mac virtual IP"
  MAC_IP="${MAC_IP%%/*}"
}

provision_linux_container() {
  log "start Linux Docker container: $IMAGE"
  start_container
  log "install runtime dependencies inside container"
  install_container_dependencies
  log "install local package inside container"
  install_client
  log "start client-core-service in foreground via console bootstrap"
  start_console
  log "wait for Linux Docker local API"
  wait_local_api
  log "wait for Linux Docker signed-in local status"
  local signed_in_json
  signed_in_json="$(wait_signed_in)"
  LINUX_DEVICE_ID="$(jq -r '.deviceId // empty' <<<"$signed_in_json")"
  [[ -n "$LINUX_DEVICE_ID" ]] || fail "failed to parse Linux Docker device id"
  log "wait for Linux Docker control transport ready"
  wait_control_ready
}

resolve_security_group() {
  local groups_json
  groups_json="$(curl --silent --show-error --fail "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/security-groups")"
  SECURITY_GROUP_ID="$(printf '%s' "$groups_json" | jq -r '.items[0].securityGroupId // empty')"
  [[ -n "$SECURITY_GROUP_ID" ]] || fail "network ${NETWORK_ID} has no security group"
}

create_dns_zone() {
  ZONE_NAME="linux-mac-$(date +%s).slan.test"
  local zone_json
  zone_json="$(create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/zones" \
    "{\"zoneName\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(printf '%s' "$zone_json" | jq -r '.zoneId // empty')"
  [[ -n "$ZONE_ID" ]] || fail "dns zone create returned empty zoneId"
}

create_dns_record() {
  local name="$1"
  local target_device_id="$2"
  local record_json record_id
  record_json="$(create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records" \
    "{\"zoneId\":\"${ZONE_ID}\",\"name\":\"${name}\",\"recordType\":\"A\",\"targetDeviceId\":\"${target_device_id}\",\"targetIp\":\"\",\"cname\":\"\",\"port\":\"443\",\"ttl\":60}")"
  record_id="$(printf '%s' "$record_json" | jq -r '.recordId // empty')"
  [[ -n "$record_id" ]] || fail "dns record create returned empty recordId for ${name}"
  RECORD_IDS+=("$record_id")
}

add_rule() {
  local direction="$1"
  local protocol="$2"
  local port="$3"
  local peer_value="$4"
  local priority="$5"
  local rule_json rule_id
  rule_json="$(create_json "${WEB_BASE_URL}/api/web/security-groups/${SECURITY_GROUP_ID}/rules" \
    "{\"direction\":\"${direction}\",\"priority\":${priority},\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"device\",\"peerValue\":\"${peer_value}\",\"enabled\":true}")"
  rule_id="$(printf '%s' "$rule_json" | jq -r '.ruleId // empty')"
  [[ -n "$rule_id" ]] || fail "failed to create ${protocol}:${port} ${direction} rule for ${peer_value}"
  RULE_IDS+=("$rule_id")
}

wait_network_module() {
  local min_peers="$1"
  local min_dns="$2"
  local min_rules="$3"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local module_json='' peer_count=0 dns_count=0 rule_count=0
  while (( $(date +%s) < deadline )); do
    module_json="$(request_json localNetworkModule || true)"
    if [[ -n "$module_json" ]]; then
      peer_count="$(jq -r '(.peerCount // ([.configs[]?.peers[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      dns_count="$(jq -r '(.dnsRecordCount // ([.configs[]?.dnsRecords[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      rule_count="$(jq -r '(.securityRuleCount // ([.configs[]?.rules[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
    fi
    if [[ -n "$module_json" ]] && (( peer_count >= min_peers && dns_count >= min_dns && rule_count >= min_rules )); then
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$module_json"
  fail "linux docker network module did not receive expected dns/acl config"
}

dump_linux_network_module() {
  local module_json
  module_json="$(request_json localNetworkModule || true)"
  if [[ -n "$module_json" ]]; then
    echo "---- Linux network module snapshot ----" >&2
    printf '%s\n' "$module_json" >&2
  fi
}

send_linux_message() {
  local target_device_id="$1"
  local body="$2"
  local args_json payload response
  args_json="$(jq -cn --arg target_device_id "$target_device_id" --arg body "$body" \
    '{targetDeviceId:$target_device_id,body:$body,metadata:{smoke:"linux-docker-mac"}}')"
  payload="$(printf '{"method":"%s","args":%s}\n' "localSendClientMessage" "$args_json")"
  response="$(
    printf '%s' "$payload" | docker exec -i "$CONTAINER_NAME" python3 -c '
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
payload = sys.stdin.buffer.read()
if not payload.endswith(b"\n"):
    payload += b"\n"
sock = socket.create_connection((host, port), timeout=8)
sock.settimeout(8)
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
' "127.0.0.1" "46392" || true
  )"
  jq -e '(.messageId // "" | length > 0)' >/dev/null <<<"$response" || fail "linux docker send client message failed: ${response:-<empty>}"
}

wait_linux_message() {
  local from_device_id="$1"
  local body="$2"
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local response=''
  while (( $(date +%s) < deadline )); do
    response="$(request_json localState || true)"
    if [[ -n "$response" ]] && jq -e --arg from "$from_device_id" --arg body "$body" \
      '.lastClientMessageFromDeviceId == $from and .lastClientMessageBody == $body' >/dev/null <<<"$response"; then
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$response"
  fail "linux docker did not receive client message from ${from_device_id}"
}

resolve_linux_ip_from_status() {
  local deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  local response='' candidate_ip=''
  while (( $(date +%s) < deadline )); do
    response="$(request_json localStatus || true)"
    candidate_ip="$(jq -r '.virtualIp // empty' <<<"$response" 2>/dev/null || true)"
    candidate_ip="${candidate_ip%%/*}"
    if [[ -n "$candidate_ip" && ( -z "$MAC_IP" || "$candidate_ip" != "$MAC_IP" ) ]]; then
      printf '%s\n' "$candidate_ip"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$candidate_ip"
  fail "failed to resolve stable Linux Docker virtual IP from localStatus"
}

wait_mac_message() {
  local from_device_id="$1"
  local body="$2"
  run_client_core_login_check "mac wait message" \
    -address "$MAC_SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register=false \
    -login=false \
    -expect-from "$from_device_id" \
    -expect-body "$body" \
    -timeout 90s >/dev/null
}

send_mac_message() {
  local target_device_id="$1"
  local body="$2"
  run_client_core_login_check "mac send message" \
    -address "$MAC_SERVICE_HOST" \
    -email "$EMAIL" \
    -password "$PASSWORD" \
    -register=false \
    -login=false \
    -send-target "$target_device_id" \
    -send-body "$body" \
    -timeout 45s >/dev/null
}

run_linux_packet_server() {
  docker exec -d "$CONTAINER_NAME" bash -lc "
set -euo pipefail
exec python3 - '${UDP_PORT}' '${TCP_PORT}' >/tmp/socket-echo.out 2>/tmp/socket-echo.err <<'PY'
import socket
import sys
import threading
import time

udp_port = int(sys.argv[1])
tcp_port = int(sys.argv[2])

def log(message):
    print(message, flush=True)

def run_udp():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(('0.0.0.0', udp_port))
    sock.settimeout(0.5)
    log(f'SOCKET_ECHO_UDP_READY={udp_port}')
    while True:
        try:
            data, addr = sock.recvfrom(2048)
        except socket.timeout:
            continue
        body = data.decode()
        log(f'SOCKET_ECHO_UDP_RECEIVED={addr[0]}:{addr[1]} body={body}')
        payload = f'echo:{body}'.encode()
        sock.sendto(payload, addr)

def handle_tcp(conn, addr):
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
        conn.sendall(f'echo:{body}\n'.encode())
        time.sleep(0.5)
    finally:
        conn.close()

def run_tcp():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(('0.0.0.0', tcp_port))
    sock.listen()
    sock.settimeout(0.5)
    log(f'SOCKET_ECHO_TCP_READY={tcp_port}')
    while True:
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

wait_linux_echo_ready() {
  local deadline=$(( $(date +%s) + 30 ))
  while (( $(date +%s) < deadline )); do
    docker_exec "grep -q 'SOCKET_ECHO_UDP_READY=${UDP_PORT}' /tmp/socket-echo.out && grep -q 'SOCKET_ECHO_TCP_READY=${TCP_PORT}' /tmp/socket-echo.out" >/dev/null 2>&1 && return 0
    sleep 1
  done
  docker_exec "cat /tmp/socket-echo.out 2>/dev/null || true"
  fail "linux docker echo server did not become ready"
}

start_mac_echo_server() {
  (
    cd "$ROOT_DIR"
    /opt/homebrew/bin/go run scripts/socket_echo_server.go \
      -udp-port "$UDP_PORT" \
      -tcp-port "$TCP_PORT" \
      -listen-host "$MAC_IP"
  ) >"$MAC_ECHO_LOG" 2>&1 &
  PIDS+=("$!")
}

wait_mac_echo_ready() {
  local deadline=$(( $(date +%s) + 30 ))
  while (( $(date +%s) < deadline )); do
    if grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$MAC_ECHO_LOG" && grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$MAC_ECHO_LOG"; then
      return 0
    fi
    sleep 1
  done
  cat "$MAC_ECHO_LOG"
  fail "mac echo server did not become ready"
}

resolve_linux_record_from_module() {
  local fqdn="linux.${ZONE_NAME}"
  local module_json ip
  module_json="$(request_json localNetworkModule || true)"
  ip="$(jq -r --arg fqdn "$fqdn" '
    .configs[]? as $config
    | $config.dnsRecords[]?
    | select((.fqdn // "" | ascii_downcase) == ($fqdn | ascii_downcase))
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
  [[ -n "$ip" ]] || fail "failed to resolve ${fqdn} from Linux Docker localNetworkModule"
  printf '%s\n' "$ip"
}

send_mac_udp() {
  local source_ip="$1"
  local target_ip="$2"
  local body="$3"
  local attempts=5
  local output=''
  local status=0
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    set +e
    output="$(python3 - "$source_ip" "$target_ip" "$UDP_PORT" "$body" <<'PY'
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
body = sys.argv[4].encode()
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(3)
try:
    s.bind((source_ip, 0))
    s.sendto(body, (target_ip, port))
    data, peer = s.recvfrom(2048)
    print(data.decode())
except Exception as exc:
    print(f"udp_error:{exc!r}")
    raise
finally:
    s.close()
PY
)"
    status=$?
    set -e
    if [[ $status -eq 0 && "$output" == "echo:${body}" ]]; then
      return 0
    fi
    log "Mac UDP echo retry ${attempt}/${attempts} source=$source_ip target=$target_ip output=${output:-<empty>}"
    sleep "$attempt"
  done
  fail "Mac UDP echo failed: got=${output:-<empty>} want=echo:${body}"
}

send_mac_tcp() {
  local source_ip="$1"
  local target_ip="$2"
  local body="$3"
  local attempts=5
  local output=''
  local status=0
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    set +e
    output="$(python3 - "$source_ip" "$target_ip" "$TCP_PORT" "$body" <<'PY'
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
body = (sys.argv[4] + '\n').encode()
s = None
try:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(3)
    s.bind((source_ip, 0))
    s.connect((target_ip, port))
    s.settimeout(3)
    s.sendall(body)
    print(s.recv(2048).decode().strip())
except Exception as exc:
    print(f"tcp_error:{exc!r}")
    raise
finally:
    try:
        s.close()
    except Exception:
        pass
PY
)"
    status=$?
    set -e
    if [[ $status -eq 0 && "$output" == "echo:${body}" ]]; then
      return 0
    fi
    log "Mac TCP echo retry ${attempt}/${attempts} source=$source_ip target=$target_ip output=${output:-<empty>}"
    sleep "$attempt"
  done
  fail "Mac TCP echo failed: got=${output:-<empty>} want=echo:${body}"
}

warm_mac_to_linux_path() {
  local source_ip="$1"
  local target_ip="$2"
  log "warm macOS -> Linux path source=$source_ip target=$target_ip"
  ping -c 2 "$target_ip" >/dev/null 2>&1 || true
  set +e
  python3 - "$source_ip" "$target_ip" "$UDP_PORT" <<'PY' >/dev/null 2>&1
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(2)
try:
    s.bind((source_ip, 0))
    s.sendto(b"slan-warmup-udp", (target_ip, port))
    s.recvfrom(2048)
except Exception:
    pass
finally:
    s.close()
PY
  python3 - "$source_ip" "$target_ip" "$TCP_PORT" <<'PY' >/dev/null 2>&1
import socket
import sys
source_ip = sys.argv[1]
target_ip = sys.argv[2]
port = int(sys.argv[3])
s = None
try:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(2)
    s.bind((source_ip, 0))
    s.connect((target_ip, port))
    s.settimeout(2)
    s.sendall(b"slan-warmup-tcp\n")
    s.recv(2048)
except Exception:
    pass
finally:
    try:
        s.close()
    except Exception:
        pass
PY
  set -e
  sleep 2
}

send_linux_udp() {
  local target_ip="$1"
  local body="$2"
  local output
  output="$(docker_exec "
python3 - '${target_ip}' '${UDP_PORT}' '${body}' <<'PY'
import socket
import sys
target_ip = sys.argv[1]
port = int(sys.argv[2])
body = sys.argv[3].encode()
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(8)
s.sendto(body, (target_ip, port))
data, _ = s.recvfrom(2048)
print(data.decode())
PY
")"
  [[ "$output" == "echo:${body}" ]] || fail "Linux Docker UDP echo failed: got=${output:-<empty>} want=echo:${body}"
}

send_linux_tcp() {
  local target_ip="$1"
  local body="$2"
  local output
  output="$(docker_exec "
python3 - '${target_ip}' '${TCP_PORT}' '${body}' <<'PY'
import socket
import sys
target_ip = sys.argv[1]
port = int(sys.argv[2])
body = (sys.argv[3] + '\\n').encode()
s = socket.create_connection((target_ip, port), timeout=8)
s.sendall(body)
print(s.recv(2048).decode().strip())
PY
")"
  [[ "$output" == "echo:${body}" ]] || fail "Linux Docker TCP echo failed: got=${output:-<empty>} want=echo:${body}"
}

cleanup() {
  local status=$?
  docker_exec "cat /tmp/socket-echo.out 2>/dev/null || true" >"$LINUX_ECHO_LOG" 2>/dev/null || true
  if [[ $status -ne 0 ]]; then
    dump_linux_network_module
    if [[ -n "${LINUX_IP:-}" ]]; then
      echo "---- macOS route snapshot ----" >&2
      route -n get "$LINUX_IP" >&2 || true
      netstat -rn -f inet | egrep "^10\\.0\\.0\\.($(printf '%s' "$MAC_IP" | awk -F. '{print $4}')|$(printf '%s' "$LINUX_IP" | awk -F. '{print $4}'))\\b" >&2 || true
    fi
    echo "---- macOS interface snapshot ----" >&2
    ifconfig | egrep '^(utun|[[:space:]]inet )' >&2 || true
    echo "---- macOS localStatus snapshot ----" >&2
    python3 - <<'PY' >&2 || true
import socket
payload=b'{"method":"localStatus","args":{}}\n'
sock=socket.create_connection(("127.0.0.1",46392),timeout=5)
sock.sendall(payload)
sock.shutdown(socket.SHUT_WR)
print(sock.recv(65535).decode())
sock.close()
PY
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$MAC_ECHO_LOG" ]] && { echo "---- Mac echo log ----" >&2; cat "$MAC_ECHO_LOG" >&2; }
    [[ -f "$LINUX_ECHO_LOG" ]] && { echo "---- Linux echo log ----" >&2; cat "$LINUX_ECHO_LOG" >&2; }
    docker_exec "cat /tmp/socket-echo.out 2>/dev/null || true" >&2 || true
    docker_exec "cat /tmp/socket-echo.err 2>/dev/null || true" >&2 || true
    docker_exec "cat /tmp/slan-console.out 2>/dev/null || true" >&2 || true
    docker_exec "cat /tmp/slan-console.err 2>/dev/null || true" >&2 || true
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  if [[ "$KEEP_CONTAINER" != "1" ]]; then
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BOOTSTRAP_ID" && -n "$USER_TOKEN" && -n "$USER_ID" ]]; then
    curl --silent --show-error -X POST \
      "${WEB_BASE_URL}/api/web/device-bootstrap-keys/${BOOTSTRAP_ID}/revoke" \
      -H "Authorization: Bearer ${USER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"userId\":\"${USER_ID}\"}" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_REMOTE_RESOURCES" != "1" ]]; then
    for rule_id in "${RULE_IDS[@]:-}"; do
      best_effort_delete "${WEB_BASE_URL}/api/web/security-groups/rules/${rule_id}"
    done
    for record_id in "${RECORD_IDS[@]:-}"; do
      best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records/${record_id}"
    done
    [[ -n "$ZONE_ID" ]] && best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/zones/${ZONE_ID}"
  else
    echo "kept remote network resources: networkId=$NETWORK_ID zoneId=${ZONE_ID:-} ruleCount=${#RULE_IDS[@]} recordCount=${#RECORD_IDS[@]}"
  fi
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
  if [[ "$KEEP_WORK_DIR" == "1" ]]; then
    echo "kept work dir: $WORK_DIR"
  else
    rm -rf "$WORK_DIR"
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

main() {
  need curl
  need jq
  need docker
  need python3
  need /opt/homebrew/bin/go

  ensure_package
  register_and_login
  resolve_network
  create_bootstrap_key

  start_mac_service_if_needed
  log "login and enable Mac network"
  login_and_enable_mac
  log "provision Linux Docker client"
  provision_linux_container

  create_dns_zone
  create_dns_record mac "$MAC_DEVICE_ID"
  create_dns_record linux "$LINUX_DEVICE_ID"
  resolve_security_group
  add_rule ingress tcp 443 "$MAC_DEVICE_ID" 100
  add_rule egress tcp 443 "$LINUX_DEVICE_ID" 110
  add_rule ingress tcp 443 "$LINUX_DEVICE_ID" 120
  add_rule egress tcp 443 "$MAC_DEVICE_ID" 130
  add_rule ingress udp "$UDP_PORT" "$MAC_DEVICE_ID" 140
  add_rule egress udp "$UDP_PORT" "$LINUX_DEVICE_ID" 150
  add_rule ingress udp "$UDP_PORT" "$LINUX_DEVICE_ID" 160
  add_rule egress udp "$UDP_PORT" "$MAC_DEVICE_ID" 170
  add_rule ingress tcp "$TCP_PORT" "$MAC_DEVICE_ID" 180
  add_rule egress tcp "$TCP_PORT" "$LINUX_DEVICE_ID" 190
  add_rule ingress tcp "$TCP_PORT" "$LINUX_DEVICE_ID" 200
  add_rule egress tcp "$TCP_PORT" "$MAC_DEVICE_ID" 210

  log "wait Linux Docker network module receive dns/acl config"
  wait_network_module 1 2 4

  log "enable Linux Docker network"
  local linux_network_json
  linux_network_json="$(ensure_linux_network_ready)"
  LINUX_IP="$(jq -r '.virtualIp // empty' <<<"$linux_network_json")"
  [[ -n "$LINUX_IP" ]] || fail "failed to parse Linux Docker virtual IP"
  LINUX_IP="${LINUX_IP%%/*}"
  if [[ -n "$MAC_IP" && "$LINUX_IP" == "$MAC_IP" ]]; then
    LINUX_IP="$(resolve_linux_ip_from_status)"
  fi
  log "resolved network identities macDeviceId=$MAC_DEVICE_ID macIp=$MAC_IP linuxDeviceId=$LINUX_DEVICE_ID linuxIp=$LINUX_IP"

  log "verify bidirectional client_message"
  local body_linux_to_mac="linux-to-mac-message-$(date +%s%N)"
  local body_mac_to_linux="mac-to-linux-message-$(date +%s%N)"
  send_linux_message "$MAC_DEVICE_ID" "$body_linux_to_mac"
  wait_mac_message "$LINUX_DEVICE_ID" "$body_linux_to_mac"
  send_mac_message "$LINUX_DEVICE_ID" "$body_mac_to_linux"
  wait_linux_message "$MAC_DEVICE_ID" "$body_mac_to_linux"

  if [[ "$MACOS_NETWORK_MOCK" == "1" || "$LINUX_NETWORK_MOCK" == "1" ]]; then
    printf 'linuxDockerMacIntegration: ok email=%s networkId=%s zone=%s mac=%s linux=%s macIp=%s linuxIp=%s packetTests=0 messageTests=1\n' \
      "$EMAIL" "$NETWORK_ID" "$ZONE_NAME" "$MAC_DEVICE_ID" "$LINUX_DEVICE_ID" "${MAC_IP:-mock}" "${LINUX_IP:-mock}"
    return 0
  fi

  log "start Linux Docker echo server"
  run_linux_packet_server
  wait_linux_echo_ready
  log "start Mac echo server"
  start_mac_echo_server
  wait_mac_echo_ready
  log "wait macOS route/data-plane stabilize for Linux peer"
  slan_wait_macos_peer_route_ready "$LINUX_IP" "$MAC_IP" "$MAC_SERVICE_HOST" "${TIMEOUT_SECONDS}" \
    || fail "mac peer route did not become stable for target=${LINUX_IP} source=${MAC_IP}"

  warm_mac_to_linux_path "$MAC_IP" "$LINUX_IP"

  log "run Mac -> Linux UDP/TCP packet tests"
  log "packet target linuxIp=$LINUX_IP udpPort=$UDP_PORT tcpPort=$TCP_PORT"
  send_mac_udp "$MAC_IP" "$LINUX_IP" "$UDP_BODY_MAC_TO_LINUX"
  send_mac_tcp "$MAC_IP" "$LINUX_IP" "$TCP_BODY_MAC_TO_LINUX"

  log "run Linux -> Mac UDP/TCP packet tests"
  log "packet target macIp=$MAC_IP udpPort=$UDP_PORT tcpPort=$TCP_PORT"
  send_linux_udp "$MAC_IP" "$UDP_BODY_LINUX_TO_MAC"
  send_linux_tcp "$MAC_IP" "$TCP_BODY_LINUX_TO_MAC"

  docker_exec "grep -q 'SOCKET_ECHO_UDP_RECEIVED=' /tmp/socket-echo.out"
  docker_exec "grep -q 'SOCKET_ECHO_TCP_RECEIVED=' /tmp/socket-echo.out"
  grep -q 'SOCKET_ECHO_UDP_RECEIVED=' "$MAC_ECHO_LOG"
  grep -q 'SOCKET_ECHO_TCP_RECEIVED=' "$MAC_ECHO_LOG"

  printf 'linuxDockerMacIntegration: ok email=%s networkId=%s zone=%s mac=%s linux=%s macIp=%s linuxIp=%s packetTests=1 messageTests=1\n' \
    "$EMAIL" "$NETWORK_ID" "$ZONE_NAME" "$MAC_DEVICE_ID" "$LINUX_DEVICE_ID" "$MAC_IP" "$LINUX_IP"
}

main "$@"
