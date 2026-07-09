#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

DEPLOY_SCRIPT="${SLAN_REMOTE_DEPLOY_SCRIPT:-$ROOT_DIR/.tmp/remote-deploy/deploy_to_47.245.40.231.sh}"
DUAL_ARCH_CHECK_SCRIPT="${SLAN_LINUX_DUAL_ARCH_CHECK_SCRIPT:-$ROOT_DIR/scripts/tests/linux/linux_dual_arch_publish_check.sh}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ -x "$DEPLOY_SCRIPT" ]] || fail "missing deploy script: $DEPLOY_SCRIPT"
[[ -x "$DUAL_ARCH_CHECK_SCRIPT" ]] || fail "missing dual-arch check script: $DUAL_ARCH_CHECK_SCRIPT"

log "deploy remote stack"
"$DEPLOY_SCRIPT"

log "publish Linux amd64/arm64 packages and verify live install.sh"
"$DUAL_ARCH_CHECK_SCRIPT"

echo "linuxRemoteRecoverPublishCheck: ok deploy=$DEPLOY_SCRIPT check=$DUAL_ARCH_CHECK_SCRIPT"
