#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT_PATH="$ROOT_DIR/client/app/macos/Runner.xcodeproj"
DERIVED_DATA_PATH="$ROOT_DIR/.cache/xcode-packet-tunnel-derived"
CLANG_MODULE_CACHE_PATH="$ROOT_DIR/.cache/xcode-clang-module-cache"
SWIFT_MODULE_CACHE_PATH="$ROOT_DIR/.cache/xcode-swift-module-cache"
MODULE_CACHE_DIR="$ROOT_DIR/.cache/xcode-module-cache"
OBJROOT_PATH="$DERIVED_DATA_PATH/Build/Intermediates.noindex"
SYMROOT_PATH="$DERIVED_DATA_PATH/Build/Products"
LOG_DIR="${MACOS_PACKET_TUNNEL_LOG_DIR:-$ROOT_DIR/artifacts/macos-packet-tunnel}"
LOG_PATH="$LOG_DIR/xcodebuild.log"

mkdir -p \
  "$DERIVED_DATA_PATH" \
  "$CLANG_MODULE_CACHE_PATH" \
  "$SWIFT_MODULE_CACHE_PATH" \
  "$MODULE_CACHE_DIR" \
  "$LOG_DIR"

cd "$ROOT_DIR"

rm -f "$LOG_PATH"

xcodebuild \
  -project "$PROJECT_PATH" \
  -target PacketTunnel \
  -configuration Debug \
  CODE_SIGNING_ALLOWED=NO \
  CODE_SIGNING_REQUIRED=NO \
  CLANG_MODULE_CACHE_PATH="$CLANG_MODULE_CACHE_PATH" \
  MODULE_CACHE_DIR="$MODULE_CACHE_DIR" \
  SWIFT_MODULECACHE_PATH="$SWIFT_MODULE_CACHE_PATH" \
  COMPILER_INDEX_STORE_ENABLE=NO \
  OBJROOT="$OBJROOT_PATH" \
  SYMROOT="$SYMROOT_PATH" \
  build | tee "$LOG_PATH"
