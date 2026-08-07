#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [[ ! -e "$ROOT_DIR/.git" && "$ROOT_DIR" != "/" ]]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

IMAGE="${SLAN_LINUX_DOCKER_IMAGE:-ubuntu:24.04}"
PLATFORM="${SLAN_LINUX_DOCKER_PLATFORM:-linux/amd64}"
CONTAINER_NAME="${SLAN_LINUX_DOCKER_NAME:-slan-linux-console-installer-check}"
SERVER_URL="${SLAN_BIZ_URL:-http://127.0.0.1:28080}"
AUTHORIZATION_KEY="${SLAN_TEST_DEVICE_AUTHORIZATION_KEY:-console-installer-test-key}"
INSTALLER="${SLAN_LINUX_CONSOLE_INSTALLER:-$ROOT_DIR/client/.tmp/installer/linux/SLAN-Client-V2-linux-amd64-console.run}"

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

[[ -x "$INSTALLER" ]] || {
  echo "Linux console installer not found or not executable: $INSTALLER" >&2
  exit 1
}

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
  --platform "$PLATFORM" \
  --name "$CONTAINER_NAME" \
  -v "$ROOT_DIR:/workspace/slan:ro" \
  "$IMAGE" sleep infinity >/dev/null

docker exec "$CONTAINER_NAME" sh -c '
set -eu
apt-get update >/dev/null
DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates tar >/dev/null
'

installer_in_container="/workspace/slan/${INSTALLER#$ROOT_DIR/}"
docker exec "$CONTAINER_NAME" "$installer_in_container" \
  --server-url "$SERVER_URL/" \
  --authorization-key "$AUTHORIZATION_KEY"

docker exec "$CONTAINER_NAME" sh -c "
set -eu
test -x /opt/slan-client-v2/bin/client-core-service
test -x /usr/bin/slan-client-v2-console
test \"\$(stat -c '%a' /etc/slan/client-v2-console.env)\" = 600
grep -Fqx 'SLAN_CONTROL_BASE_URL=$SERVER_URL' /etc/slan/client-v2-console.env
grep -Fqx 'SLAN_DEVICE_AUTHORIZATION_KEY=$AUTHORIZATION_KEY' /etc/slan/client-v2-console.env
grep -Fqx 'SLAN_PENDING_ENABLE_NETWORK=false' /etc/slan/client-v2-console.env
"

echo "linuxConsoleInstallerCheck: ok installer=$INSTALLER image=$IMAGE platform=$PLATFORM"
