#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_DIR="$ROOT_DIR/client/app"
CLEANUP_SCRIPT="$ROOT_DIR/scripts/cleanup_devices_integration.sh"
SIGNING_CHECK_SCRIPT="$ROOT_DIR/scripts/check_macos_packet_tunnel_signing.sh"
LOG_DIR="${DEVICES_INTEGRATION_LOG_DIR:-$ROOT_DIR/artifacts/devices-integration}"

trap '"$CLEANUP_SCRIPT"' EXIT

cd "$APP_DIR"
"$SIGNING_CHECK_SCRIPT"
"$CLEANUP_SCRIPT"
mkdir -p "$LOG_DIR"

for test_file in integration_test/devices_*_flow_test.dart; do
  "$CLEANUP_SCRIPT"
  echo "==> flutter test $test_file"
  log_file="$LOG_DIR/$(basename "$test_file" .dart).log"
  flutter test "$test_file" 2>&1 | tee "$log_file"
  "$CLEANUP_SCRIPT"
done
