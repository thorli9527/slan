#!/usr/bin/env bash
set -euo pipefail

SIM_A_NAME="${SLAN_IOS_SIM_A_NAME:-iPhone 17 Pro}"
SIM_B_NAME="${SLAN_IOS_SIM_B_NAME:-SLAN iPhone 16 Pro Clean 26.5}"

shutdown_if_booted() {
  local name="$1"
  if ! xcrun simctl list devices booted | grep -Fq "$name"; then
    echo "==> ${name} not booted"
    return 0
  fi
  echo "==> shutdown ${name}"
  xcrun simctl shutdown "$name"
}

shutdown_if_booted "$SIM_A_NAME"
shutdown_if_booted "$SIM_B_NAME"

echo "==> iOS dual simulator stop requested: ${SIM_A_NAME}, ${SIM_B_NAME}"
