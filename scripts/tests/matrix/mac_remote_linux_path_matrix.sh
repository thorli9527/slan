#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE:-$0}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$SCRIPT_PATH")" && pwd)
ROOT_DIR="$SCRIPT_DIR"
while [ ! -e "$ROOT_DIR/.git" ] && [ "$ROOT_DIR" != "/" ]; do
  ROOT_DIR=$(dirname "$ROOT_DIR")
done

MODES="${SLAN_PATH_MATRIX_MODES:-direct,udp-relay,tcp-relay}"

run_mode() {
  local mode="$1"
  local force_relay_only="$2"
  local transport_allowlist="$3"
  local expected_path="$4"
  local skip_udp_checks="${5:-0}"
  echo "==> Mac/remote Linux path mode: $mode expected=$expected_path"
  env \
    SLAN_PATH_MODE="$mode" \
    SLAN_FORCE_RELAY_ONLY="$force_relay_only" \
    SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST="$transport_allowlist" \
    SLAN_EXPECT_PATH_KIND="$expected_path" \
    SLAN_SKIP_UDP_CHECKS="$skip_udp_checks" \
    SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK=0 \
    SLAN_REMOTE_LINUX_BUILD_PACKAGE_REMOTE=0 \
    bash "$ROOT_DIR/scripts/tests/matrix/mac_remote_linux_integration.sh"
}

IFS=',' read -r -a mode_list <<<"$MODES"
for mode in "${mode_list[@]}"; do
  mode="$(printf '%s' "$mode" | tr -d '[:space:]')"
  case "$mode" in
    direct) run_mode direct 0 "" direct_udp 0 ;;
    udp-relay) run_mode udp-relay 1 udp relay_udp 0 ;;
    tcp-relay) run_mode tcp-relay 1 derp_tcp_tls_443 derp_tcp_tls_443 1 ;;
    "") ;;
    *) echo "unknown path matrix mode: $mode" >&2; exit 2 ;;
  esac
done

echo "macRemoteLinuxPathMatrix: ok modes=$MODES"
