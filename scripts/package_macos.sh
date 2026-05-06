#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
APP_PATH="${SLAN_MACOS_APP_PATH:-$ROOT_DIR/client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
OUTPUT_DIR="${SLAN_MACOS_PACKAGE_OUTPUT_DIR:-$ROOT_DIR/client_v2/.tmp/installer/macos}"
PKG_PATH="$OUTPUT_DIR/SLAN-Client-V2-macos.pkg"
STAGE_DIR="$OUTPUT_DIR/stage"
ROOT_STAGE="$STAGE_DIR/root"
SCRIPT_STAGE="$STAGE_DIR/scripts"
COMPONENT_PLIST="$STAGE_DIR/components.plist"
PACKAGE_ID="${SLAN_MACOS_PACKAGE_ID:-dev.slan.client-v2}"
VERSION="${SLAN_MACOS_PACKAGE_VERSION:-0.1.0}"
SERVICE_IN_APP="${APP_PATH%/}/Contents/MacOS/client-core-service"

if ! command -v pkgbuild >/dev/null 2>&1; then
  echo "pkgbuild not found; install Xcode command line tools" >&2
  exit 1
fi
if ! command -v ditto >/dev/null 2>&1; then
  echo "ditto not found" >&2
  exit 1
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "missing app bundle: $APP_PATH" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi

if [[ ! -x "$SERVICE_IN_APP" ]]; then
  echo "missing bundled client-core-service: $SERVICE_IN_APP" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi

rm -rf "$STAGE_DIR"
mkdir -p "$ROOT_STAGE/Applications" "$ROOT_STAGE/Library/Application Support/SLAN" "$SCRIPT_STAGE" "$OUTPUT_DIR"

ditto --norsrc "$APP_PATH" "$ROOT_STAGE/Applications/SLAN Client V2.app"
cp "$SERVICE_IN_APP" "$ROOT_STAGE/Library/Application Support/SLAN/client-core-service"
chmod 755 "$ROOT_STAGE/Library/Application Support/SLAN/client-core-service"
find "$ROOT_STAGE" -name '._*' -delete
if command -v xattr >/dev/null 2>&1; then
  xattr -cr "$ROOT_STAGE" >/dev/null 2>&1 || true
fi

cp "$ROOT_DIR/client_v2/install/macos/scripts/preinstall" "$SCRIPT_STAGE/preinstall"
cp "$ROOT_DIR/client_v2/install/macos/scripts/postinstall" "$SCRIPT_STAGE/postinstall"
chmod 755 "$SCRIPT_STAGE/preinstall" "$SCRIPT_STAGE/postinstall"

rm -f "$PKG_PATH"
pkgbuild --analyze --root "$ROOT_STAGE" "$COMPONENT_PLIST"
plutil -replace 0.BundleIsRelocatable -bool NO "$COMPONENT_PLIST"
plutil -replace 0.BundleOverwriteAction -string upgrade "$COMPONENT_PLIST"
COPYFILE_DISABLE=1 pkgbuild \
  --root "$ROOT_STAGE" \
  --scripts "$SCRIPT_STAGE" \
  --component-plist "$COMPONENT_PLIST" \
  --identifier "$PACKAGE_ID" \
  --version "$VERSION" \
  --install-location / \
  "$PKG_PATH"

echo "macosPackage: $PKG_PATH"
echo "app: /Applications/SLAN Client V2.app"
echo "service: /Library/Application Support/SLAN/client-core-service"
