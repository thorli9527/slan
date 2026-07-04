#!/usr/bin/env bash
set -euo pipefail

ADB="${SLAN_ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
DEVICE_A="${SLAN_ANDROID_DEVICE_A:-emulator-5554}"
DEVICE_B="${SLAN_ANDROID_DEVICE_B:-emulator-5556}"

stop_if_present() {
  local device="$1"
  if ! "$ADB" devices | grep -q "^${device}[[:space:]]"; then
    echo "==> ${device} not present"
    return 0
  fi
  echo "==> stop ${device}"
  "$ADB" -s "$device" emu kill >/dev/null
}

stop_if_present "$DEVICE_A"
stop_if_present "$DEVICE_B"

echo "==> android dual AVD stop requested: ${DEVICE_A}, ${DEVICE_B}"
