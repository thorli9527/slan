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

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

if [[ -z "$PACKAGE_PATH" ]]; then
  log "build Linux package in Docker"
  bash "$ROOT_DIR/scripts/build_linux_client_docker.sh"
  PACKAGE_PATH="$(find "$OUTPUT_DIR" -maxdepth 1 -type f -name 'SLAN-Client-V2-linux-*.tar.gz' | sort | tail -1)"
fi

[[ -n "$PACKAGE_PATH" && -f "$PACKAGE_PATH" ]] || fail "missing Linux package tarball"

log "upload Linux package to remote ops downloads"
bash "$ROOT_DIR/scripts/upload_linux_client_download.sh" "$PACKAGE_PATH"

log "run Linux Docker bootstrap install verification"
bash "$ROOT_DIR/scripts/linux_docker_bootstrap_install_check.sh"
