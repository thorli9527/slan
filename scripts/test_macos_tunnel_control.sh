#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_MACOS_DIR="$ROOT_DIR/client/app_core_plugin_macos/macos"
MODULE_CACHE_DIR="$PLUGIN_MACOS_DIR/.build/module-cache"
SCRATCH_DIR="$PLUGIN_MACOS_DIR/.build-spm"
LOG_DIR="${MACOS_TUNNEL_CONTROL_LOG_DIR:-$ROOT_DIR/artifacts/macos-tunnel-control}"
LOG_PATH="$LOG_DIR/swift-test.log"

mkdir -p "$MODULE_CACHE_DIR" "$SCRATCH_DIR" "$LOG_DIR"

cd "$PLUGIN_MACOS_DIR"

env \
  CLANG_MODULE_CACHE_PATH="$MODULE_CACHE_DIR" \
  SWIFTPM_MODULECACHE_OVERRIDE="$MODULE_CACHE_DIR" \
  swift test --scratch-path "$SCRATCH_DIR" | tee "$LOG_PATH"
