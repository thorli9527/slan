#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"

REMOTE_HOST="${1:-${SLAN_REMOTE_HOST:-$SLAN_DEFAULT_MQTT_HOST}}"
REMOTE_DIR="${REMOTE_DIR:-/opt/slan}"
REMOTE_USER="${REMOTE_USER:-root}"
PACKAGE="${SLAN_SERVER_DOCKER_PACKAGE:-}"
REMOTE_PACKAGE_DIR="${SLAN_REMOTE_PACKAGE_DIR:-/tmp/slan-server-release}"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30)
SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
TEMP_ENV=""

cleanup() {
  if [ -n "$TEMP_ENV" ]; then
    rm -f "$TEMP_ENV"
  fi
}
trap cleanup EXIT

if [ -z "$PACKAGE" ]; then
  "$ROOT_DIR/scripts/package_server_docker.sh"
  PACKAGE=$(find "$ROOT_DIR/.tmp/server-docker" -maxdepth 1 -name 'SLAN-server-docker-*-linux-amd64.tar.gz' -type f -print | sort | tail -n1)
fi
if [ ! -f "$PACKAGE" ]; then
  echo "server Docker package not found: $PACKAGE" >&2
  exit 1
fi

echo "==> Verifying package checksum"
expected_checksum=$(awk '{print $1}' "${PACKAGE}.sha256")
actual_checksum=$(shasum -a 256 "$PACKAGE" | awk '{print $1}')
if [ -z "$expected_checksum" ] || [ "$actual_checksum" != "$expected_checksum" ]; then
  echo "server Docker package checksum mismatch" >&2
  exit 1
fi

echo "==> Uploading server Docker package"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "mkdir -p '$REMOTE_PACKAGE_DIR'"
remote_package="$REMOTE_PACKAGE_DIR/$(basename "$PACKAGE")"
scp "${SSH_OPTS[@]}" "$PACKAGE" "$SSH_TARGET:$remote_package"

echo "==> Loading server Docker images"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "printf '%s  %s\n' '$actual_checksum' '$remote_package' | sha256sum -c - && gzip -dc '$remote_package' | docker image load"

env_source="${ENV_SOURCE:-}"
if [ -z "$env_source" ]; then
  TEMP_ENV=$(mktemp "${TMPDIR:-/tmp}/slan-env.XXXXXX")
  chmod 600 "$TEMP_ENV"
  scp "${SSH_OPTS[@]}" "$SSH_TARGET:$REMOTE_DIR/.env.prod" "$TEMP_ENV"
  env_source="$TEMP_ENV"
fi

echo "==> Publishing preloaded images"
ENV_SOURCE="$env_source" \
SKIP_REMOTE_BUILD=1 \
RUN_REMOTE_SMOKE="${RUN_REMOTE_SMOKE:-1}" \
RUN_REMOTE_PUNCH_SMOKE="${RUN_REMOTE_PUNCH_SMOKE:-1}" \
RUN_REMOTE_UI_OPS_SMOKE="${RUN_REMOTE_UI_OPS_SMOKE:-1}" \
"$ROOT_DIR/scripts/remote_docker_deploy.sh" "$REMOTE_HOST" "$REMOTE_DIR" .env.prod
