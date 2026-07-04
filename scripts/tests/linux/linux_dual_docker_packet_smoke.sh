#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

export SLAN_LINUX_DUAL_PACKET_TESTS="${SLAN_LINUX_DUAL_PACKET_TESTS:-1}"
export SLAN_LINUX_NETWORK_MOCK="${SLAN_LINUX_NETWORK_MOCK:-0}"
export SLAN_LINUX_DUAL_CONTAINER_PRIVILEGED="${SLAN_LINUX_DUAL_CONTAINER_PRIVILEGED:-1}"
export SLAN_LINUX_DUAL_BUILD_PACKAGE="${SLAN_LINUX_DUAL_BUILD_PACKAGE:-0}"

exec bash "$ROOT_DIR/scripts/linux_dual_docker_integration.sh"
