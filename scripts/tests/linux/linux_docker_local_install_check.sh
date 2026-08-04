#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done
source "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_NAME:-slan-linux-local-install-check}"
SERVER_URL="${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}"
AUTHORIZATION_KEY="${SLAN_TEST_DEVICE_AUTHORIZATION_KEY:-local-docker-authorization-key}"
TRAY_MODE="${SLAN_LINUX_TRAY_MODE:-disabled}"

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

resolve_linux_package_path() {
  if [[ -n "${SLAN_LINUX_CLIENT_PACKAGE:-}" ]]; then
    printf '%s\n' "$SLAN_LINUX_CLIENT_PACKAGE"
    return
  fi

  local installer_dir="$ROOT_DIR/client/.tmp/installer/linux"
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

need docker

[[ -f "$PACKAGE_PATH" ]] || fail "package not found: $PACKAGE_PATH"

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "start local install test container: $IMAGE"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
  --name "$CONTAINER_NAME" \
  -v "$ROOT_DIR:/workspace/slan" \
  -w /workspace/slan \
  "$IMAGE" \
  sleep infinity >/dev/null

log "install minimal runtime tools"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y bash curl ca-certificates tar
'

log "run Linux installer against local package"
docker exec "$CONTAINER_NAME" bash -lc "
set -euo pipefail
cd /workspace/slan/client/install/linux
bash install.sh \
  --server='${SERVER_URL}' \
  --authorization-key='${AUTHORIZATION_KEY}' \
  --tray='${TRAY_MODE}' \
  --package-url='file:///workspace/slan/${PACKAGE_PATH#$ROOT_DIR/}'
"

log "verify installed files"
docker exec "$CONTAINER_NAME" bash -lc "
set -euo pipefail
test -x /opt/slan-client-v2/bin/client-core-service
test -f /etc/slan/client-v2-console.env
grep -q '^SLAN_CONTROL_BASE_URL=${SERVER_URL}\$' /etc/slan/client-v2-console.env
grep -q '^SLAN_DEVICE_AUTHORIZATION_KEY=${AUTHORIZATION_KEY}\$' /etc/slan/client-v2-console.env
test -f /etc/slan/client-v2-install.env
grep -q '^SLAN_LINUX_TRAY_MODE=${TRAY_MODE}\$' /etc/slan/client-v2-install.env
test -f /usr/bin/slan-client-v2-console
"

log "inspect installed tree"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
find /opt/slan-client-v2 -maxdepth 3 -type f | sort
'

echo "linuxDockerLocalInstallCheck: ok package=$PACKAGE_PATH image=$IMAGE tray=$TRAY_MODE"
