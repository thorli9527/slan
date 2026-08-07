#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${SLAN_LINUX_BUILD_IMAGE:-ubuntu:24.04}"
CONTAINER_NAME="${SLAN_LINUX_BUILD_CONTAINER:-slan-linux-client-build}"
OUTPUT_DIR="${SLAN_LINUX_BUILD_OUTPUT_DIR:-$ROOT_DIR/client/.tmp/installer/linux}"
VARIANT="${SLAN_LINUX_BUILD_VARIANT:-console}"
VERSION="${SLAN_CLIENT_V2_VERSION:-0.1.0}"
PLATFORM="${SLAN_LINUX_BUILD_PLATFORM:-}"
ARCHIVE_PATH=""

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

need docker

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$OUTPUT_DIR"

log "start build container: $IMAGE"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker_run_args=(
  run -d
  --name "$CONTAINER_NAME"
  -v "$ROOT_DIR:/workspace/slan"
  -w /workspace/slan
)
if [[ -n "$PLATFORM" ]]; then
  docker_run_args+=(--platform "$PLATFORM")
fi
docker_run_args+=("$IMAGE" sleep infinity)
docker "${docker_run_args[@]}" >/dev/null

log "install Linux build dependencies"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y \
  bash \
  build-essential \
  ca-certificates \
  clang \
  curl \
  file \
  git \
  pkg-config
'

if [[ "$VARIANT" != "console" ]]; then
  log "install Linux desktop build dependencies"
  docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get install -y \
  cmake \
  libgtk-3-dev \
  liblzma-dev \
  libsecret-1-dev \
  libstdc++-12-dev \
  ninja-build \
  unzip \
  xz-utils \
  zip
'
fi

log "install Rust toolchain"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
if ! command -v cargo >/dev/null 2>&1; then
  curl https://sh.rustup.rs -sSf | sh -s -- -y
fi
'

log "build Linux client-core-service"
docker exec "$CONTAINER_NAME" bash -lc '
set -euo pipefail
source "$HOME/.cargo/env"
cd /workspace/slan/client/rust
cargo build -p client-core-service --release
'

log "package Linux client"
docker exec "$CONTAINER_NAME" bash -lc "
set -euo pipefail
source \"\$HOME/.cargo/env\"
cd /workspace/slan
bash scripts/package_linux.sh \
  --service-bin=/workspace/slan/client/rust/target/release/client-core-service \
  --variant=${VARIANT} \
  --version=${VERSION} \
  --output-dir=/workspace/slan/client/.tmp/installer/linux
"

ARCHIVE_PATH="$(find "$OUTPUT_DIR" -maxdepth 1 -type f -name 'SLAN-Client-V2-linux-*.tar.gz' | sort | tail -1)"
[[ -n "$ARCHIVE_PATH" ]] || fail "failed to locate Linux package tarball in $OUTPUT_DIR"

log "built Linux package: $ARCHIVE_PATH"
echo "linuxDockerBuild: ok archive=$ARCHIVE_PATH image=$IMAGE variant=$VARIANT version=$VERSION"
