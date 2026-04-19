#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_DIR="$ROOT_DIR/client/app"

cleanup_pattern() {
  local pattern="$1"
  pkill -f "$pattern" >/dev/null 2>&1 || true
}

wait_until_gone() {
  local pattern="$1"
  local attempts=0
  while pgrep -f "$pattern" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [[ "$attempts" -eq 10 ]]; then
      pkill -9 -f "$pattern" >/dev/null 2>&1 || true
    fi
    if [[ "$attempts" -ge 20 ]]; then
      echo "timed out waiting for processes matching [$pattern] to exit" >&2
      return 1
    fi
    sleep 1
  done
}

FLUTTER_TEST_PATTERN='flutter_tools.snapshot test integration_test/devices_'
SLAN_APP_PATTERN="$APP_DIR/build/macos/Build/Products/Debug/slan_app.app/Contents/MacOS/slan_app"

# Kill lingering Flutter integration runners for the split devices suites.
cleanup_pattern "$FLUTTER_TEST_PATTERN"

# Kill lingering macOS app hosts spawned by Flutter integration tests.
cleanup_pattern "$SLAN_APP_PATTERN"

wait_until_gone "$FLUTTER_TEST_PATTERN"
wait_until_gone "$SLAN_APP_PATTERN"
