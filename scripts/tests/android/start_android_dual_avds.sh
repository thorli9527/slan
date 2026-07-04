#!/usr/bin/env bash
set -euo pipefail

ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
EMULATOR_BIN="${SLAN_ANDROID_EMULATOR_BIN:-$HOME/Library/Android/sdk/emulator/emulator}"
AVD_A="${SLAN_ANDROID_AVD_A:-Pixel_3a_API_34_extension_level_7_arm64-v8a}"
AVD_B="${SLAN_ANDROID_AVD_B:-Pixel_3a_API_34_extension_level_7_arm64-v8a_dual2}"
DEVICE_A="${SLAN_ANDROID_DEVICE_A:-emulator-5554}"
DEVICE_B="${SLAN_ANDROID_DEVICE_B:-emulator-5556}"
PORT_A="${SLAN_ANDROID_PORT_A:-5554}"
PORT_B="${SLAN_ANDROID_PORT_B:-5556}"
NO_WINDOW="${SLAN_ANDROID_NO_WINDOW:-1}"
GPU_MODE="${SLAN_ANDROID_GPU_MODE:-swiftshader_indirect}"

emulator_args() {
  local args=(
    -no-snapshot
    -no-snapshot-load
    -no-snapshot-save
    -no-boot-anim
    -no-audio
    -netdelay none
    -netspeed full
    -gpu "$GPU_MODE"
    -camera-back none
    -camera-front none
  )
  if [[ "$NO_WINDOW" == "1" ]]; then
    args+=(-no-window)
  fi
  printf '%s ' "${args[@]}"
}

start_if_missing() {
  local device="$1"
  local avd="$2"
  local port="$3"
  if "$ADB" devices | grep -q "^${device}[[:space:]]"; then
    echo "==> ${device} already present"
    return 0
  fi
  echo "==> start ${device} from AVD ${avd} on port ${port}"
  # Keep the emulator detached from the launching shell so it survives
  # non-interactive script exits during automation.
  # shellcheck disable=SC2086
  nohup "$EMULATOR_BIN" -avd "$avd" -port "$port" $(emulator_args) \
    </dev/null >/tmp/"${device}".log 2>&1 &
}

wait_ready() {
  local device="$1"
  echo "==> wait for ${device}"
  "$ADB" -s "$device" wait-for-device
  for _ in $(seq 1 180); do
    state="$("$ADB" -s "$device" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
    if [[ "$state" == "1" ]]; then
      echo "==> ${device} boot completed"
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for ${device} boot completion" >&2
  return 1
}

start_if_missing "$DEVICE_A" "$AVD_A" "$PORT_A"
start_if_missing "$DEVICE_B" "$AVD_B" "$PORT_B"

wait_ready "$DEVICE_A"
wait_ready "$DEVICE_B"

echo "==> android dual AVDs ready: ${DEVICE_A}/${AVD_A}, ${DEVICE_B}/${AVD_B}"
