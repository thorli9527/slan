#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1
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
for tool in pkgutil mkbom gzip cpio; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "$tool not found; install Xcode command line tools" >&2
    exit 1
  fi
done

strip_appledouble_payload() {
  local pkg_path="$1"
  local repack_dir="$STAGE_DIR/repack"
  local expanded_pkg="$repack_dir/expanded"
  local payload_root="$repack_dir/payload-root"
  local clean_pkg="$repack_dir/clean.pkg"

  rm -rf "$repack_dir"
  mkdir -p "$payload_root"
  pkgutil --expand "$pkg_path" "$expanded_pkg"
  find "$expanded_pkg" \( -name '._*' -o -name '.DS_Store' \) -delete
  (
    cd "$payload_root"
    gzip -dc "$expanded_pkg/Payload" | cpio -idm --quiet
    find . \( -name '._*' -o -name '.DS_Store' \) -delete
    mkbom . "$expanded_pkg/Bom"
    find . | cpio -o --format odc --quiet | gzip -c > "$expanded_pkg/Payload"
  )
  pkgutil --flatten "$expanded_pkg" "$clean_pkg"
  mv "$clean_pkg" "$pkg_path"
  rm -rf "$repack_dir"
}

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

STALE_SOURCE="$(find \
  "$ROOT_DIR/client_v2/rust/crates" \
  "$ROOT_DIR/client_v2/app_flutter/lib" \
  "$ROOT_DIR/client_v2/plugins/client_core_plugin" \
  "$ROOT_DIR/client_v2/app_flutter/macos" \
  -type f \
  \( -name '*.rs' -o -name '*.dart' -o -name '*.swift' -o -name '*.cc' -o -name '*.cpp' -o -name '*.h' -o -name '*.yaml' -o -name '*.plist' \) \
  -newer "$SERVICE_IN_APP" \
  -print \
  -quit)"
if [[ -n "$STALE_SOURCE" ]]; then
  echo "macOS Release app is older than source: $STALE_SOURCE" >&2
  echo "run: make client-macos-package" >&2
  exit 1
fi

rm -rf "$STAGE_DIR"
mkdir -p "$ROOT_STAGE/Applications" "$ROOT_STAGE/Library/Application Support/SLAN" "$SCRIPT_STAGE" "$OUTPUT_DIR"

ditto --norsrc --noextattr --noqtn --noacl "$APP_PATH" "$ROOT_STAGE/Applications/SLAN Client V2.app"
ditto --norsrc --noextattr --noqtn --noacl "$SERVICE_IN_APP" "$ROOT_STAGE/Library/Application Support/SLAN/client-core-service"
chmod 755 "$ROOT_STAGE/Library/Application Support/SLAN/client-core-service"
find "$ROOT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
if command -v xattr >/dev/null 2>&1; then
  xattr -cr "$ROOT_STAGE" >/dev/null 2>&1 || true
  find "$ROOT_STAGE" -exec xattr -d com.apple.provenance {} \; >/dev/null 2>&1 || true
fi

cp "$ROOT_DIR/client_v2/install/macos/scripts/preinstall" "$SCRIPT_STAGE/preinstall"
cp "$ROOT_DIR/client_v2/install/macos/scripts/postinstall" "$SCRIPT_STAGE/postinstall"
chmod 755 "$SCRIPT_STAGE/preinstall" "$SCRIPT_STAGE/postinstall"
find "$SCRIPT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
if command -v xattr >/dev/null 2>&1; then
  xattr -cr "$SCRIPT_STAGE" >/dev/null 2>&1 || true
fi

rm -f "$PKG_PATH"
find "$ROOT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
find "$SCRIPT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
pkgbuild --analyze --root "$ROOT_STAGE" "$COMPONENT_PLIST"
plutil -replace 0.BundleIsRelocatable -bool NO "$COMPONENT_PLIST"
plutil -replace 0.BundleOverwriteAction -string upgrade "$COMPONENT_PLIST"
find "$ROOT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
find "$SCRIPT_STAGE" \( -name '._*' -o -name '.DS_Store' \) -delete
pkgbuild \
  --root "$ROOT_STAGE" \
  --scripts "$SCRIPT_STAGE" \
  --component-plist "$COMPONENT_PLIST" \
  --identifier "$PACKAGE_ID" \
  --version "$VERSION" \
  --install-location / \
  "$PKG_PATH"

strip_appledouble_payload "$PKG_PATH"

echo "macosPackage: $PKG_PATH"
echo "app: /Applications/SLAN Client V2.app"
echo "service: /Library/Application Support/SLAN/client-core-service"
