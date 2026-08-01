#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/lib/ops_test_network.sh"

REMOTE_HOST="${SLAN_REMOTE_LINUX_HOST:-100.87.66.24}"
REMOTE_USER="${SLAN_REMOTE_LINUX_USER:-root}"
REMOTE_PASSWORD="${SLAN_REMOTE_LINUX_PASSWORD:-}"
REMOTE_SSH_KEY="${SLAN_REMOTE_LINUX_SSH_KEY:-}"
REMOTE_DIR="${SLAN_REMOTE_LINUX_WORK_DIR:-/tmp/slan-linux-remote-install-check}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
WEB_BASE_URL="${SLAN_WEB_BASE_URL:-$SLAN_DEFAULT_WEB_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
TRAY_MODE="${SLAN_LINUX_TRAY_MODE:-disabled}"
BUILD_REMOTE_PACKAGE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE_REMOTE:-1}"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1}"
SERVICE_PORT="${SLAN_CLIENT_CORE_SERVICE_PORT:-46392}"
PASSWORD="${SLAN_TEST_PASSWORD:-Password123!}"
TTL_SECONDS="${SLAN_REMOTE_LINUX_BOOTSTRAP_TTL_SECONDS:-1800}"
DEVICE_ALIAS="${SLAN_REMOTE_LINUX_DEVICE_ALIAS:-Remote Linux Install Check}"
TEST_USER_ID=""
TEST_NETWORK_ID=""

cleanup_test_network() {
  slan_ops_delete_test_network "$OPS_BASE_URL" "$TEST_USER_ID" "$TEST_NETWORK_ID"
}
trap cleanup_test_network EXIT

if [[ -n "${SLAN_TEST_EMAIL:-}" ]]; then
  EMAIL="$SLAN_TEST_EMAIL"
else
  EMAIL="linux-remote-install-$(date +%s%N)@example.test"
fi

ssh_opts=(
  -o StrictHostKeyChecking=accept-new
  -o ServerAliveInterval=30
  -o ConnectTimeout=15
  -o NumberOfPasswordPrompts=1
)

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
set timeout 600
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
  REMOTE_EXPECT_HOST="$REMOTE_HOST" \
  REMOTE_EXPECT_USER="$REMOTE_USER" \
  REMOTE_EXPECT_PASSWORD="$REMOTE_PASSWORD" \
  REMOTE_EXPECT_LOCAL="$local_path" \
  REMOTE_EXPECT_REMOTE="$remote_path" \
  REMOTE_EXPECT_SSH_OPTS="$(printf '%s\n' "${ssh_opts[@]}")" \
  /usr/bin/expect <<'EOF'
set timeout 1200
set ssh_opts [split $env(REMOTE_EXPECT_SSH_OPTS) "\n"]
set remote_host $env(REMOTE_EXPECT_HOST)
set remote_user $env(REMOTE_EXPECT_USER)
set remote_password $env(REMOTE_EXPECT_PASSWORD)
set local_path $env(REMOTE_EXPECT_LOCAL)
set remote_path $env(REMOTE_EXPECT_REMOTE)
spawn scp {*}$ssh_opts $local_path ${remote_user}@${remote_host}:$remote_path
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

resolve_linux_package_path() {
  if [[ -n "${SLAN_REMOTE_LINUX_PACKAGE:-}" ]]; then
    printf '%s\n' "$SLAN_REMOTE_LINUX_PACKAGE"
    return
  fi
  local candidate="$ROOT_DIR/client_v2/.tmp/installer/linux/SLAN-Client-V2-linux-amd64.tar.gz"
  if [[ -f "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return
  fi
  if is_truthy "$BUILD_REMOTE_PACKAGE"; then
    bash "$ROOT_DIR/scripts/build_linux_client_remote.sh"
    [[ -f "$candidate" ]] || fail "remote build finished but package missing: $candidate"
    printf '%s\n' "$candidate"
    return
  fi
  fail "Linux amd64 package not found: $candidate"
}

main() {
  need expect
  need scp
  need ssh
  need curl
  need jq

  log "register/login bootstrap test user"
  curl --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" >/dev/null 2>&1 || true

  local auth_json user_id user_token network_id bootstrap_json bootstrap_id bootstrap_key install_command
  auth_json="$(curl --silent --show-error --fail \
    -X POST "${BIZ_URL}/api/app/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")"
  user_id="$(printf '%s' "$auth_json" | jq -r '.userId // .auth.userId // .auth.session.userId // empty')"
  user_token="$(printf '%s' "$auth_json" | jq -r '.accessToken // .token // .auth.accessToken // .auth.session.token // empty')"
  [[ -n "$user_id" && -n "$user_token" ]] || fail "failed to login bootstrap test user"

  log "create managed test network"
  slan_ops_create_test_network "$OPS_BASE_URL" "$user_id" "linux-remote-$(date +%s%N)" \
    || fail "failed to create managed test network"
  network_id="$SLAN_OPS_TEST_NETWORK_ID"
  TEST_USER_ID="$user_id"
  TEST_NETWORK_ID="$network_id"

  log "create bootstrap installation key"
  bootstrap_json="$(curl --silent --show-error --fail \
    -X POST "${WEB_BASE_URL}/api/app/device-bootstrap-keys" \
    -H "Authorization: Bearer ${user_token}" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"${user_id}\",\"networkId\":\"${network_id}\",\"deviceAlias\":\"${DEVICE_ALIAS}\",\"ttlSeconds\":${TTL_SECONDS}}")"
  bootstrap_id="$(printf '%s' "$bootstrap_json" | jq -r '.id // .installationKeyId // empty')"
  bootstrap_key="$(printf '%s' "$bootstrap_json" | jq -r '.key // .installationKey // empty')"
  [[ -n "$bootstrap_id" && -n "$bootstrap_key" ]] || fail "failed to create bootstrap installation key"

  local package_path
  package_path="$(resolve_linux_package_path)"
  local package_name
  package_name="$(basename "$package_path")"
  install_command="bash \"$REMOTE_DIR/install.sh\" --server=\"$BIZ_URL\" --installation-key=\"$bootstrap_key\" --tray=\"$TRAY_MODE\" --package-url=\"file://$REMOTE_DIR/$package_name\""

  log "prepare remote Linux install host"
  remote_expect_ssh "mkdir -p '$REMOTE_DIR'"
  remote_expect_ssh "bash -lc '
set -euo pipefail
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update >/dev/null
  apt-get install -y bash curl ca-certificates tar jq netcat-openbsd >/dev/null
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y bash curl ca-certificates tar jq nmap-ncat >/dev/null
elif command -v yum >/dev/null 2>&1; then
  yum install -y bash curl ca-certificates tar jq nc >/dev/null
else
  echo unsupported remote package manager >&2
  exit 1
fi
'"

  log "upload Linux package to remote host"
  remote_expect_scp "$package_path" "$REMOTE_DIR/$package_name"
  remote_expect_ssh "mkdir -p '$REMOTE_DIR/lib'"
  remote_expect_scp "$ROOT_DIR/client_v2/install/linux/install.sh" "$REMOTE_DIR/install.sh"
  remote_expect_scp "$ROOT_DIR/client_v2/install/linux/lib/slan-linux-install.sh" "$REMOTE_DIR/lib/slan-linux-install.sh"

  log "run remote Linux installer"
  remote_expect_ssh "bash -lc '
set -euo pipefail
${install_command}
'"

  log "verify remote Linux installed files and service"
  remote_expect_ssh "bash -lc '
set -euo pipefail
test -x /opt/slan-client-v2/bin/client-core-service
test -x /usr/bin/slan-client-v2-console
test -f /etc/slan/bootstrap.env
grep -q \"^SLAN_CONTROL_BASE_URL=${BIZ_URL}\$\" /etc/slan/bootstrap.env
grep -q \"^SLAN_INSTALLATION_KEY=${bootstrap_key}\$\" /etc/slan/bootstrap.env
test -f /etc/slan/client-v2-install.env
grep -q \"^SLAN_LINUX_TRAY_MODE=${TRAY_MODE}\$\" /etc/slan/client-v2-install.env
jq -e \".deviceId | strings | select(length != 0)\" /var/lib/SLAN/config.json >/dev/null
systemctl is-active --quiet slan-client-v2.service
'"

  log "verify remote Linux local API"
  remote_expect_ssh "bash -lc '
set -euo pipefail
printf \"{\\\"method\\\":\\\"localStatus\\\",\\\"args\\\":{}}\\n\" | nc -w 5 \"$SERVICE_HOST\" \"$SERVICE_PORT\" | jq -e \"(.signedIn | type) == \\\"boolean\\\" and (.switchEnabled | type) == \\\"boolean\\\"\"
'"

  echo "linuxRemoteInstallCheck: ok host=$REMOTE_HOST package=$package_path tray=$TRAY_MODE bootstrapKeyId=$bootstrap_id"
}

main "$@"
