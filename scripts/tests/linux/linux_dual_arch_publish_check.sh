#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

BIZ_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
AMD64_PACKAGE="${SLAN_LINUX_CLIENT_AMD64_PACKAGE:-$ROOT_DIR/client_v2/.tmp/installer/linux/SLAN-Client-V2-linux-amd64.tar.gz}"
ARM64_PACKAGE="${SLAN_LINUX_CLIENT_ARM64_PACKAGE:-$ROOT_DIR/client_v2/.tmp/installer/linux/SLAN-Client-V2-linux-arm64.tar.gz}"
VERIFY_LIVE_SCRIPT="${SLAN_VERIFY_LIVE_INSTALL_SCRIPT:-1}"

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

need bash
need curl
need grep

[[ -f "$AMD64_PACKAGE" ]] || fail "missing amd64 package: $AMD64_PACKAGE"
[[ -f "$ARM64_PACKAGE" ]] || fail "missing arm64 package: $ARM64_PACKAGE"

log "upload Linux amd64 package"
bash "$ROOT_DIR/scripts/upload_linux_client_download.sh" "$AMD64_PACKAGE"

log "upload Linux arm64 package"
bash "$ROOT_DIR/scripts/upload_linux_client_download.sh" "$ARM64_PACKAGE"

if [[ "$VERIFY_LIVE_SCRIPT" == "1" ]]; then
  log "verify live install.sh exposes amd64 and arm64 package selectors"
  install_script="$(curl --max-time 20 -fsSL "${BIZ_URL%/}/downloads/clients/install.sh")" \
    || fail "failed to fetch live install.sh from ${BIZ_URL%/}/downloads/clients/install.sh"
  printf '%s\n' "$install_script" | grep -q 'package_url_amd64=' \
    || fail "live install.sh missing package_url_amd64"
  printf '%s\n' "$install_script" | grep -q 'package_url_arm64=' \
    || fail "live install.sh missing package_url_arm64"
  printf '%s\n' "$install_script" | grep -q 'uname -m' \
    || fail "live install.sh missing uname -m architecture selection"
fi

echo "linuxDualArchPublishCheck: ok biz=${BIZ_URL} amd64=${AMD64_PACKAGE} arm64=${ARM64_PACKAGE}"
