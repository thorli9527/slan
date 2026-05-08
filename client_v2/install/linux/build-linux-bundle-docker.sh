#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)"
image_name="${SLAN_LINUX_BUILDER_IMAGE:-slan-client-v2-linux-builder}"
dockerfile="$repo_root/client_v2/install/linux/Dockerfile.build"
platform="${SLAN_LINUX_DOCKER_PLATFORM:-linux/amd64}"
package_variant="${SLAN_LINUX_PACKAGE_VARIANT:-all}"

usage() {
  cat <<'EOF'
Usage: build-linux-bundle-docker.sh [options]

Options:
  --image=NAME             Docker image name. Default: slan-client-v2-linux-builder
  --platform=PLATFORM      Docker platform. Default: linux/amd64
  --variant=gui|console|all Package variant passed to package-linux.sh. Default: all
  --no-build-image         Reuse an existing builder image.
  -h, --help               Show this help.
EOF
}

build_image="true"
for arg in "$@"; do
  case "$arg" in
    --image=*)
      image_name="${arg#--image=}"
      ;;
    --platform=*)
      platform="${arg#--platform=}"
      ;;
    --variant=*)
      package_variant="${arg#--variant=}"
      ;;
    --no-build-image)
      build_image="false"
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$package_variant" in
  gui|console|all)
    ;;
  *)
    echo "Invalid variant: $package_variant" >&2
    exit 2
    ;;
esac

if [ "$build_image" = "true" ]; then
  docker build --platform "$platform" -f "$dockerfile" -t "$image_name" "$repo_root"
fi

docker run --rm --platform "$platform" \
  -v "$repo_root:/workspace" \
  -w /workspace \
  -e PUB_CACHE=/workspace/client_v2/.tmp/flutter-pub-cache \
  "$image_name" \
  bash -lc "cd client_v2/rust && CARGO_TARGET_DIR=/workspace/client_v2/.tmp/docker-rust-target cargo build -p client-core-service --release && mkdir -p /workspace/client_v2/.tmp/linux-build && cp /workspace/client_v2/.tmp/docker-rust-target/release/client-core-service /workspace/client_v2/.tmp/linux-build/client-core-service && cd /workspace/client_v2/app_flutter && flutter config --enable-linux-desktop && flutter pub get && flutter build linux --release && /workspace/client_v2/install/linux/package-linux.sh --service-bin=/workspace/client_v2/.tmp/linux-build/client-core-service --variant=$package_variant"
