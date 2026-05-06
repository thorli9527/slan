#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
INSTALL_DIR="/Library/Application Support/SLAN"
LOG_DIR="/Library/Logs/SLAN"
PLIST="/Library/LaunchDaemons/${LABEL}.plist"
PURGE=0

if [[ "${1:-}" == "--purge" ]]; then
  PURGE=1
fi

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo "$0" "$@"
fi

launchctl bootout "system/${LABEL}" >/dev/null 2>&1 || true
rm -f "$PLIST"
rm -f "${INSTALL_DIR}/client-core-service"

if [[ "$PURGE" == "1" ]]; then
  rm -rf "$INSTALL_DIR" "$LOG_DIR"
fi

echo "uninstalled ${LABEL}"
if [[ "$PURGE" != "1" ]]; then
  echo "state/log directories kept: $INSTALL_DIR, $LOG_DIR"
fi
