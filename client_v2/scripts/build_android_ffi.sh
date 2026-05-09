#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUST_DIR="$ROOT_DIR/rust"
PLUGIN_DIR="$ROOT_DIR/plugins/client_core_plugin"
ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$HOME/Library/Android/sdk}}"
NDK_VERSION="${SLAN_ANDROID_NDK_VERSION:-26.3.11579264}"
NDK_HOME="${ANDROID_NDK_HOME:-$ANDROID_SDK_ROOT/ndk/$NDK_VERSION}"
TOOLCHAIN_BIN="$NDK_HOME/toolchains/llvm/prebuilt/darwin-x86_64/bin"
TARGET="aarch64-linux-android"
API_LEVEL="${SLAN_ANDROID_API_LEVEL:-34}"
ABI="arm64-v8a"

if [[ ! -x "$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang" ]]; then
  echo "Android NDK clang is missing: $TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang" >&2
  exit 1
fi

rustup target add "$TARGET"

(
  cd "$RUST_DIR"
  CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER="$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang" \
    AR_aarch64_linux_android="$TOOLCHAIN_BIN/llvm-ar" \
    CC_aarch64_linux_android="$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang" \
    cargo build -p client-core-ffi --release --target "$TARGET"
)

mkdir -p "$PLUGIN_DIR/android/src/main/jniLibs/$ABI"
cp "$RUST_DIR/target/$TARGET/release/libclient_core_ffi.so" \
  "$PLUGIN_DIR/android/src/main/jniLibs/$ABI/libclient_core_ffi.so"

echo "created $PLUGIN_DIR/android/src/main/jniLibs/$ABI/libclient_core_ffi.so"
