#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/lib/flutter_mobile_activation_test.sh"
source "$ROOT_DIR/scripts/test_cleanup_lib.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"

APP_DIR="$ROOT_DIR/client/app_flutter"
ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
if [[ -n "${SLAN_ANDROID_BIZ_URL:-}" ]]; then
  ANDROID_BIZ_URL="$SLAN_ANDROID_BIZ_URL"
elif [[ "$BIZ_URL" == "http://127.0.0.1:28080" || "$BIZ_URL" == "http://localhost:28080" ]]; then
  ANDROID_BIZ_URL="http://10.0.2.2:28080"
else
  ANDROID_BIZ_URL="$BIZ_URL"
fi
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
OPS_EMAIL="${SLAN_OPS_EMAIL:-admin1}"
OPS_PASSWORD="${SLAN_OPS_PASSWORD:-admin1}"
REMOTE_HOST="${SLAN_REMOTE_LINUX_HOST:-100.87.66.24}"
REMOTE_USER="${SLAN_REMOTE_LINUX_USER:-root}"
REMOTE_PASSWORD="${SLAN_REMOTE_LINUX_PASSWORD:-}"
REMOTE_SSH_KEY="${SLAN_REMOTE_LINUX_SSH_KEY:-}"
REMOTE_DIR="${SLAN_REMOTE_LINUX_WORK_DIR:-/tmp/slan-android-remote-linux}"
REMOTE_HELPER_PATH="$REMOTE_DIR/remote_linux_local_api.sh"
REMOTE_PACKAGE_PATH="$REMOTE_DIR/$(basename "${SLAN_REMOTE_LINUX_PACKAGE:-package.tar.gz}")"
REMOTE_SERVICE_HOST="${SLAN_REMOTE_LINUX_SERVICE_HOST:-127.0.0.1:46392}"
RUN_REMOTE_INSTALL_CHECK="${SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK:-1}"
TIMEOUT_SECONDS="${SLAN_ANDROID_REMOTE_LINUX_TIMEOUT_SECONDS:-180}"
LOCAL_API_TIMEOUT_SECONDS="${SLAN_ANDROID_REMOTE_LINUX_LOCAL_API_TIMEOUT_SECONDS:-120}"
ANDROID_DEVICE="${SLAN_ANDROID_FLUTTER_DEVICE:-emulator-5554}"
ADB_TARGET=("$ADB" -s "$ANDROID_DEVICE")
ANDROID_TEST_DEVICE_ID="${SLAN_ANDROID_TEST_DEVICE_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"
LINUX_DEVICE_ALIAS="${SLAN_REMOTE_LINUX_DEVICE_ALIAS:-Remote Linux CLI}"
BOOTSTRAP_TTL_SECONDS="${SLAN_REMOTE_LINUX_BOOTSTRAP_TTL_SECONDS:-1800}"
BUILD_LINUX_PACKAGE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE:-0}"
BUILD_LINUX_PACKAGE_REMOTE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE_REMOTE:-1}"
PREPARE_REMOTE_DEPENDENCIES="${SLAN_REMOTE_LINUX_PREPARE_DEPENDENCIES:-1}"
USE_REMOTE_BUILT_PACKAGE_DIRECTLY="${SLAN_REMOTE_LINUX_USE_REMOTE_BUILT_PACKAGE_DIRECTLY:-0}"
LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-0}"
if [[ -n "${SLAN_TEST_UDP_ECHO_PORT:-}" ]]; then
  UDP_PORT="${SLAN_TEST_UDP_ECHO_PORT}"
else
  UDP_PORT="$((20000 + (RANDOM % 10000) * 2))"
fi
if [[ -n "${SLAN_TEST_TCP_ECHO_PORT:-}" ]]; then
  TCP_PORT="${SLAN_TEST_TCP_ECHO_PORT}"
else
  TCP_PORT="$((UDP_PORT + 1))"
fi
NETWORK_MODULE_RULES_MIN="${SLAN_ANDROID_REMOTE_LINUX_MIN_SECURITY_RULES:-4}"
ANDROID_DEVICE_ID_WAIT_SECONDS="${SLAN_ANDROID_DEVICE_ID_WAIT_SECONDS:-180}"
ANDROID_ECHO_HOLD_SECONDS="${SLAN_ANDROID_ECHO_HOLD_SECONDS:-75}"
ANDROID_ECHO_STARTUP_TIMEOUT_SECONDS="${SLAN_ANDROID_ECHO_STARTUP_TIMEOUT_SECONDS:-180}"
ANDROID_POST_ENABLE_WAIT_SECONDS="${SLAN_ANDROID_POST_ENABLE_WAIT_SECONDS:-8}"
ANDROID_TO_LINUX_BODY="${SLAN_ANDROID_TO_LINUX_BODY:-hello-android-to-linux-$(date +%s%N)}"
LINUX_TO_ANDROID_BODY="${SLAN_LINUX_TO_ANDROID_BODY:-hello-linux-to-android-$(date +%s%N)}"
ANDROID_UDP_BODY="${SLAN_ANDROID_TO_LINUX_UDP_BODY:-android-to-linux-udp-$(date +%s%N)}"
ANDROID_TCP_BODY="${SLAN_ANDROID_TO_LINUX_TCP_BODY:-android-to-linux-tcp-$(date +%s%N)}"
LINUX_UDP_BODY="${SLAN_LINUX_TO_ANDROID_UDP_BODY:-linux-to-android-udp-$(date +%s%N)}"
LINUX_TCP_BODY="${SLAN_LINUX_TO_ANDROID_TCP_BODY:-linux-to-android-tcp-$(date +%s%N)}"

WORK_DIR="${SLAN_ANDROID_REMOTE_LINUX_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/slan-android-remote-linux.XXXXXX")}"
ANDROID_LOG="$WORK_DIR/android-flutter-test.log"
ANDROID_ECHO_LOG="$WORK_DIR/android-echo-test.log"
REMOTE_PREP_LOG="$WORK_DIR/remote-prepare.log"

NETWORK_ID=""
SECURITY_GROUP_ID=""
BOOTSTRAP_KEY_ID=""
BOOTSTRAP_KEY=""
ANDROID_CREDENTIAL_ID=""
ANDROID_AUTHORIZATION_KEY=""
OPS_TOKEN=""
DEVICE_GROUP_ID=""
ZONE_ID=""
ZONE_NAME=""
RULE_IDS=()
RECORD_IDS=()
REMOTE_ARCH=""
PACKAGE_PATH=""
LINUX_DEVICE_ID=""
LINUX_IP=""
ANDROID_DEVICE_ID=""

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

json_field() {
  local json="$1"
  local filter="$2"
  local payload
  payload="$(printf '%s\n' "$json" | tr -d '\r' | awk '/\{.*\}/ { line = $0 } END { print line }')"
  [[ -n "$payload" ]] || fail "failed to locate JSON payload in output: $json"
  jq -r "$filter" <<<"$payload"
}

best_effort_delete() {
  local url="$1"
  curl --silent --show-error --connect-timeout 5 --max-time 20 \
    -X DELETE "$url" \
    -H "Authorization: Bearer ${OPS_TOKEN}" >/dev/null 2>&1 || true
}

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    [[ -f "$ANDROID_LOG" ]] && { echo "---- Android message log ----" >&2; cat "$ANDROID_LOG" >&2; }
    [[ -f "$ANDROID_ECHO_LOG" ]] && { echo "---- Android echo log ----" >&2; cat "$ANDROID_ECHO_LOG" >&2; }
    [[ -f "$WORK_DIR/android-linux-socket.log" ]] && {
      echo "---- Android Linux socket log ----" >&2
      cat "$WORK_DIR/android-linux-socket.log" >&2
    }
    [[ -f "$REMOTE_PREP_LOG" ]] && { echo "---- Remote prep log ----" >&2; cat "$REMOTE_PREP_LOG" >&2; }
  fi
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$BOOTSTRAP_KEY_ID" >/dev/null 2>&1 || true
  slan_ops_revoke_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$ANDROID_CREDENTIAL_ID" >/dev/null 2>&1 || true
  if [[ -n "$NETWORK_ID" ]]; then
    local index
    for ((index=${#RULE_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${OPS_BASE_URL}/api/ops/security-rules/${RULE_IDS[$index]}"
    done
    for ((index=${#RECORD_IDS[@]}-1; index>=0; index--)); do
      best_effort_delete "${OPS_BASE_URL}/api/ops/dns/records/${RECORD_IDS[$index]}"
    done
    if [[ -n "$ZONE_ID" ]]; then
      best_effort_delete "${OPS_BASE_URL}/api/ops/dns/zones/${ZONE_ID}"
    fi
    if [[ -n "$DEVICE_GROUP_ID" ]]; then
      best_effort_delete "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups/${DEVICE_GROUP_ID}"
    fi
    slan_ops_delete_network "$OPS_BASE_URL" "$OPS_TOKEN" "$NETWORK_ID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$DEVICE_GROUP_ID" ]]; then
    best_effort_delete "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}"
  fi
  if [[ "${SLAN_KEEP_ANDROID_REMOTE_LINUX_WORK_DIR:-0}" != "1" ]]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

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
  local installer_dir="$ROOT_DIR/client/.tmp/installer/linux"
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
  printf '%s\n' "/tmp/slan-linux-remote-build/client/.tmp/installer/linux/SLAN-Client-V2-linux-amd64.tar.gz"
}

create_device_authorization_key() {
  local bootstrap_json android_json
  OPS_TOKEN="$(slan_ops_login "$OPS_BASE_URL" "$OPS_EMAIL" "$OPS_PASSWORD")"
  bootstrap_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "$LINUX_DEVICE_ALIAS")"
  BOOTSTRAP_KEY_ID="$(printf '%s' "$bootstrap_json" | jq -r '.credentialId // empty')"
  BOOTSTRAP_KEY="$(printf '%s' "$bootstrap_json" | jq -r '.key // empty')"
  [[ -n "$BOOTSTRAP_KEY_ID" && -n "$BOOTSTRAP_KEY" ]] || fail "failed to create remote Linux device authorization key"
  android_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$OPS_TOKEN" "Android Remote Linux Android" "$ANDROID_TEST_DEVICE_ID")"
  ANDROID_CREDENTIAL_ID="$(printf '%s' "$android_json" | jq -r '.credentialId // empty')"
  ANDROID_AUTHORIZATION_KEY="$(printf '%s' "$android_json" | jq -r '.key // empty')"
  [[ -n "$ANDROID_CREDENTIAL_ID" && -n "$ANDROID_AUTHORIZATION_KEY" ]] || fail "failed to create Android device authorization key"
  curl --silent --show-error --fail -X POST "${BIZ_URL}/api/device-auth/token" \
    -H 'Content-Type: application/json' \
    -d "{\"key\":\"${ANDROID_AUTHORIZATION_KEY}\",\"deviceId\":\"${ANDROID_TEST_DEVICE_ID}\"}" >/dev/null
}

resolve_network_context() {
  local network_json group_json
  network_json="$(slan_ops_create_network "$OPS_BASE_URL" "$OPS_TOKEN" "android-remote-linux-$(date +%s%N)")"
  NETWORK_ID="$(printf '%s' "$network_json" | jq -r '.networkId // empty')"
  [[ -n "$NETWORK_ID" ]] || fail "failed to create Ops network"
  group_json="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/security-groups" \
    "{\"name\":\"android-remote-linux-security-$(date +%s%N)\"}")"
  SECURITY_GROUP_ID="$(printf '%s' "$group_json" | jq -r '.securityGroupId // empty')"
  [[ -n "$SECURITY_GROUP_ID" ]] || fail "failed to create Ops security group"
}

create_json() {
  local url="$1"
  local payload="$2"
  local response_file
  local status
  response_file="$(mktemp "${TMPDIR:-/tmp}/slan-create-json.XXXXXX")"
  status="$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
    -o "$response_file" \
    -w '%{http_code}' \
    -X POST "$url" \
    -H "Authorization: Bearer ${OPS_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$payload")"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "create_json failed status=$status url=$url payload=$payload body=$(cat "$response_file")" >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

create_dns_zone() {
  local zone_json
  ZONE_NAME="android-linux-$(date +%s).slan.test"
  zone_json="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/dns/zones" \
    "{\"name\":\"${ZONE_NAME}\"}")"
  ZONE_ID="$(printf '%s' "$zone_json" | jq -r '.zoneId // empty')"
  [[ -n "$ZONE_ID" ]] || fail "dns zone create returned empty zoneId"
}

create_dns_record() {
  local name="$1"
  local target_device_id="$2"
  local record_json record_id
  record_json="$(create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/dns/records" \
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
  rule_json="$(create_json "${OPS_BASE_URL}/api/ops/security-groups/${SECURITY_GROUP_ID}/rules" \
    "{\"direction\":\"${direction}\",\"priority\":${priority},\"action\":\"allow\",\"protocol\":\"${protocol}\",\"portFrom\":${port},\"portTo\":${port},\"peerType\":\"device_group\",\"peerValue\":\"${peer_value}\",\"enabled\":true}")"
  rule_id="$(printf '%s' "$rule_json" | jq -r '.ruleId // empty')"
  [[ -n "$rule_id" ]] || fail "failed to create ${protocol}:${port} ${direction} rule for ${peer_value}"
  RULE_IDS+=("$rule_id")
}

provision_network_device_group() {
  local group_json
  group_json="$(create_json "${OPS_BASE_URL}/api/ops/device-groups" \
    "{\"name\":\"android-linux-$(date +%s%N)\",\"description\":\"Android Linux integration devices\"}")"
  DEVICE_GROUP_ID="$(printf '%s' "$group_json" | jq -r '.groupId // empty')"
  [[ -n "$DEVICE_GROUP_ID" ]] || fail "device group create returned empty groupId"

  local device_id
  for device_id in "$ANDROID_DEVICE_ID" "$LINUX_DEVICE_ID"; do
    curl --silent --show-error --fail \
      -X POST "${OPS_BASE_URL}/api/ops/device-groups/${DEVICE_GROUP_ID}/devices" \
      -H "Authorization: Bearer ${OPS_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "{\"deviceId\":\"${device_id}\"}" >/dev/null
  done
  log "prepared device group assignments group=${DEVICE_GROUP_ID}"
}

attach_device_group_to_network() {
  create_json "${OPS_BASE_URL}/api/ops/networks/${NETWORK_ID}/device-groups" \
    "{\"groupId\":\"${DEVICE_GROUP_ID}\"}" >/dev/null
  log "attached device group network=${NETWORK_ID} group=${DEVICE_GROUP_ID}"
}

provision_dns_resources() {
  create_dns_zone
  create_dns_record linux "$LINUX_DEVICE_ID"
  create_dns_record android "$ANDROID_DEVICE_ID"
}

provision_acl_resources() {
  add_rule ingress udp "$UDP_PORT" "$DEVICE_GROUP_ID" 100
  add_rule egress udp "$UDP_PORT" "$DEVICE_GROUP_ID" 110
  add_rule ingress tcp "$TCP_PORT" "$DEVICE_GROUP_ID" 120
  add_rule egress tcp "$TCP_PORT" "$DEVICE_GROUP_ID" 130
}

start_android_vpn_appops_guard() {
  (
    while true; do
      "${ADB_TARGET[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
      sleep 0.25
    done
  ) &
  PIDS+=("$!")
}

tap_bounds_center() {
  local bounds="$1"
  [[ "$bounds" =~ \[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\] ]] || return 1
  local x1="${BASH_REMATCH[1]}"
  local y1="${BASH_REMATCH[2]}"
  local x2="${BASH_REMATCH[3]}"
  local y2="${BASH_REMATCH[4]}"
  local x=$(( (x1 + x2) / 2 ))
  local y=$(( (y1 + y2) / 2 ))
  "${ADB_TARGET[@]}" shell input tap "$x" "$y" >/dev/null 2>&1 || true
}

start_android_vpn_consent_guard() {
  (
    while true; do
      xml="$("${ADB_TARGET[@]}" shell uiautomator dump /sdcard/slan-ui.xml >/dev/null 2>&1 && "${ADB_TARGET[@]}" shell cat /sdcard/slan-ui.xml 2>/dev/null || true)"
      if [[ "$xml" == *"package=\"com.android.vpndialogs\""* || "$xml" == *"VPN"* || "$xml" == *"连接请求"* || "$xml" == *"网络请求"* ]]; then
        bounds="$(
          printf '%s' "$xml" | tr '>' '\n' | grep -E \
            'resource-id="android:id/button1"|resource-id="com.android.vpndialogs:id/button1"|text="(确定|OK|继续|允许|始终允许|同意|接受|Connect|Allow)"' \
            | sed -n 's/.*bounds="\([^"]*\)".*/\1/p' | head -n 1
        )"
        if [[ -n "$bounds" ]]; then
          tap_bounds_center "$bounds"
        else
          "${ADB_TARGET[@]}" shell input keyevent 66 >/dev/null 2>&1 || true
        fi
      fi
      sleep 0.5
    done
  ) &
  PIDS+=("$!")
}

wait_android_boot() {
  "${ADB_TARGET[@]}" wait-for-device
  local boot_completed
  boot_completed="$("${ADB_TARGET[@]}" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
  if [[ "$boot_completed" != "1" ]]; then
    for _ in $(seq 1 60); do
      boot_completed="$("${ADB_TARGET[@]}" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
      [[ "$boot_completed" == "1" ]] && break
      sleep 1
    done
  fi
  [[ "$boot_completed" == "1" ]] || fail "Android device did not finish booting"
}

start_android_message_harness() {
  local common_defines=()
  local send_defines=()
  local expect_defines=()
  while IFS= read -r line; do
    common_defines+=("$line")
  done < <(slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true)
  while IFS= read -r line; do
    send_defines+=("$line")
  done < <(slan_mobile_message_send_defines "$LINUX_DEVICE_ID" "$ANDROID_TO_LINUX_BODY")
  while IFS= read -r line; do
    expect_defines+=("$line")
  done < <(slan_mobile_message_expect_defines "$LINUX_DEVICE_ID" "$LINUX_TO_ANDROID_BODY" 20 90)
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$ANDROID_DEVICE" \
      --timeout 12m \
      "${common_defines[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
      --dart-define="SLAN_TEST_EXPECT_NETWORK_MODULE=true" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_PEERS=1" \
      --dart-define="SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES=$NETWORK_MODULE_RULES_MIN" \
      "${send_defines[@]}" \
      "${expect_defines[@]}"
  ) >"$ANDROID_LOG" 2>&1 &
  ANDROID_PID="$!"
  PIDS+=("$ANDROID_PID")
}

capture_android_device_id_or_die() {
  ANDROID_DEVICE_ID=""
  for _ in $(seq 1 "$ANDROID_DEVICE_ID_WAIT_SECONDS"); do
    if ! kill -0 "$ANDROID_PID" 2>/dev/null; then
      cat "$ANDROID_LOG"
      fail "Android integration test exited before device id was reported"
    fi
    ANDROID_DEVICE_ID="$(sed -n 's/.*SLAN_TEST_CLIENT_DEVICE_ID=\([^[:space:]]*\).*/\1/p' "$ANDROID_LOG" | tail -n 1)"
    if [[ -n "$ANDROID_DEVICE_ID" ]]; then
      [[ "$ANDROID_DEVICE_ID" == "$ANDROID_TEST_DEVICE_ID" ]] \
        || fail "Android device id mismatch: expected=$ANDROID_TEST_DEVICE_ID actual=$ANDROID_DEVICE_ID"
      echo "Android device id: $ANDROID_DEVICE_ID"
      return 0
    fi
    sleep 1
  done
  cat "$ANDROID_LOG"
  fail "timed out waiting for Android device id marker"
}

run_android_socket_client_harness() {
  local target_host="$1"
  local udp_body="$2"
  local tcp_body="$3"
  local log_file="$4"
  local common_defines=()
  while IFS= read -r line; do
    common_defines+=("$line")
  done < <(slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true)
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$ANDROID_DEVICE" \
      --timeout 12m \
      "${common_defines[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
      --dart-define="SLAN_TEST_UDP_SEND_TARGET=${target_host}:${UDP_PORT}" \
      --dart-define="SLAN_TEST_UDP_SEND_BODY=${udp_body}" \
      --dart-define="SLAN_TEST_TCP_SEND_TARGET=${target_host}:${TCP_PORT}" \
      --dart-define="SLAN_TEST_TCP_SEND_BODY=${tcp_body}"
  ) >"$log_file" 2>&1
}

start_android_echo_harness() {
  local common_defines=()
  while IFS= read -r line; do
    common_defines+=("$line")
  done < <(slan_mobile_activation_common_defines "$ANDROID_BIZ_URL" "$ANDROID_AUTHORIZATION_KEY" true)
  (
    cd "$APP_DIR"
    flutter test integration_test/device_activation_harness_test.dart \
      -d "$ANDROID_DEVICE" \
      --timeout 12m \
      "${common_defines[@]}" \
      --dart-define="SLAN_TEST_DEVICE_ID=$ANDROID_TEST_DEVICE_ID" \
      --dart-define="SLAN_TEST_CHECK_SWITCH=true" \
      --dart-define="SLAN_TEST_POST_ENABLE_WAIT_SECONDS=$ANDROID_POST_ENABLE_WAIT_SECONDS" \
      --dart-define="SLAN_TEST_UDP_ECHO_PORT=${UDP_PORT}" \
      --dart-define="SLAN_TEST_TCP_ECHO_PORT=${TCP_PORT}" \
      --dart-define="SLAN_TEST_HOLD_SECONDS=${ANDROID_ECHO_HOLD_SECONDS}"
  ) >"$ANDROID_ECHO_LOG" 2>&1 &
  ANDROID_ECHO_PID="$!"
  PIDS+=("$ANDROID_ECHO_PID")
}

wait_android_log_marker() {
  local pid="$1"
  local log_file="$2"
  local marker="$3"
  local timeout_seconds="$4"
  for _ in $(seq 1 "$timeout_seconds"); do
    if ! kill -0 "$pid" 2>/dev/null; then
      cat "$log_file"
      fail "process exited before marker ${marker}"
    fi
    if grep -q "$marker" "$log_file" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  cat "$log_file"
  fail "timed out waiting for marker ${marker}"
}

need jq
need curl
need expect
need "$ADB"

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

wait_android_boot
"${ADB_TARGET[@]}" shell pm clear dev.slan.slan_client_v2 >/dev/null 2>&1 || true
"${ADB_TARGET[@]}" shell cmd appops set dev.slan.slan_client_v2 ACTIVATE_VPN allow >/dev/null 2>&1 || true
start_android_vpn_appops_guard
start_android_vpn_consent_guard

create_device_authorization_key
resolve_network_context
remote_prepare_dependencies

PACKAGE_PATH=""
REMOTE_PACKAGE_PATH=""
NORMALIZED_REMOTE_ARCH="$(normalize_arch "$REMOTE_ARCH")"
remote_expect_scp "$ROOT_DIR/scripts/tests/linux/remote_linux_local_api.sh" "$REMOTE_HELPER_PATH"
remote_exec "mkdir -p '$REMOTE_DIR/lib'"
remote_expect_scp "$ROOT_DIR/client/install/linux/install.sh" "$REMOTE_DIR/install.sh"
remote_expect_scp "$ROOT_DIR/client/install/linux/lib/slan-linux-install.sh" "$REMOTE_DIR/lib/slan-linux-install.sh"
if is_truthy "$USE_REMOTE_BUILT_PACKAGE_DIRECTLY"; then
  REMOTE_PACKAGE_PATH="$(resolve_remote_built_package_path "$NORMALIZED_REMOTE_ARCH")"
else
  PACKAGE_PATH="$(resolve_linux_package_path "$NORMALIZED_REMOTE_ARCH")"
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
bash '${REMOTE_DIR}/install.sh' \
  --server='${BIZ_URL}' \
  --authorization-key='${BOOTSTRAP_KEY}' \
  --tray=disabled \
  --package-url='file://${REMOTE_PACKAGE_PATH}'
"

log "wait remote Linux local API"
remote_helper wait_local_api "$LOCAL_API_TIMEOUT_SECONDS" >/dev/null
log "sign in remote Linux local service"
remote_activated_json="$(remote_helper wait_activated "$BOOTSTRAP_KEY" "$TIMEOUT_SECONDS")"
LINUX_DEVICE_ID="$(json_field "$remote_activated_json" '.deviceId // empty')"
[[ -n "$LINUX_DEVICE_ID" ]] || fail "failed to parse remote Linux device id"
log "wait remote Linux control transport"
remote_helper wait_control_ready "$TIMEOUT_SECONDS" >/dev/null
log "start Android flutter message harness"
start_android_message_harness
capture_android_device_id_or_die

log "attach Android and Linux through a network device group"
provision_network_device_group

log "provision dns resources"
provision_dns_resources
attach_device_group_to_network
log "provision acl resources"
provision_acl_resources

log "restart remote Linux client and reload the complete network snapshot"
remote_exec "systemctl restart slan-client-v2.service"
remote_helper wait_local_api "$LOCAL_API_TIMEOUT_SECONDS" >/dev/null
remote_helper wait_activated "$BOOTSTRAP_KEY" "$TIMEOUT_SECONDS" >/dev/null
remote_helper wait_control_ready "$TIMEOUT_SECONDS" >/dev/null

log "enable remote Linux network"
remote_network_json="$(remote_helper ensure_network_ready "$TIMEOUT_SECONDS")"
LINUX_IP="$(json_field "$remote_network_json" '.virtualIp // empty')"
LINUX_IP="${LINUX_IP%%/*}"
[[ -n "$LINUX_IP" ]] || fail "failed to parse remote Linux virtual IP"

log "wait remote Linux network module snapshot"
remote_helper wait_network_module 1 2 "$NETWORK_MODULE_RULES_MIN" "$TIMEOUT_SECONDS" >/dev/null

log "wait remote Linux receive Android message"
remote_helper wait_client_message "$ANDROID_DEVICE_ID" "$ANDROID_TO_LINUX_BODY" "$TIMEOUT_SECONDS" >/dev/null
log "send remote Linux reply message"
remote_helper send_client_message "$ANDROID_DEVICE_ID" "$LINUX_TO_ANDROID_BODY" >/dev/null

log "wait Android message harness"
if ! wait "$ANDROID_PID"; then
  cat "$ANDROID_LOG"
  fail "Android message harness failed"
fi

log "start remote Linux echo server"
remote_helper start_echo_server "$UDP_PORT" "$TCP_PORT" "${REMOTE_DIR}/remote-echo.log" >/dev/null
remote_helper wait_echo_ready "$UDP_PORT" "$TCP_PORT" "${REMOTE_DIR}/remote-echo.log" 30 >/dev/null

log "run Android -> remote Linux UDP/TCP socket checks"
run_android_socket_client_harness "linux.${ZONE_NAME}" "$ANDROID_UDP_BODY" "$ANDROID_TCP_BODY" "$WORK_DIR/android-linux-socket.log"
cat "$WORK_DIR/android-linux-socket.log"

log "start Android echo harness for reverse UDP/TCP checks"
start_android_echo_harness
wait_android_log_marker \
  "$ANDROID_ECHO_PID" \
  "$ANDROID_ECHO_LOG" \
  "SLAN_TEST_UDP_ECHO_PORT=${UDP_PORT}" \
  "$ANDROID_ECHO_STARTUP_TIMEOUT_SECONDS"
wait_android_log_marker \
  "$ANDROID_ECHO_PID" \
  "$ANDROID_ECHO_LOG" \
  "SLAN_TEST_TCP_ECHO_PORT=${TCP_PORT}" \
  "$ANDROID_ECHO_STARTUP_TIMEOUT_SECONDS"

log "run remote Linux -> Android UDP/TCP socket checks"
remote_helper send_udp_echo "android.${ZONE_NAME}" "$UDP_PORT" "$LINUX_UDP_BODY" >/dev/null
remote_helper send_tcp_echo "android.${ZONE_NAME}" "$TCP_PORT" "$LINUX_TCP_BODY" >/dev/null

log "wait Android echo harness"
if ! wait "$ANDROID_ECHO_PID"; then
  cat "$ANDROID_ECHO_LOG"
  fail "Android echo harness failed"
fi

echo "androidRemoteLinuxIntegration: ok android=$ANDROID_DEVICE_ID linux=$LINUX_DEVICE_ID linuxIp=$LINUX_IP zone=$ZONE_NAME remote=$REMOTE_HOST"
