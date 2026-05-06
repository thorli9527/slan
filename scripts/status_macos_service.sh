#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
INSTALL_DIR="/Library/Application Support/SLAN"
LOG_DIR="/Library/Logs/SLAN"
PLIST="/Library/LaunchDaemons/${LABEL}.plist"
SERVICE_BIN="${INSTALL_DIR}/client-core-service"

echo "label: $LABEL"
echo "plist: $PLIST"
if [[ -f "$PLIST" ]]; then
  echo "plistExists: true"
else
  echo "plistExists: false"
fi

echo "binary: $SERVICE_BIN"
if [[ -x "$SERVICE_BIN" ]]; then
  echo "binaryExecutable: true"
else
  echo "binaryExecutable: false"
fi

echo "stateDir: $INSTALL_DIR"
echo "logDir: $LOG_DIR"

if launchctl print "system/${LABEL}" >/tmp/slan-launchd-status.$$ 2>&1; then
  echo "launchdLoaded: true"
  sed -n '1,80p' /tmp/slan-launchd-status.$$
else
  echo "launchdLoaded: false"
  cat /tmp/slan-launchd-status.$$ >&2 || true
fi
rm -f /tmp/slan-launchd-status.$$

echo "processes:"
pgrep -fl "client-core-service" || true
