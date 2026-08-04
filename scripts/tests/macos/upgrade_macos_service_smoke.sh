#!/usr/bin/env bash
set -euo pipefail

APP_PATH="${SLAN_MACOS_APP_PATH:-client/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
SERVICE_IN_APP="${APP_PATH%/}/Contents/MacOS/client-core-service"
INSTALLED_SERVICE="/Library/Application Support/SLAN/client-core-service"
CONFIG_FILE="/Library/Preferences/dev.slan.client-core-service/SLAN/config.json"

if [[ ! -x "$SERVICE_IN_APP" ]]; then
  echo "missing bundled client-core-service: $SERVICE_IN_APP" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi

before_hash=""
config_hash_before=""
if [[ -x "$INSTALLED_SERVICE" ]]; then
  before_hash="$(shasum -a 256 "$INSTALLED_SERVICE" | awk '{print $1}')"
fi
if [[ -f "$CONFIG_FILE" ]]; then
  config_hash_before="$(shasum -a 256 "$CONFIG_FILE" | awk '{print $1}')"
fi
bundle_hash="$(shasum -a 256 "$SERVICE_IN_APP" | awk '{print $1}')"

echo "appBundle: $APP_PATH"
echo "bundledServiceSha256: $bundle_hash"
if [[ -n "$before_hash" ]]; then
  echo "installedServiceSha256Before: $before_hash"
else
  echo "installedServiceSha256Before:"
fi

if [[ "$before_hash" == "$bundle_hash" ]]; then
  echo "installSkipped: already up to date"
else
  scripts/install_macos_service.sh --app "$APP_PATH"
fi

after_hash="$(shasum -a 256 "$INSTALLED_SERVICE" | awk '{print $1}')"
echo "installedServiceSha256After: $after_hash"
if [[ "$after_hash" != "$bundle_hash" ]]; then
  echo "installed service hash does not match bundled service after upgrade" >&2
  exit 1
fi
if [[ -n "$config_hash_before" ]]; then
  config_hash_after="$(shasum -a 256 "$CONFIG_FILE" | awk '{print $1}')"
  if [[ "$config_hash_after" != "$config_hash_before" ]]; then
    echo "client config changed during upgrade: $CONFIG_FILE" >&2
    exit 1
  fi
  echo "clientConfigPreserved: true"
fi
echo "installedServiceUpdated: true"

scripts/macos_service_smoke.sh --shutdown-check
