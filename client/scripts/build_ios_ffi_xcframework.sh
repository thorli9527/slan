#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUST_DIR="${ROOT_DIR}/rust"
OUT_DIR="${ROOT_DIR}/plugins/client_core_plugin/ios/Frameworks"
FRAMEWORK_NAME="ClientCoreFfi"
XCFRAMEWORK_PATH="${OUT_DIR}/${FRAMEWORK_NAME}.xcframework"
HEADER_DIR="${OUT_DIR}/Headers"
HEADER_PATH="${HEADER_DIR}/client_core_ffi.h"

TARGET_DEVICE="aarch64-apple-ios"
TARGET_SIM_ARM="aarch64-apple-ios-sim"
TARGET_SIM_X86="x86_64-apple-ios"

require_target() {
  local target="$1"
  if ! rustup target list --installed | grep -qx "${target}"; then
    echo "missing Rust target: ${target}" >&2
    echo "install with: rustup target add ${target}" >&2
    exit 1
  fi
}

require_target "${TARGET_DEVICE}"
require_target "${TARGET_SIM_ARM}"
if [[ "$(uname -m)" != "arm64" ]]; then
  require_target "${TARGET_SIM_X86}"
fi

mkdir -p "${OUT_DIR}" "${HEADER_DIR}"

cat > "${HEADER_PATH}" <<'HEADER'
#pragma once

#ifdef __cplusplus
extern "C" {
#endif

char *client_core_v2_version(void);
char *client_core_v2_default_state_json(void);
char *client_core_v2_initialize(const char *state_dir);
char *client_core_v2_service_request_json(const char *request_json);
void client_core_v2_free_string(char *value);

#ifdef __cplusplus
}
#endif
HEADER

pushd "${RUST_DIR}" >/dev/null
cargo build -p client-core-ffi --release --target "${TARGET_DEVICE}"
cargo build -p client-core-ffi --release --target "${TARGET_SIM_ARM}"
if [[ "$(uname -m)" != "arm64" ]]; then
  cargo build -p client-core-ffi --release --target "${TARGET_SIM_X86}"
fi
popd >/dev/null

DEVICE_LIB="${RUST_DIR}/target/${TARGET_DEVICE}/release/libclient_core_ffi.a"
SIM_ARM_LIB="${RUST_DIR}/target/${TARGET_SIM_ARM}/release/libclient_core_ffi.a"
SIM_LIB="${SIM_ARM_LIB}"

if [[ "$(uname -m)" != "arm64" ]]; then
  SIM_X86_LIB="${RUST_DIR}/target/${TARGET_SIM_X86}/release/libclient_core_ffi.a"
  UNIVERSAL_SIM_LIB="${OUT_DIR}/libclient_core_ffi_sim.a"
  lipo -create "${SIM_ARM_LIB}" "${SIM_X86_LIB}" -output "${UNIVERSAL_SIM_LIB}"
  SIM_LIB="${UNIVERSAL_SIM_LIB}"
fi

rm -rf "${XCFRAMEWORK_PATH}"
xcodebuild -create-xcframework \
  -library "${DEVICE_LIB}" -headers "${HEADER_DIR}" \
  -library "${SIM_LIB}" -headers "${HEADER_DIR}" \
  -output "${XCFRAMEWORK_PATH}"

echo "created ${XCFRAMEWORK_PATH}"
