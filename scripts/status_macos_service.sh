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
  echo "binarySha256: $(shasum -a 256 "$SERVICE_BIN" | awk '{print $1}')"
else
  echo "binaryExecutable: false"
fi

echo "stateDir: $INSTALL_DIR"
echo "logDir: $LOG_DIR"

if launchctl print "system/${LABEL}" >/tmp/slan-launchd-status.$$ 2>&1; then
  echo "launchdLoaded: true"
  sed -n '1,80p' /tmp/slan-launchd-status.$$
  if awk '/^[[:space:]]*pid = / { found=1; print "launchdPid: "$3 } END { exit found ? 0 : 1 }' /tmp/slan-launchd-status.$$; then
    :
  else
    echo "launchdPid:"
  fi
else
  echo "launchdLoaded: false"
  cat /tmp/slan-launchd-status.$$ >&2 || true
fi
rm -f /tmp/slan-launchd-status.$$

echo "processes:"
pgrep -fl "client-core-service" 2>/dev/null || echo "processListUnavailableOrEmpty: true"
