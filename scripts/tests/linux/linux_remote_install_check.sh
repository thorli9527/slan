#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
source "$ROOT_DIR/scripts/tests/shared/ops_device_credentials.sh"

REMOTE_HOST="${SLAN_REMOTE_LINUX_HOST:-100.87.66.24}"
REMOTE_USER="${SLAN_REMOTE_LINUX_USER:-root}"
REMOTE_PASSWORD="${SLAN_REMOTE_LINUX_PASSWORD:-}"
REMOTE_SSH_KEY="${SLAN_REMOTE_LINUX_SSH_KEY:-}"
REMOTE_DIR="${SLAN_REMOTE_LINUX_WORK_DIR:-/tmp/slan-linux-remote-install-check}"
BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
OPS_BASE_URL="${SLAN_OPS_BASE_URL:-$SLAN_DEFAULT_OPS_BASE_URL}"
TRAY_MODE="${SLAN_LINUX_TRAY_MODE:-disabled}"
BUILD_REMOTE_PACKAGE="${SLAN_REMOTE_LINUX_BUILD_PACKAGE_REMOTE:-1}"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1}"
SERVICE_PORT="${SLAN_CLIENT_CORE_SERVICE_PORT:-46392}"
DEVICE_ALIAS="${SLAN_REMOTE_LINUX_DEVICE_ALIAS:-Remote Linux Install Check}"

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
  local candidate="$ROOT_DIR/client/.tmp/installer/linux/SLAN-Client-V2-linux-amd64.tar.gz"
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

  log "create Ops device authorization key"
  local ops_token bootstrap_json bootstrap_id bootstrap_key install_command
  ops_token="$(slan_ops_login "$OPS_BASE_URL")"
  bootstrap_json="$(slan_ops_create_device_credential "$OPS_BASE_URL" "$ops_token" "$DEVICE_ALIAS")"
  bootstrap_id="$(printf '%s' "$bootstrap_json" | jq -r '.credentialId // empty')"
  bootstrap_key="$(printf '%s' "$bootstrap_json" | jq -r '.key // empty')"
  [[ -n "$bootstrap_id" && -n "$bootstrap_key" ]] || fail "failed to create device authorization key"

  local package_path
  package_path="$(resolve_linux_package_path)"
  local package_name
  package_name="$(basename "$package_path")"
  install_command="bash \"$REMOTE_DIR/install.sh\" --server=\"$BIZ_URL\" --authorization-key=\"$bootstrap_key\" --tray=\"$TRAY_MODE\" --package-url=\"file://$REMOTE_DIR/$package_name\""

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
  remote_expect_scp "$ROOT_DIR/client/install/linux/install.sh" "$REMOTE_DIR/install.sh"
  remote_expect_scp "$ROOT_DIR/client/install/linux/lib/slan-linux-install.sh" "$REMOTE_DIR/lib/slan-linux-install.sh"

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
if [ -f /etc/slan/client-v2-console.env ]; then
  grep -q \"^SLAN_CONTROL_BASE_URL=${BIZ_URL}\$\" /etc/slan/client-v2-console.env
  grep -q \"^SLAN_DEVICE_AUTHORIZATION_KEY=${bootstrap_key}\$\" /etc/slan/client-v2-console.env
fi
test -f /etc/slan/client-v2-install.env
grep -q \"^SLAN_LINUX_TRAY_MODE=${TRAY_MODE}\$\" /etc/slan/client-v2-install.env
jq -e \".deviceId | strings | select(length != 0)\" /var/lib/SLAN/config.json >/dev/null
systemctl is-active --quiet slan-client-v2.service
'"

  log "verify remote Linux local API"
  remote_expect_ssh "bash -lc '
set -euo pipefail
printf \"{\\\"method\\\":\\\"localStatus\\\",\\\"args\\\":{}}\\n\" | nc -w 5 \"$SERVICE_HOST\" \"$SERVICE_PORT\" | jq -e \"(.activated | type) == \\\"boolean\\\" and (.switchEnabled | type) == \\\"boolean\\\"\"
'"

  echo "linuxRemoteInstallCheck: ok host=$REMOTE_HOST package=$package_path tray=$TRAY_MODE bootstrapKeyId=$bootstrap_id"
}

main "$@"
