#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "$ROOT_DIR/scripts/lib/client_default_endpoints.sh"
APP_DIR="$ROOT_DIR/client/app_flutter"
APP_PATH="${SLAN_MACOS_APP_PATH:-$APP_DIR/build/macos/Build/Products/Release/slan_client_v2.app}"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46392}"
CONTROL_BASE_URL="${SLAN_CONTROL_BASE_URL:-${SLAN_BIZ_URL:-$SLAN_DEFAULT_CONTROL_BASE_URL}}"
MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
RUN_BUILD="${SLAN_SETUP_MACOS_BUILD:-1}"
RUN_PACKAGE="${SLAN_SETUP_MACOS_PACKAGE:-0}"
RUN_INSTALL="${SLAN_SETUP_MACOS_INSTALL:-1}"
RUN_STATUS="${SLAN_SETUP_MACOS_STATUS:-1}"
RUN_MAC_ANDROID_FAST="${SLAN_SETUP_MACOS_RUN_MAC_ANDROID_FAST:-${SLAN_SETUP_MACOS_RUN_MAC_ANDROID_QUICK:-0}}"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/setup_macos_client.sh

Optional environment variables:
  SLAN_SETUP_MACOS_BUILD=1|0
  SLAN_SETUP_MACOS_PACKAGE=1|0
  SLAN_SETUP_MACOS_INSTALL=1|0
  SLAN_SETUP_MACOS_STATUS=1|0
  SLAN_SETUP_MACOS_RUN_MAC_ANDROID_FAST=1|0
  SLAN_MACOS_APP_PATH=/abs/path/to/slan_client_v2.app
  SLAN_BIZ_URL=$SLAN_DEFAULT_CONTROL_BASE_URL
  SLAN_CONTROL_BASE_URL=$SLAN_DEFAULT_CONTROL_BASE_URL
  SLAN_CLIENT_CORE_SERVICE_HOST=127.0.0.1:46392
  SLAN_MACOS_NETWORK_MOCK=0|1

Examples:
  bash scripts/setup_macos_client.sh
  SLAN_SETUP_MACOS_RUN_MAC_ANDROID_FAST=1 bash scripts/setup_macos_client.sh
  SLAN_MACOS_NETWORK_MOCK=1 SLAN_SETUP_MACOS_RUN_MAC_ANDROID_FAST=1 bash scripts/setup_macos_client.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

log() {
  printf '==> %s\n' "$*"
}

build_macos_app() {
  log "Build macOS app bundle with bundled client-core-service"
  (
    cd "$ROOT_DIR"
    make client-macos-build
  )
}

package_macos_app() {
  log "Package macOS installer"
  (
    cd "$ROOT_DIR"
    ./scripts/package_macos.sh
  )
}

install_macos_service() {
  log "Install macOS client-core-service"
  (
    cd "$ROOT_DIR"
    SLAN_CLIENT_CORE_SERVICE_HOST="$SERVICE_HOST" \
      SLAN_CONTROL_BASE_URL="$CONTROL_BASE_URL" \
      SLAN_MACOS_NETWORK_MOCK="$MACOS_NETWORK_MOCK" \
      ./scripts/install_macos_service.sh --app "$APP_PATH"
  )
}

status_macos_service() {
  log "Inspect installed macOS service"
  (
    cd "$ROOT_DIR"
    ./scripts/status_macos_service.sh
  )
}

run_mac_android_fast() {
  log "Run Mac + Android fast validation"
  (
    cd "$ROOT_DIR"
    SLAN_BIZ_URL="$CONTROL_BASE_URL" \
      SLAN_MACOS_NETWORK_MOCK="$MACOS_NETWORK_MOCK" \
      ./scripts/mac_android_fast_check.sh
  )
}

log "setup toggles: build=${RUN_BUILD} package=${RUN_PACKAGE} install=${RUN_INSTALL} status=${RUN_STATUS} mac-android-fast=${RUN_MAC_ANDROID_FAST}"
log "service host: ${SERVICE_HOST}"
log "control base url: ${CONTROL_BASE_URL}"
log "macOS network mock: ${MACOS_NETWORK_MOCK}"

if [[ "$RUN_BUILD" == "1" ]]; then
  build_macos_app
fi

if [[ "$RUN_PACKAGE" == "1" ]]; then
  package_macos_app
fi

if [[ "$RUN_INSTALL" == "1" ]]; then
  install_macos_service
fi

if [[ "$RUN_STATUS" == "1" ]]; then
  status_macos_service
fi

if [[ "$RUN_MAC_ANDROID_FAST" == "1" ]]; then
  run_mac_android_fast
fi

log "macOS client setup complete"
