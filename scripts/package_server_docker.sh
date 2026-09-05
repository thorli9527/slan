#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
OUTPUT_DIR="${SLAN_SERVER_PACKAGE_OUTPUT_DIR:-$ROOT_DIR/.tmp/server-docker}"
PLATFORM="${SLAN_SERVER_DOCKER_PLATFORM:-linux/amd64}"
VERSION="${SLAN_SERVER_PACKAGE_VERSION:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD)}"
ARCHIVE="${1:-$OUTPUT_DIR/SLAN-server-docker-${VERSION}-linux-amd64.tar.gz}"
MANIFEST="${ARCHIVE%.tar.gz}.manifest.txt"
CHECKSUM="${ARCHIVE}.sha256"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.local.yml}"
BUILD_SERVICES=(server-biz server-wire server-wire-relay server-wire-punch server-wire-derp server-edge-proxy opt-ui)
APP_IMAGES=(
  slan-service-biz:latest
  slan-server-wire:latest
  slan-server-wire-relay:latest
  slan-server-wire-punch:latest
  slan-server-wire-derp:latest
  slan-server-edge-proxy:latest
  slan-opt-ui:latest
)
INFRA_IMAGES=(
  postgres:16-alpine
  redis:7-alpine
  emqx/emqx:5.8.9
  caddy:2-alpine
)

command -v docker >/dev/null 2>&1 || { echo "docker command not found" >&2; exit 1; }
command -v gzip >/dev/null 2>&1 || { echo "gzip command not found" >&2; exit 1; }
mkdir -p "$OUTPUT_DIR" "$(dirname "$ARCHIVE")"

echo "==> Building unique server images for $PLATFORM"
DOCKER_DEFAULT_PLATFORM="$PLATFORM" docker compose -f "$ROOT_DIR/$COMPOSE_FILE" build "${BUILD_SERVICES[@]}"

echo "==> Pulling infrastructure images for $PLATFORM"
for image in "${INFRA_IMAGES[@]}"; do
  docker pull --platform "$PLATFORM" "$image"
done

all_images=("${APP_IMAGES[@]}" "${INFRA_IMAGES[@]}")
for image in "${all_images[@]}"; do
  docker image inspect "$image" >/dev/null
done

tmp_archive="${ARCHIVE}.tmp"
rm -f "$tmp_archive"
echo "==> Exporting ${#all_images[@]} images"
docker image save "${all_images[@]}" | gzip -9 > "$tmp_archive"
mv "$tmp_archive" "$ARCHIVE"

{
  echo "version=$VERSION"
  echo "gitCommit=$(git -C "$ROOT_DIR" rev-parse HEAD)"
  echo "gitBranch=$(git -C "$ROOT_DIR" branch --show-current)"
  echo "platform=$PLATFORM"
  echo "createdAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  for image in "${all_images[@]}"; do
    printf 'image=%s id=%s\n' "$image" "$(docker image inspect --format '{{.Id}}' "$image")"
  done
} > "$MANIFEST"

shasum -a 256 "$ARCHIVE" > "$CHECKSUM"
echo "serverDockerPackage: $ARCHIVE"
echo "manifest: $MANIFEST"
echo "checksum: $CHECKSUM"
