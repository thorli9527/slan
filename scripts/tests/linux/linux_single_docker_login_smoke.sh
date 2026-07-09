#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

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
RUNS="${SLAN_LINUX_SINGLE_RUNS:-1}"
IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
TIMEOUT_SECONDS="${SLAN_LINUX_RUNTIME_TIMEOUT_SECONDS:-90}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ -f "$PACKAGE_PATH" ]] || fail "Linux client package not found: $PACKAGE_PATH"
[[ "$RUNS" =~ ^[0-9]+$ ]] || fail "SLAN_LINUX_SINGLE_RUNS must be an integer"
(( RUNS > 0 )) || fail "SLAN_LINUX_SINGLE_RUNS must be > 0"

successes=0

for run in $(seq 1 "$RUNS"); do
  log "single docker login smoke run ${run}/${RUNS}"
  SLAN_LINUX_CLIENT_PACKAGE="$PACKAGE_PATH" \
  SLAN_LINUX_DOCKER_IMAGE="$IMAGE" \
  SLAN_LINUX_RUNTIME_TIMEOUT_SECONDS="$TIMEOUT_SECONDS" \
  SLAN_LINUX_DOCKER_NAME="slan-linux-runtime-check-${run}" \
  bash "$ROOT_DIR/scripts/linux_docker_runtime_login_check.sh"
  successes=$((successes + 1))
done

printf 'linuxSingleDockerLoginSmoke: ok runs=%d image=%s package=%s\n' \
  "$successes" "$IMAGE" "$PACKAGE_PATH"
