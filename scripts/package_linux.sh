#!/usr/bin/env bash
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PACKAGE_SCRIPT="$ROOT_DIR/client_v2/install/linux/package-linux.sh"
INSTALL_LIB="$ROOT_DIR/client_v2/install/linux/lib/slan-linux-install.sh"
OUTPUT_DIR="$ROOT_DIR/client_v2/.tmp/installer/linux"
ARCH="${SLAN_CLIENT_V2_ARCH:-}"
VARIANT="all"
VERSION="${SLAN_CLIENT_V2_VERSION:-0.1.0}"
BUILD=0
SERVICE_BIN="$ROOT_DIR/client_v2/rust/target/release/client-core-service"
PACKAGE_ARGS=()

usage() {
  cat <<'EOF'
Usage: package_linux.sh [options]

Options:
  --build                    Build Rust service and Flutter Linux GUI before packaging.
  --flutter-build-dir=PATH   Flutter Linux release bundle.
  --service-bin=PATH         client-core-service binary.
  --output-dir=PATH          Output directory.
  --variant=gui|console|all  Package variant.
  --version=VERSION          Package version.
  -h, --help                 Show this help.
EOF
}

for arg in "$@"; do
  case "$arg" in
    --build)
      BUILD=1
      ;;
    --output-dir=*)
      OUTPUT_DIR="${arg#--output-dir=}"
      PACKAGE_ARGS+=("$arg")
      ;;
    --variant=*)
      VARIANT="${arg#--variant=}"
      PACKAGE_ARGS+=("$arg")
      ;;
    --service-bin=*)
      SERVICE_BIN="${arg#--service-bin=}"
      PACKAGE_ARGS+=("$arg")
      ;;
    --flutter-build-dir=*|--version=*)
      if [[ "$arg" == --version=* ]]; then
        VERSION="${arg#--version=}"
      fi
      PACKAGE_ARGS+=("$arg")
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

if [ ! -x "$PACKAGE_SCRIPT" ]; then
  echo "Missing Linux package implementation: $PACKAGE_SCRIPT" >&2
  exit 1
fi

if [ ! -f "$INSTALL_LIB" ]; then
  echo "Missing Linux install library: $INSTALL_LIB" >&2
  exit 1
fi

# shellcheck disable=SC1090
. "$INSTALL_LIB"

case "$VARIANT" in
  gui|console|all)
    ;;
  *)
    echo "Invalid variant: $VARIANT" >&2
    exit 2
    ;;
esac

if [ "$BUILD" -eq 1 ]; then
  if [ "$(uname -s)" != "Linux" ]; then
    echo "--build for Linux packages must run on Linux. Build a Linux ELF separately and pass --service-bin=PATH." >&2
    exit 2
  fi
  (
    cd "$ROOT_DIR/client_v2/rust"
    cargo build -p client-core-service --release
  )
  if [ "$VARIANT" != "console" ]; then
    (
      cd "$ROOT_DIR/client_v2/app_flutter"
      flutter build linux
    )
  fi
fi

if [ ! -f "$SERVICE_BIN" ]; then
  echo "Missing Linux service binary: $SERVICE_BIN" >&2
  exit 1
fi

DETECTED_ARCH="$(slan_linux_service_arch "$SERVICE_BIN" || true)"
if [ -z "$DETECTED_ARCH" ]; then
  echo "Refusing to package non-Linux or unsupported service binary: $SERVICE_BIN" >&2
  if command -v file >/dev/null 2>&1; then
    file "$SERVICE_BIN" >&2 || true
  fi
  echo "Pass --service-bin=PATH for a Linux ELF build output." >&2
  exit 1
fi

if [ -z "$ARCH" ]; then
  ARCH="$DETECTED_ARCH"
else
  ARCH="$(slan_linux_normalize_arch "$ARCH")"
  if [ "$ARCH" != "$DETECTED_ARCH" ]; then
    echo "Linux package arch mismatch: requested $ARCH, service binary is $DETECTED_ARCH ($SERVICE_BIN)" >&2
    exit 1
  fi
fi

export SLAN_CLIENT_V2_ARCH="$ARCH"
"$PACKAGE_SCRIPT" "${PACKAGE_ARGS[@]}"

TAR_PATH="$OUTPUT_DIR/SLAN-Client-V2-linux-$ARCH.tar.gz"
DEB_PATH="$OUTPUT_DIR/${SLAN_LINUX_PACKAGE_NAME}_${VERSION}_${ARCH}.deb"

if [ ! -f "$TAR_PATH" ]; then
  echo "Missing Linux tarball: $TAR_PATH" >&2
  exit 1
fi

tar -tzf "$TAR_PATH" | grep -q 'opt/slan-client-v2/bin/client-core-service'
tar -tzf "$TAR_PATH" | grep -q 'usr/bin/slan-client-v2-console'
tar -tzf "$TAR_PATH" | grep -q 'usr/lib/systemd/system/slan-client-v2.service'
if [ "$VARIANT" != "console" ]; then
  tar -tzf "$TAR_PATH" | grep -q 'opt/slan-client-v2/gui/'
fi

if [ -f "$DEB_PATH" ]; then
  if command -v dpkg-deb >/dev/null 2>&1; then
    dpkg-deb -c "$DEB_PATH" | grep -q 'opt/slan-client-v2/bin/client-core-service'
    dpkg-deb -c "$DEB_PATH" | grep -q 'usr/lib/systemd/system/slan-client-v2.service'
  fi
  echo "linuxDeb: $DEB_PATH"
fi

echo "linuxTarball: $TAR_PATH"
echo "linuxStage: $OUTPUT_DIR/stage/root"
