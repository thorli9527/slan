#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/client/app"
LOG_DIR="${CLIENT_DESKTOP_UI_LOG_DIR:-$ROOT_DIR/artifacts/client-desktop-ui}"
LOG_FILE="$LOG_DIR/flutter-test.log"

mkdir -p "$LOG_DIR"

cd "$APP_DIR"

flutter test \
  test/widget_test.dart \
  test/features/shared/desktop_client_widgets_test.dart \
  test/features/auth/auth_page_desktop_test.dart \
  test/features/networks/networks_page_desktop_test.dart \
  test/features/home/home_page_desktop_test.dart \
  test/features/devices/devices_page_desktop_test.dart \
  | tee "$LOG_FILE"
