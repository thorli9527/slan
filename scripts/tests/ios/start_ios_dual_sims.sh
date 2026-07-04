#!/usr/bin/env bash
set -euo pipefail

SIM_A_NAME="${SLAN_IOS_SIM_A_NAME:-iPhone 17 Pro}"
SIM_B_NAME="${SLAN_IOS_SIM_B_NAME:-SLAN iPhone 16 Pro Clean 26.5}"

boot_if_needed() {
  local name="$1"
  if xcrun simctl list devices booted | grep -Fq "$name"; then
    echo "==> ${name} already booted"
    return 0
  fi
  echo "==> boot ${name}"
  xcrun simctl boot "$name"
}

wait_booted() {
  local name="$1"
  for _ in $(seq 1 60); do
    if xcrun simctl list devices booted | grep -Fq "$name"; then
      echo "==> ${name} booted"
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for iOS simulator to boot: ${name}" >&2
  return 1
}

boot_if_needed "$SIM_A_NAME"
boot_if_needed "$SIM_B_NAME"

wait_booted "$SIM_A_NAME"
wait_booted "$SIM_B_NAME"

echo "==> iOS dual simulators ready: ${SIM_A_NAME}, ${SIM_B_NAME}"
