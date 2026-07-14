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

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
TIMEOUT_SECONDS="${SLAN_MAC_REMOTE_LINUX_TIMEOUT_SECONDS:-180}"
LOCAL_API_TIMEOUT_SECONDS="${SLAN_MAC_REMOTE_LINUX_LOCAL_API_TIMEOUT_SECONDS:-120}"
BOOTSTRAP_TTL_SECONDS="${SLAN_MAC_REMOTE_LINUX_BOOTSTRAP_TTL_SECONDS:-1800}"

REMOTE_HOST="${SLAN_REMOTE_LINUX_HOST:-100.87.66.24}"
REMOTE_USER="${SLAN_REMOTE_LINUX_USER:-root}"
REMOTE_PASSWORD="${SLAN_REMOTE_LINUX_PASSWORD:-}"
REMOTE_SSH_KEY="${SLAN_REMOTE_LINUX_SSH_KEY:-}"
REMOTE_DIR="${SLAN_REMOTE_LINUX_WORK_DIR:-/tmp/slan-mac-remote-linux}"
REMOTE_HELPER_PATH="$REMOTE_DIR/remote_linux_local_api.sh"
REMOTE_SERVICE_HOST="${SLAN_REMOTE_LINUX_SERVICE_HOST:-127.0.0.1:46392}"
RUN_REMOTE_INSTALL_CHECK="${SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK:-1}"
BUILD_LINUX_PACKAGE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE:-0}"
BUILD_LINUX_PACKAGE_REMOTE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE_REMOTE:-1}"
PREPARE_REMOTE_DEPENDENCIES="${SLAN_REMOTE_LINUX_PREPARE_DEPENDENCIES:-1}"
USE_REMOTE_BUILT_PACKAGE_DIRECTLY="${SLAN_REMOTE_LINUX_USE_REMOTE_BUILT_PACKAGE_DIRECTLY:-0}"
LINUX_DEVICE_ALIAS="${SLAN_REMOTE_LINUX_DEVICE_ALIAS:-Remote Linux CLI}"

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

if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
  GENERATED_TEST_EMAIL=0
else
  EMAIL="mac-remote-linux-$(date +%s%N)@example.test"
  GENERATED_TEST_EMAIL=1
fi
CLEANUP_TEST_DEVICES="${SLAN_CLEANUP_REMOTE_TEST_DEVICES:-$GENERATED_TEST_EMAIL}"

if [[ -n "${SLAN_TEST_UDP_ECHO_PORT:-}" ]]; then
  UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT}"
else
  UDP_PORT="$((21000 + (RANDOM % 10000) * 2))"
fi
if [[ -n "${SLAN_TEST_TCP_ECHO_PORT:-}" ]]; then
  TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT}"
else
  TCP_PORT="$((UDP_PORT + 1))"
fi

MAC_TO_LINUX_BODY="${SLAN_MAC_TO_LINUX_BODY:-hello-mac-to-linux-$(date +%s%N)}"
LINUX_TO_MAC_BODY="${SLAN_LINUX_TO_MAC_BODY:-hello-linux-to-mac-$(date +%s%N)}"
MAC_UDP_BODY="${SLAN_MAC_TO_LINUX_UDP_BODY:-mac-to-linux-udp-$(date +%s%N)}"
MAC_TCP_BODY="${SLAN_MAC_TO_LINUX_TCP_BODY:-mac-to-linux-tcp-$(date +%s%N)}"
LINUX_UDP_BODY="${SLAN_LINUX_TO_MAC_UDP_BODY:-linux-to-mac-udp-$(date +%s%N)}"
LINUX_TCP_BODY="${SLAN_LINUX_TO_MAC_TCP_BODY:-linux-to-mac-tcp-$(date +%s%N)}"

WORK_DIR="${SLAN_MAC_REMOTE_LINUX_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-mac-remote-linux.XXXXXX")}"
REMOTE_PREP_LOG="$WORK_DIR/remote-prepare.log"
MAC_ECHO_LOG="$WORK_DIR/mac-echo.log"
MAC_SERVICE_LOG="$WORK_DIR/macos-service.log"
MAC_CORE_LOG="$WORK_DIR/state/SLAN/client-core-service.log"

USER_ID=""
USER_TOKEN=""
NETWORK_ID=""
SECURITY_GROUP_ID=""
BOOTSTRAP_KEY_ID=""
BOOTSTRAP_KEY=""
DEVICE_GROUP_ID=""
ZONE_ID=""
ZONE_NAME=""
RULE_IDS=()
RECORD_IDS=()
REMOTE_ARCH=""
PACKAGE_PATH=""
REMOTE_PACKAGE_PATH=""
MAC_DEVICE_ID=""
MAC_IP=""
LINUX_DEVICE_ID=""
LINUX_IP=""

PIDS=()

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
  curl --silent --show-error --connect-timeout 5 --max-time 20 \
    -X DELETE "$url" \
    -H "Authorization: Bearer ${USER_TOKEN}" >/dev/null 2>&1 || true
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
      local index
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

verify_existing_macos_service() {
  local expected_bin="$1"
  local expected_hash installed_hash health_output
  [[ -x "$expected_bin" ]] || fail "expected mac client-core-service binary is missing: $expected_bin"
  sudo_run test -x "/Library/Application Support/SLAN/client-core-service" \
    || fail "installed mac client-core-service is missing"
  expected_hash="$(sha256_file "$expected_bin")"
  installed_hash="$(
    sudo_run shasum -a 256 "/Library/Application Support/SLAN/client-core-service" \
      | awk '{print $1}'
  )"
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
    run_client_core_login_check "mac remote linux login" \
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
    if grep -q "SOCKET_ECHO_UDP_READY=$UDP_PORT" "$MAC_ECHO_LOG" &&
      grep -q "SOCKET_ECHO_TCP_READY=$TCP_PORT" "$MAC_ECHO_LOG"; then
      return 0
    fi
    sleep 1
  done
  cat "$MAC_ECHO_LOG"
  fail "mac echo server did not become ready"
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

resolve_record_from_mac_module() {
  local fqdn="$1"
  python3 - "$MAC_SERVICE_HOST" "$fqdn" <<'PY'
import json
import socket
import sys

address = sys.argv[1]
fqdn = sys.argv[2].lower()
host, port = address.rsplit(":", 1)
sock = socket.create_connection((host, int(port)), timeout=5)
sock.settimeout(5)
sock.sendall(b'{"method":"localNetworkModule","args":{}}\n')
sock.shutdown(socket.SHUT_WR)
data = b""
while True:
    chunk = sock.recv(65535)
    if not chunk:
        break
    data += chunk
sock.close()
module = json.loads(data.decode() or "{}")
for config in module.get("configs", []) or []:
    peers = {}
    for peer in config.get("peers", []) or []:
        device_id = str(peer.get("deviceId") or "").strip()
        if not device_id:
            continue
        ip = ""
        for value in peer.get("virtualIps", []) or []:
            value = str(value).strip()
            if value:
                ip = value
                break
        if not ip:
            ip = str(peer.get("globalIp") or "").strip()
        if ip:
            peers[device_id] = ip
    for record in config.get("resolverRecords", []) or []:
        names = {
            str(record.get("fqdn") or "").strip().lower(),
            str(record.get("name") or "").strip().lower(),
        }
        if fqdn not in names:
            continue
        target_ip = str(record.get("targetIp") or "").strip()
        if target_ip:
            print(target_ip)
            raise SystemExit(0)
        peer_ip = peers.get(str(record.get("targetDeviceId") or "").strip(), "")
        if peer_ip:
            print(peer_ip)
            raise SystemExit(0)
raise SystemExit(1)
PY
}

ssh_opts=(
  -o StrictHostKeyChecking=accept-new
  -o ServerAliveInterval=30
  -o ConnectTimeout=10
)

remote_expect_ssh() {
  local remote_command="$1"
  if [[ -n "$REMOTE_SSH_KEY" ]]; then
    ssh "${ssh_opts[@]}" -i "$REMOTE_SSH_KEY" "${REMOTE_USER}@${REMOTE_HOST}" "$remote_command"
    return
  fi
  [[ -n "$REMOTE_PASSWORD" ]] || fail "set SLAN_REMOTE_LINUX_PASSWORD or SLAN_REMOTE_LINUX_SSH_KEY"
  REMOTE_EXPECT_HOST="$REMOTE_HOST" \
  REMOTE_EXPECT_USER="$REMOTE_USER" \
  REMOTE_EXPECT_PASSWORD="$REMOTE_PASSWORD" \
  REMOTE_EXPECT_COMMAND="$remote_command" \
  REMOTE_EXPECT_SSH_OPTS="$(printf '%s\n' "${ssh_opts[@]}")" \
  /usr/bin/expect <<'EOF'
set timeout 120
set ssh_opts [split $env(REMOTE_EXPECT_SSH_OPTS) "\n"]
set remote_host $env(REMOTE_EXPECT_HOST)
set remote_user $env(REMOTE_EXPECT_USER)
set remote_password $env(REMOTE_EXPECT_PASSWORD)
set remote_command $env(REMOTE_EXPECT_COMMAND)
spawn ssh {*}$ssh_opts ${remote_user}@${remote_host} $remote_command
expect {
  "yes/no" { send "yes\r"; exp_continue }
  "*assword:" { send "${remote_password}\r"; exp_continue }
  eof
}
catch wait result
set exit_code [lindex $result 3]
if {$exit_code eq ""} {
  set exit_code 1
}
exit $exit_code
EOF
}

remote_expect_scp() {
  local local_path="$1"
  local remote_path="$2"
  if [[ -n "$REMOTE_SSH_KEY" ]]; then
    scp "${ssh_opts[@]}" -i "$REMOTE_SSH_KEY" "$local_path" "${REMOTE_USER}@${REMOTE_HOST}:$remote_path"
    return
  fi
  [[ -n "$REMOTE_PASSWORD" ]] || fail "set SLAN_REMOTE_LINUX_PASSWORD or SLAN_REMOTE_LINUX_SSH_KEY"
  /usr/bin/expect <<EOF
set timeout 180
spawn scp {*}{${ssh_opts[*]}} $local_path ${REMOTE_USER}@${REMOTE_HOST}:$remote_path
expect {
  "yes/no" { send "yes\r"; exp_continue }
  "*assword:" { send "${REMOTE_PASSWORD}\r"; exp_continue }
  eof
}
catch wait result
exit [lindex \$result 3]
EOF
}

remote_exec() {
  local script="$1"
  local local_script="$WORK_DIR/remote-exec-$(date +%s%N).sh"
  local remote_script="$REMOTE_DIR/remote-exec-$(date +%s%N).sh"
  printf '%s\n' "$script" >"$local_script"
  chmod 700 "$local_script"
  remote_expect_ssh "mkdir -p '$REMOTE_DIR'"
  remote_expect_scp "$local_script" "$remote_script"
  remote_expect_ssh "bash '$remote_script'"
  remote_expect_ssh "rm -f '$remote_script'" || true
  rm -f "$local_script"
}

remote_helper() {
  local command="$1"
  shift
  local args=("$@")
  local quoted=""
  local arg
  for arg in "${args[@]}"; do
    quoted+=" $(printf '%q' "$arg")"
  done
  local output
  output="$(remote_exec "export SLAN_CLIENT_CORE_SERVICE_HOST='${REMOTE_SERVICE_HOST}'; bash '${REMOTE_HELPER_PATH}' ${command}${quoted}")"
  local json_line
  json_line="$(printf '%s\n' "$output" | tr -d '\r' | awk '/\{.*\}/ { line = $0 } END { print line }')"
  if [[ -n "$json_line" ]]; then
    printf '%s\n' "$json_line"
    return 0
  fi
  printf '%s\n' "$output"
}

remote_prepare_dependencies() {
  if ! is_truthy "$PREPARE_REMOTE_DEPENDENCIES"; then
    remote_exec "uname -m" >"$REMOTE_PREP_LOG"
    REMOTE_ARCH="$(tr -d '\r' <"$REMOTE_PREP_LOG" | grep -E '^(x86_64|amd64|aarch64|arm64|armv7l|armhf)$' | tail -n 1)"
    [[ -n "$REMOTE_ARCH" ]] || fail "failed to detect remote Linux architecture"
    return 0
  fi
  remote_exec "
set -euo pipefail
mkdir -p '${REMOTE_DIR}'
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update >/dev/null
  apt-get install -y bash curl jq netcat-openbsd python3 iproute2 iputils-ping dnsutils >/dev/null
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y bash curl jq nmap-ncat python3 iproute iputils bind-utils >/dev/null
elif command -v yum >/dev/null 2>&1; then
  yum install -y bash curl jq nc python3 iproute iputils bind-utils >/dev/null
else
  echo 'unsupported remote package manager' >&2
  exit 1
fi
uname -m
" >"$REMOTE_PREP_LOG"
  REMOTE_ARCH="$(
    tr -d '\r' <"$REMOTE_PREP_LOG" | grep -E '^(x86_64|amd64|aarch64|arm64|armv7l|armhf)$' | tail -n 1
  )"
  [[ -n "$REMOTE_ARCH" ]] || fail "failed to detect remote Linux architecture"
}

normalize_arch() {
  case "$1" in
    x86_64|amd64) printf 'amd64\n' ;;
    aarch64|arm64) printf 'arm64\n' ;;
    armv7l|armhf) printf 'armhf\n' ;;
    *) printf '%s\n' "$1" ;;
  esac
}

resolve_linux_package_path() {
  local normalized_arch="$1"
  if [[ -n "${SLAN_REMOTE_LINUX_PACKAGE:-}" ]]; then
    printf '%s\n' "$SLAN_REMOTE_LINUX_PACKAGE"
    return
  fi
  local installer_dir="$ROOT_DIR/client_v2/.tmp/installer/linux"
  local candidate="$installer_dir/SLAN-Client-V2-linux-${normalized_arch}.tar.gz"
  if [[ -f "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return
  fi
  if [[ "$normalized_arch" == "amd64" ]] && is_truthy "$BUILD_LINUX_PACKAGE_REMOTE"; then
    bash "$ROOT_DIR/scripts/build_linux_client_remote.sh"
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  fi
  if is_truthy "$BUILD_LINUX_PACKAGE"; then
    bash "$ROOT_DIR/scripts/build_linux_client_docker.sh"
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  fi
  fail "Linux package not found for arch=${normalized_arch}: $candidate"
}

resolve_remote_built_package_path() {
  local normalized_arch="$1"
  [[ "$normalized_arch" == "amd64" ]] || fail \
    "remote direct build/install currently supports amd64 only, got arch=${normalized_arch}"
  SLAN_REMOTE_LINUX_HOST="$REMOTE_HOST" \
    SLAN_REMOTE_LINUX_USER="$REMOTE_USER" \
    SLAN_REMOTE_LINUX_PASSWORD="$REMOTE_PASSWORD" \
    SLAN_REMOTE_LINUX_SSH_KEY="$REMOTE_SSH_KEY" \
    SLAN_REMOTE_LINUX_RUST_PROFILE="${SLAN_REMOTE_LINUX_RUST_PROFILE:-debug}" \
    SLAN_REMOTE_LINUX_SKIP_FETCH=1 \
    bash "$ROOT_DIR/scripts/build_linux_client_remote.sh" >/dev/null
  printf '%s\n' "/tmp/slan-linux-remote-build/client_v2/.tmp/installer/linux/SLAN-Client-V2-linux-amd64.tar.gz"
}

register_and_login_user() {
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

refresh_user_token() {
  local auth_json
  auth_json="$(curl --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
  USER_TOKEN="$(printf '%s' "$auth_json" | jq -r '.accessToken // .token // .auth.accessToken // .auth.session.token // empty')"
  [[ -n "$USER_TOKEN" ]] || fail "failed to refresh test user token"
}

resolve_network_context() {
  local networks_json groups_json
  networks_json="$(curl --silent --show-error --fail \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    "${WEB_BASE_URL}/api/web/networks?userId=${USER_ID}")"
  NETWORK_ID="$(printf '%s' "$networks_json" | jq -r '.items[0].networkId // .[0].networkId // empty')"
  [[ -n "$NETWORK_ID" ]] || fail "failed to resolve default network"
  groups_json="$(curl --silent --show-error --fail \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/security-groups")"
  SECURITY_GROUP_ID="$(printf '%s' "$groups_json" | jq -r '.items[0].securityGroupId // .[0].securityGroupId // empty')"
  [[ -n "$SECURITY_GROUP_ID" ]] || fail "network ${NETWORK_ID} has no security group"
}

create_bootstrap_key() {
  local bootstrap_json
  bootstrap_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_BASE_URL}/api/web/device-bootstrap-keys" \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"${USER_ID}\",\"networkId\":\"${NETWORK_ID}\",\"deviceAlias\":\"${LINUX_DEVICE_ALIAS}\",\"ttlSeconds\":${BOOTSTRAP_TTL_SECONDS}}")"
  BOOTSTRAP_KEY_ID="$(printf '%s' "$bootstrap_json" | jq -r '.id // .installationKeyId // empty')"
  BOOTSTRAP_KEY="$(printf '%s' "$bootstrap_json" | jq -r '.key // .installationKey // empty')"
  [[ -n "$BOOTSTRAP_KEY_ID" && -n "$BOOTSTRAP_KEY" ]] || fail "failed to create remote Linux bootstrap key"
}

create_json() {
  local url="$1"
  local payload="$2"
  local request_url="$url"
  local response_file
  local status
  response_file="$(mktemp "${TMPDIR:-/tmp}/slan-create-json.XXXXXX")"
  if [[ "$request_url" == *\?* ]]; then
    request_url="${request_url}&actorUserId=${USER_ID}"
  else
    request_url="${request_url}?actorUserId=${USER_ID}"
  fi
  status="$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
    -o "$response_file" \
    -w '%{http_code}' \
    -X POST "$request_url" \
    -H "Authorization: Bearer ${USER_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$payload")"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "create_json failed status=$status url=$request_url payload=$payload body=$(cat "$response_file")" >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

create_dns_zone() {
  local zone_json
  ZONE_NAME="mac-linux-$(date +%s).slan.test"
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

resolve_record_from_remote_module() {
  local fqdn="$1"
  local module_json
  module_json="$(remote_helper request_json localNetworkModule)"
  jq -r --arg fqdn "$fqdn" '
    .configs[]? as $config
    | $config.resolverRecords[]?
    | select(
        ((.fqdn // "" | ascii_downcase) == ($fqdn | ascii_downcase)) or
        ((.name // "" | ascii_downcase) == ($fqdn | ascii_downcase))
      )
    | if (.targetIp // "") != "" then
        .targetIp
      else
        (.targetDeviceId // "") as $targetDeviceId
        | (
            $config.peers[]?
            | select((.deviceId // "") == $targetDeviceId)
            | if ((.virtualIps // []) | length) > 0 then
                .virtualIps[0]
              else
                .globalIp // empty
              end
          )
      end
  ' <<<"$module_json" | head -n 1
}

add_rule() {
  local direction="$1"
  local protocol="$2"
  local port="$3"
  local peer_value="$4"
  local priority="$5"
  local rule_json rule_id
  rule_json="$(create_json "${WEB_BASE_URL}/api/web/security-groups/${SECURITY_GROUP_ID}/rules" \
    "{\"direction\":\"${direction}\",\"priority\":${priority},\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"device_group\",\"peerValue\":\"${peer_value}\",\"enabled\":true}")"
  rule_id="$(printf '%s' "$rule_json" | jq -r '.ruleId // empty')"
  [[ -n "$rule_id" ]] || fail "failed to create ${protocol}:${port} ${direction} rule for ${peer_value}"
  RULE_IDS+=("$rule_id")
}

provision_network_device_group() {
  local group_json
  group_json="$(create_json "${WEB_BASE_URL}/api/web/users/${USER_ID}/device-groups" \
    "{\"name\":\"mac-linux-$(date +%s%N)\",\"description\":\"Mac Linux integration devices\"}")"
  DEVICE_GROUP_ID="$(printf '%s' "$group_json" | jq -r '.groupId // empty')"
  [[ -n "$DEVICE_GROUP_ID" ]] || fail "device group create returned empty groupId"

  local device_id
  for device_id in "$MAC_DEVICE_ID" "$LINUX_DEVICE_ID"; do
    curl --silent --show-error --fail \
      -X PUT "${WEB_BASE_URL}/api/web/users/${USER_ID}/devices/${device_id}/groups" \
      -H "Authorization: Bearer ${USER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"groupIds\":[\"${DEVICE_GROUP_ID}\"]}" >/dev/null
  done
  log "prepared device group assignments group=${DEVICE_GROUP_ID}"
}

attach_device_group_to_network() {
  create_json "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/device-groups" \
    "{\"groupId\":\"${DEVICE_GROUP_ID}\"}" >/dev/null
  log "attached device group network=${NETWORK_ID} group=${DEVICE_GROUP_ID}"
}

json_field() {
  local json="$1"
  local filter="$2"
  local payload
  payload="$(printf '%s\n' "$json" | tr -d '\r' | awk '/\{.*\}/ { line = $0 } END { print line }')"
  [[ -n "$payload" ]] || fail "failed to locate JSON payload in output: $json"
  jq -r "$filter" <<<"$payload"
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$REMOTE_PREP_LOG" ]] && { echo "---- Remote prep log ----" >&2; cat "$REMOTE_PREP_LOG" >&2; }
    [[ -f "$MAC_SERVICE_LOG" ]] && { echo "---- Mac service log ----" >&2; cat "$MAC_SERVICE_LOG" >&2; }
    [[ -f "$MAC_CORE_LOG" ]] && { echo "---- Mac core log ----" >&2; cat "$MAC_CORE_LOG" >&2; }
    [[ -f "$MAC_ECHO_LOG" ]] && { echo "---- Mac echo log ----" >&2; cat "$MAC_ECHO_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  if [[ -n "$NETWORK_ID" ]]; then
    local index
    for ((index=${#RULE_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${WEB_BASE_URL}/api/web/security-groups/rules/${RULE_IDS[$index]}"
    done
    for ((index=${#RECORD_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/records/${RECORD_IDS[$index]}"
    done
    if [[ -n "$ZONE_ID" ]]; then
      best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/dns/zones/${ZONE_ID}"
    fi
    if [[ -n "$DEVICE_GROUP_ID" ]]; then
      best_effort_delete "${WEB_BASE_URL}/api/web/networks/${NETWORK_ID}/device-groups/${DEVICE_GROUP_ID}"
    fi
  fi
  if [[ -n "$DEVICE_GROUP_ID" && -n "$USER_ID" ]]; then
    best_effort_delete "${WEB_BASE_URL}/api/web/users/${USER_ID}/device-groups/${DEVICE_GROUP_ID}"
  fi
  slan_cleanup_remote_test_devices "$BIZ_URL" "$EMAIL" "$PASSWORD" "$CLEANUP_TEST_DEVICES"
  if [[ "${SLAN_KEEP_MAC_REMOTE_LINUX_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

main() {
  need curl
  need jq
  need python3
  need expect
  need /opt/homebrew/bin/go
  [[ -n "$REMOTE_HOST" && -n "$REMOTE_USER" ]] || fail "set remote Linux host/user"

  if is_truthy "$RUN_REMOTE_INSTALL_CHECK"; then
    log "run remote Linux install preflight"
    SLAN_REMOTE_LINUX_HOST="$REMOTE_HOST" \
      SLAN_REMOTE_LINUX_USER="$REMOTE_USER" \
      SLAN_REMOTE_LINUX_PASSWORD="$REMOTE_PASSWORD" \
      SLAN_REMOTE_LINUX_SSH_KEY="$REMOTE_SSH_KEY" \
      SLAN_REMOTE_LINUX_WORK_DIR="$REMOTE_DIR" \
      SLAN_BIZ_URL="$BIZ_URL" \
      bash "$ROOT_DIR/scripts/tests/linux/linux_remote_install_check.sh"
  fi

  register_and_login_user
  resolve_network_context
  create_bootstrap_key
  remote_prepare_dependencies

  local normalized_remote_arch
  normalized_remote_arch="$(normalize_arch "$REMOTE_ARCH")"
  remote_expect_scp "$ROOT_DIR/scripts/tests/linux/remote_linux_local_api.sh" "$REMOTE_HELPER_PATH"
  if is_truthy "$USE_REMOTE_BUILT_PACKAGE_DIRECTLY"; then
    REMOTE_PACKAGE_PATH="$(resolve_remote_built_package_path "$normalized_remote_arch")"
  else
    PACKAGE_PATH="$(resolve_linux_package_path "$normalized_remote_arch")"
    REMOTE_PACKAGE_PATH="$REMOTE_DIR/$(basename "$PACKAGE_PATH")"
    remote_expect_scp "$PACKAGE_PATH" "$REMOTE_PACKAGE_PATH"
  fi

  log "install remote Linux client package"
  remote_exec "
set -euo pipefail
mkdir -p '${REMOTE_DIR}'
systemctl stop slan-client-v2.service >/dev/null 2>&1 || true
rm -f /var/lib/SLAN/config.json
rm -f /var/lib/SLAN/client-v2-session.json
rm -f /var/lib/SLAN/client-v2-device-id.txt
rm -f /var/lib/SLAN/client-v2-device-public-key.txt
rm -f /var/lib/SLAN/client-v2-control-tasks.xml
rm -f /etc/slan/client-v2-console.env
curl -fsSL '${BIZ_URL}/downloads/clients/install.sh' -o '${REMOTE_DIR}/install.sh'
bash '${REMOTE_DIR}/install.sh' \
  --server='${BIZ_URL}' \
  --installation-key='${BOOTSTRAP_KEY}' \
  --tray=disabled \
  --package-url='file://${REMOTE_PACKAGE_PATH}'
"

  log "start or verify mac local service"
  start_mac_service_if_needed
  log "login and enable Mac network"
  login_and_enable_mac

  log "wait remote Linux local API"
  remote_helper wait_local_api "$LOCAL_API_TIMEOUT_SECONDS" >/dev/null
  log "sign in remote Linux local service"
  local remote_signed_in_json
  remote_signed_in_json="$(remote_helper wait_signed_in_or_login "$EMAIL" "$PASSWORD" "$TIMEOUT_SECONDS")"
  LINUX_DEVICE_ID="$(json_field "$remote_signed_in_json" '.deviceId // empty')"
  [[ -n "$LINUX_DEVICE_ID" ]] || fail "failed to parse remote Linux device id"
  log "wait remote Linux control transport"
  remote_helper wait_control_ready "$TIMEOUT_SECONDS" >/dev/null

  refresh_user_token
  provision_network_device_group
  create_dns_zone
  create_dns_record mac "$MAC_DEVICE_ID"
  create_dns_record linux "$LINUX_DEVICE_ID"
  attach_device_group_to_network
  add_rule ingress tcp 443 "$DEVICE_GROUP_ID" 100
  add_rule egress tcp 443 "$DEVICE_GROUP_ID" 110
  add_rule ingress udp "$UDP_PORT" "$DEVICE_GROUP_ID" 120
  add_rule egress udp "$UDP_PORT" "$DEVICE_GROUP_ID" 130
  add_rule ingress tcp "$TCP_PORT" "$DEVICE_GROUP_ID" 140
  add_rule egress tcp "$TCP_PORT" "$DEVICE_GROUP_ID" 150

  log "restart remote Linux client and reload the complete network snapshot"
  remote_exec "systemctl restart slan-client-v2.service"
  remote_helper wait_local_api "$LOCAL_API_TIMEOUT_SECONDS" >/dev/null
  remote_helper wait_signed_in_or_login "$EMAIL" "$PASSWORD" "$TIMEOUT_SECONDS" >/dev/null
  remote_helper wait_control_ready "$TIMEOUT_SECONDS" >/dev/null

  log "wait remote Linux network module receive dns/acl config"
  remote_helper wait_network_module 1 2 4 "$TIMEOUT_SECONDS" >/dev/null

  log "enable remote Linux network"
  local remote_network_json
  remote_network_json="$(remote_helper ensure_network_ready "$TIMEOUT_SECONDS")"
  LINUX_IP="$(json_field "$remote_network_json" '.virtualIp // empty')"
  LINUX_IP="${LINUX_IP%%/*}"
  [[ -n "$LINUX_IP" ]] || fail "failed to parse remote Linux virtual IP"
  log "resolved network identities macDeviceId=$MAC_DEVICE_ID macIp=$MAC_IP linuxDeviceId=$LINUX_DEVICE_ID linuxIp=$LINUX_IP"

  local linux_target_ip mac_target_ip
  linux_target_ip="$(resolve_record_from_mac_module "linux.${ZONE_NAME}" || true)"
  if [[ -z "$linux_target_ip" ]]; then
    linux_target_ip="$LINUX_IP"
  fi
  mac_target_ip="$(resolve_record_from_remote_module "mac.${ZONE_NAME}" || true)"
  if [[ -z "$mac_target_ip" ]]; then
    mac_target_ip="$MAC_IP"
  fi
  log "wait macOS route/data-path ready for Linux peer target=${linux_target_ip} source=${MAC_IP}"
  slan_wait_macos_peer_route_ready "$linux_target_ip" "$MAC_IP" "$MAC_SERVICE_HOST" 120 || \
    fail "macOS route/data-path did not become ready for Linux peer ${linux_target_ip}"

  log "verify bidirectional client_message"
  remote_helper send_client_message "$MAC_DEVICE_ID" "$LINUX_TO_MAC_BODY" >/dev/null
  wait_mac_message "$LINUX_DEVICE_ID" "$LINUX_TO_MAC_BODY"
  send_mac_message "$LINUX_DEVICE_ID" "$MAC_TO_LINUX_BODY"
  remote_helper wait_client_message "$MAC_DEVICE_ID" "$MAC_TO_LINUX_BODY" "$TIMEOUT_SECONDS" >/dev/null

  log "start remote Linux echo server"
  remote_helper start_echo_server "$UDP_PORT" "$TCP_PORT" "${REMOTE_DIR}/remote-echo.log" >/dev/null
  remote_helper wait_echo_ready "$UDP_PORT" "$TCP_PORT" "${REMOTE_DIR}/remote-echo.log" 30 >/dev/null

  log "run macOS -> remote Linux UDP/TCP socket checks"
  send_mac_udp "$MAC_IP" "$linux_target_ip" "$MAC_UDP_BODY"
  send_mac_tcp "$MAC_IP" "$linux_target_ip" "$MAC_TCP_BODY"

  log "start macOS echo server for reverse checks"
  start_mac_echo_server
  wait_mac_echo_ready

  log "run remote Linux -> macOS UDP/TCP socket checks"
  remote_helper send_udp_echo "$mac_target_ip" "$UDP_PORT" "$LINUX_UDP_BODY" >/dev/null
  remote_helper send_tcp_echo "$mac_target_ip" "$TCP_PORT" "$LINUX_TCP_BODY" >/dev/null

  echo "macRemoteLinuxIntegration: ok email=$EMAIL mac=$MAC_DEVICE_ID linux=$LINUX_DEVICE_ID macIp=$MAC_IP linuxIp=$LINUX_IP zone=$ZONE_NAME remote=$REMOTE_HOST"
}

main "$@"
