#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

REMOTE_HOST="${SLAN_REMOTE_LINUX_HOST:-100.87.66.24}"
REMOTE_USER="${SLAN_REMOTE_LINUX_USER:-root}"
REMOTE_PASSWORD="${SLAN_REMOTE_LINUX_PASSWORD:-}"
REMOTE_SSH_KEY="${SLAN_REMOTE_LINUX_SSH_KEY:-}"
REMOTE_BUILD_DIR="${SLAN_REMOTE_LINUX_BUILD_DIR:-/tmp/slan-linux-remote-build}"
REMOTE_OUTPUT_DIR="$REMOTE_BUILD_DIR/client_v2/.tmp/installer/linux"
LOCAL_OUTPUT_DIR="${SLAN_LINUX_BUILD_OUTPUT_DIR:-$ROOT_DIR/client_v2/.tmp/installer/linux}"
VARIANT="${SLAN_LINUX_BUILD_VARIANT:-console}"
VERSION="${SLAN_CLIENT_V2_VERSION:-0.1.0}"
RUST_PROFILE="${SLAN_REMOTE_LINUX_RUST_PROFILE:-debug}"
SKIP_FETCH="${SLAN_REMOTE_LINUX_SKIP_FETCH:-0}"
LOCAL_ARCHIVE="${LOCAL_OUTPUT_DIR}/SLAN-Client-V2-linux-amd64.tar.gz"
LOCAL_DEB="${LOCAL_OUTPUT_DIR}/slan-client-v2_${VERSION}_amd64.deb"
ARCHIVE_BASENAME="$(basename "$LOCAL_ARCHIVE")"
DEB_BASENAME="$(basename "$LOCAL_DEB")"
LOCAL_SOURCE_TAR="${TMPDIR:-/tmp}/slan-linux-remote-build-src-$$.tar.gz"

ssh_opts=(
  -o StrictHostKeyChecking=accept-new
  -o ServerAliveInterval=30
  -o ConnectTimeout=15
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

validate_profile() {
  case "$RUST_PROFILE" in
    debug|release)
      ;;
    *)
      fail "unsupported SLAN_REMOTE_LINUX_RUST_PROFILE=$RUST_PROFILE; use debug or release"
      ;;
  esac
}

cleanup() {
  rm -f "$LOCAL_SOURCE_TAR"
}
trap cleanup EXIT

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

remote_expect_download() {
  local remote_path="$1"
  local local_path="$2"
  if [[ -n "$REMOTE_SSH_KEY" ]]; then
    scp "${ssh_opts[@]}" -i "$REMOTE_SSH_KEY" "${REMOTE_USER}@${REMOTE_HOST}:$remote_path" "$local_path"
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
spawn scp {*}$ssh_opts ${remote_user}@${remote_host}:$remote_path $local_path
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

prepare_source_archive() {
  log "prepare Linux remote build source archive"
  tar -C "$ROOT_DIR" \
    --exclude='client_v2/rust/target' \
    --exclude='client_v2/rust/.cargo' \
    --exclude='client_v2/.tmp' \
    -czf "$LOCAL_SOURCE_TAR" \
    scripts/package_linux.sh \
    client_v2/install/linux \
    client_v2/rust
}

install_remote_deps() {
  log "install remote Linux build dependencies"
  remote_expect_ssh "mkdir -p '$REMOTE_BUILD_DIR'"
  remote_expect_ssh "bash -lc '
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
if command -v apt-get >/dev/null 2>&1; then
  apt-get update >/dev/null
  apt-get install -y bash build-essential ca-certificates clang curl file git pkg-config tar >/dev/null
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y bash gcc gcc-c++ make clang curl file git pkgconf-pkg-config tar >/dev/null
elif command -v yum >/dev/null 2>&1; then
  yum install -y bash gcc gcc-c++ make clang curl file git pkgconfig tar >/dev/null
else
  echo unsupported remote package manager >&2
  exit 1
fi
if ! command -v cargo >/dev/null 2>&1; then
  curl https://sh.rustup.rs -sSf | sh -s -- -y >/dev/null
fi
'"
}

sync_sources() {
  log "sync sources to remote Linux builder"
  remote_expect_scp "$LOCAL_SOURCE_TAR" "$REMOTE_BUILD_DIR/source.tar.gz"
  remote_expect_ssh "bash -lc '
set -euo pipefail
rm -rf \"$REMOTE_BUILD_DIR/scripts\" \"$REMOTE_BUILD_DIR/client_v2\"
mkdir -p \"$REMOTE_BUILD_DIR\"
tar -C \"$REMOTE_BUILD_DIR\" -xzf \"$REMOTE_BUILD_DIR/source.tar.gz\"
'"
}

build_remote_package() {
  local cargo_args=("build" "-p" "client-core-service")
  local service_bin="$REMOTE_BUILD_DIR/client_v2/rust/target/${RUST_PROFILE}/client-core-service"
  if [[ "$RUST_PROFILE" == "release" ]]; then
    cargo_args+=("--release")
  fi
  log "build Linux amd64 package on remote host profile=$RUST_PROFILE"
  remote_expect_ssh "bash -lc '
set -euo pipefail
source \"\$HOME/.cargo/env\"
cd \"$REMOTE_BUILD_DIR/client_v2/rust\"
cargo ${cargo_args[*]}
cd \"$REMOTE_BUILD_DIR\"
bash scripts/package_linux.sh \
  --service-bin=\"$service_bin\" \
  --variant=\"$VARIANT\" \
  --version=\"$VERSION\" \
  --output-dir=\"$REMOTE_OUTPUT_DIR\"
'"
}

fetch_remote_package() {
  log "fetch Linux amd64 package from remote host"
  mkdir -p "$LOCAL_OUTPUT_DIR"
  remote_expect_download "${REMOTE_OUTPUT_DIR}/${ARCHIVE_BASENAME}" "$LOCAL_ARCHIVE"
  remote_expect_ssh "test -f '${REMOTE_OUTPUT_DIR}/${DEB_BASENAME}'" >/dev/null 2>&1 || return 0
  remote_expect_download "${REMOTE_OUTPUT_DIR}/${DEB_BASENAME}" "$LOCAL_DEB"
}

main() {
  need tar
  need scp
  need ssh
  need expect
  validate_profile
  prepare_source_archive
  install_remote_deps
  sync_sources
  build_remote_package
  if is_truthy "$SKIP_FETCH"; then
    printf 'remoteLinuxBuild: ok host=%s remotePackage=%s profile=%s\n' \
      "$REMOTE_HOST" "${REMOTE_OUTPUT_DIR}/${ARCHIVE_BASENAME}" "$RUST_PROFILE"
    return 0
  fi
  fetch_remote_package
  [[ -f "$LOCAL_ARCHIVE" ]] || fail "missing fetched Linux amd64 tarball: $LOCAL_ARCHIVE"
  printf 'remoteLinuxBuild: ok host=%s package=%s profile=%s\n' \
    "$REMOTE_HOST" "$LOCAL_ARCHIVE" "$RUST_PROFILE"
}

main "$@"
