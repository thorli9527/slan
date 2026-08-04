#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUST_DIR="$ROOT_DIR/rust"
PLUGIN_DIR="$ROOT_DIR/plugins/client_core_plugin"
if [[ -z "${ANDROID_SDK_ROOT:-}" ]]; then
  if [[ -n "${ANDROID_HOME:-}" ]]; then
    ANDROID_SDK_ROOT="$ANDROID_HOME"
  elif [[ -n "${LOCALAPPDATA:-}" ]]; then
    ANDROID_SDK_ROOT="$LOCALAPPDATA/Android/Sdk"
  else
    ANDROID_SDK_ROOT="$HOME/Library/Android/sdk"
  fi
fi
NDK_VERSION="${SLAN_ANDROID_NDK_VERSION:-}"
if [[ -z "$NDK_VERSION" ]]; then
  while IFS= read -r candidate; do
    if [[ -d "$ANDROID_SDK_ROOT/ndk/$candidate/toolchains/llvm/prebuilt" ]]; then
      NDK_VERSION="$candidate"
      break
    fi
  done < <(ls "$ANDROID_SDK_ROOT/ndk" 2>/dev/null | sort -Vr)
  NDK_VERSION="${NDK_VERSION:-26.3.11579264}"
fi
NDK_HOME="${ANDROID_NDK_HOME:-$ANDROID_SDK_ROOT/ndk/$NDK_VERSION}"
HOST_TAG="$(uname -s)"
case "$HOST_TAG" in
  Darwin) PREBUILT_TAG="darwin-x86_64" ;;
  MINGW*|MSYS*|CYGWIN*) PREBUILT_TAG="windows-x86_64" ;;
  Linux) PREBUILT_TAG="linux-x86_64" ;;
  *) PREBUILT_TAG="darwin-x86_64" ;;
esac
TOOLCHAIN_BIN="$NDK_HOME/toolchains/llvm/prebuilt/$PREBUILT_TAG/bin"
CLANG_EXT=""
if [[ "$HOST_TAG" == MINGW* || "$HOST_TAG" == MSYS* || "$HOST_TAG" == CYGWIN* ]]; then
  CLANG_EXT=".cmd"
fi
TARGET="aarch64-linux-android"
API_LEVEL="${SLAN_ANDROID_API_LEVEL:-34}"
ABI="arm64-v8a"

if [[ ! -f "$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang${CLANG_EXT}" ]]; then
  echo "Android NDK clang is missing: $TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang${CLANG_EXT}" >&2
  exit 1
fi
 
rustup target add "$TARGET"

(
  cd "$RUST_DIR"
  CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER="$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang${CLANG_EXT}" \
    AR_aarch64_linux_android="$TOOLCHAIN_BIN/llvm-ar${CLANG_EXT}" \
    CC_aarch64_linux_android="$TOOLCHAIN_BIN/${TARGET}${API_LEVEL}-clang${CLANG_EXT}" \
    cargo build -p client-core-ffi --release --target "$TARGET"
)

mkdir -p "$PLUGIN_DIR/android/src/main/jniLibs/$ABI"
cp "$RUST_DIR/target/$TARGET/release/libclient_core_ffi.so" \
  "$PLUGIN_DIR/android/src/main/jniLibs/$ABI/libclient_core_ffi.so"

echo "created $PLUGIN_DIR/android/src/main/jniLibs/$ABI/libclient_core_ffi.so"
