#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
OUTPUT_DIR="${SLAN_LINUX_BUILD_OUTPUT_DIR:-$ROOT_DIR/client_v2/.tmp/installer/linux}"
PACKAGE_PATH="${SLAN_LINUX_CLIENT_PACKAGE:-}"
PREFERRED_ARCH="${SLAN_LINUX_PUBLISH_ARCH:-amd64}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

if [[ -z "$PACKAGE_PATH" ]]; then
  if [[ "$PREFERRED_ARCH" == "amd64" ]]; then
    log "build preferred Linux amd64 package on remote Linux host"
    bash "$ROOT_DIR/scripts/build_linux_client_remote.sh"
  else
    log "build Linux package in Docker"
    bash "$ROOT_DIR/scripts/build_linux_client_docker.sh"
  fi
  PACKAGE_PATH="$OUTPUT_DIR/SLAN-Client-V2-linux-${PREFERRED_ARCH}.tar.gz"
fi

[[ -n "$PACKAGE_PATH" && -f "$PACKAGE_PATH" ]] || fail "missing Linux package tarball"

log "upload Linux package to remote ops downloads"
bash "$ROOT_DIR/scripts/upload_linux_client_download.sh" "$PACKAGE_PATH"

log "run Linux Docker bootstrap install verification"
bash "$ROOT_DIR/scripts/tests/linux/linux_docker_bootstrap_install_check.sh"
