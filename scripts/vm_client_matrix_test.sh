#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
VMRUN="${SLAN_VMWARE_VMRUN:-/Applications/VMware Fusion.app/Contents/Library/vmrun}"
MODE="${SLAN_VM_TEST_MODE:-all}"
REMOTE_DIR="${SLAN_VM_TEST_REMOTE_DIR:-/tmp/slan-vm-client-tests}"

linux_vmx="${SLAN_LINUX_VM_VMX:-}"
linux_host="${SLAN_LINUX_VM_HOST:-}"
linux_user="${SLAN_LINUX_VM_USER:-}"
linux_ssh_key="${SLAN_LINUX_VM_SSH_KEY:-}"

windows_host="${SLAN_WINDOWS_VM_HOST:-}"
windows_user="${SLAN_WINDOWS_VM_USER:-}"
windows_ssh_key="${SLAN_WINDOWS_VM_SSH_KEY:-}"

api_url="${SLAN_TEST_API_URL:-http://47.245.40.231:28080}"
web_url="${SLAN_TEST_WEB_URL:-http://47.245.40.231:24200}"
ops_url="${SLAN_TEST_OPS_URL:-http://47.245.40.231:24201}"
mqtt_host="${SLAN_TEST_MQTT_HOST:-47.245.40.231}"
mqtt_port="${SLAN_TEST_MQTT_PORT:-1883}"
wire_host="${SLAN_TEST_WIRE_HOST:-47.245.40.231}"
wire_port="${SLAN_TEST_WIRE_PORT:-29100}"
relay_host="${SLAN_TEST_RELAY_HOST:-47.245.40.231}"
relay_port="${SLAN_TEST_RELAY_PORT:-29110}"
derp_host="${SLAN_TEST_DERP_HOST:-47.245.40.231}"
derp_port="${SLAN_TEST_DERP_PORT:-29120}"

usage() {
  cat <<'EOF'
Usage:
  SLAN_LINUX_VM_HOST=<ip> SLAN_LINUX_VM_USER=<user> scripts/vm_client_matrix_test.sh

Optional:
  SLAN_LINUX_VM_VMX="/Users/me/Virtual Machines.localized/Ubuntu.vmwarevm/Ubuntu.vmx"
  SLAN_LINUX_VM_SSH_KEY="$HOME/.ssh/id_ed25519"
  SLAN_WINDOWS_VM_HOST=<ip>
  SLAN_WINDOWS_VM_USER=<user>
  SLAN_WINDOWS_VM_SSH_KEY="$HOME/.ssh/id_ed25519"
  SLAN_VM_TEST_MODE=all|linux|windows

The Linux VM must already have the SLAN Linux client installed at
/opt/slan-client-v2 and SSH reachable from this Mac.
EOF
}

log() {
  printf '==> %s\n' "$*"
}

warn() {
  printf 'warn: %s\n' "$*" >&2
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

ssh_opts() {
  local key="$1"
  printf '%s\n' "-o"
  printf '%s\n' "StrictHostKeyChecking=accept-new"
  printf '%s\n' "-o"
  printf '%s\n' "ServerAliveInterval=30"
  if [[ -n "$key" ]]; then
    printf '%s\n' "-i"
    printf '%s\n' "$key"
  fi
}

run_ssh() {
  local user="$1"
  local host="$2"
  local key="$3"
  shift 3
  local opts=()
  while IFS= read -r opt; do
    opts+=("$opt")
  done < <(ssh_opts "$key")
  ssh "${opts[@]}" "${user}@${host}" "$@"
}

copy_to_linux() {
  local opts=()
  while IFS= read -r opt; do
    opts+=("$opt")
  done < <(ssh_opts "$linux_ssh_key")
  run_ssh "$linux_user" "$linux_host" "$linux_ssh_key" "mkdir -p '$REMOTE_DIR/scripts'"
  scp "${opts[@]}" "$ROOT_DIR/scripts/linux_client_integration_test.sh" "${linux_user}@${linux_host}:${REMOTE_DIR}/scripts/"
}

start_vm_if_configured() {
  local vmx="$1"
  if [[ -z "$vmx" ]]; then
    return 0
  fi
  if [[ ! -x "$VMRUN" ]]; then
    warn "vmrun not executable: $VMRUN"
    return 0
  fi
  if [[ ! -f "$vmx" ]]; then
    fail "VMX not found: $vmx"
  fi
  log "checking VMware VM state"
  if "$VMRUN" -T fusion list 2>/dev/null | grep -Fq "$vmx"; then
    log "VM already running: $vmx"
    return 0
  fi
  log "starting VM: $vmx"
  if ! "$VMRUN" -T fusion start "$vmx" nogui; then
    warn "vmrun failed to start VM; continue if SSH is already reachable"
  fi
}

wait_for_ssh() {
  local user="$1"
  local host="$2"
  local key="$3"
  local deadline=$((SECONDS + 120))
  while (( SECONDS < deadline )); do
    if run_ssh "$user" "$host" "$key" "true" >/dev/null 2>&1; then
      return 0
    fi
    sleep 3
  done
  fail "SSH not reachable: ${user}@${host}"
}

run_linux_matrix() {
  if [[ -z "$linux_host" || -z "$linux_user" ]]; then
    warn "skip Linux VM: set SLAN_LINUX_VM_HOST and SLAN_LINUX_VM_USER"
    return 0
  fi
  start_vm_if_configured "$linux_vmx"
  wait_for_ssh "$linux_user" "$linux_host" "$linux_ssh_key"
  copy_to_linux
  log "running Linux client data-plane smoke on ${linux_user}@${linux_host}"
  run_ssh "$linux_user" "$linux_host" "$linux_ssh_key" \
    "cd '$REMOTE_DIR' && sudo env \
      SLAN_TEST_API_URL='$api_url' \
      SLAN_TEST_WEB_URL='$web_url' \
      SLAN_TEST_OPS_URL='$ops_url' \
      SLAN_TEST_MQTT_HOST='$mqtt_host' \
      SLAN_TEST_MQTT_PORT='$mqtt_port' \
      SLAN_TEST_WIRE_HOST='$wire_host' \
      SLAN_TEST_WIRE_PORT='$wire_port' \
      SLAN_TEST_RELAY_HOST='$relay_host' \
      SLAN_TEST_RELAY_PORT='$relay_port' \
      SLAN_TEST_DERP_HOST='$derp_host' \
      SLAN_TEST_DERP_PORT='$derp_port' \
      bash scripts/linux_client_integration_test.sh"
}

run_windows_matrix() {
  if [[ -z "$windows_host" || -z "$windows_user" ]]; then
    warn "skip Windows VM: set SLAN_WINDOWS_VM_HOST and SLAN_WINDOWS_VM_USER"
    return 0
  fi
  wait_for_ssh "$windows_user" "$windows_host" "$windows_ssh_key"
  log "Windows VM reachable. Add the Windows client smoke command here once the Windows package path is fixed."
  run_ssh "$windows_user" "$windows_host" "$windows_ssh_key" "hostname && powershell -NoProfile -Command \"Get-NetAdapter | Select-Object -First 5 | Format-Table -AutoSize\""
}

case "${1:-$MODE}" in
  all)
    run_linux_matrix
    run_windows_matrix
    ;;
  linux)
    run_linux_matrix
    ;;
  windows)
    run_windows_matrix
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
