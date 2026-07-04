#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
INSTALL_DIR="/Library/Application Support/SLAN"
LOG_DIR="/Library/Logs/SLAN"
PLIST_SYSTEM="/Library/LaunchDaemons/${LABEL}.plist"
PLIST_AGENT="/Library/LaunchAgents/${LABEL}.plist"
SERVICE_BIN="${INSTALL_DIR}/client-core-service"

service_info() {
  local binary="$1"
  local output
  output="$(mktemp)"
  SLAN_CLIENT_CORE_SERVICE_HOST=127.0.0.1:0 "$binary" --service-info >"$output" 2>/dev/null &
  local pid=$!
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if ! kill -0 "$pid" 2>/dev/null; then
      wait "$pid" || true
      local payload
      payload="$(cat "$output")"
      rm -f "$output"
      if [[ "$payload" == \{* ]]; then
        echo "$payload"
      else
        echo "unsupported"
      fi
      return
    fi
    sleep 0.1
  done
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
  rm -f "$output"
  echo "unsupported"
}

echo "label: $LABEL"
echo "systemPlist: $PLIST_SYSTEM"
echo "agentPlist: $PLIST_AGENT"
if [[ -f "$PLIST_SYSTEM" ]]; then
  echo "systemPlistExists: true"
else
  echo "systemPlistExists: false"
fi
if [[ -f "$PLIST_AGENT" ]]; then
  echo "agentPlistExists: true"
else
  echo "agentPlistExists: false"
fi

echo "binary: $SERVICE_BIN"
if [[ -x "$SERVICE_BIN" ]]; then
  echo "binaryExecutable: true"
  echo "binarySha256: $(shasum -a 256 "$SERVICE_BIN" | awk '{print $1}')"
  echo "serviceInfo: $(service_info "$SERVICE_BIN")"
else
  echo "binaryExecutable: false"
fi

echo "stateDir: $INSTALL_DIR"
echo "logDir: $LOG_DIR"

if launchctl print "system/${LABEL}" >/tmp/slan-launchd-system-status.$$ 2>&1; then
  echo "systemLaunchdLoaded: true"
  sed -n '1,80p' /tmp/slan-launchd-system-status.$$
else
  echo "systemLaunchdLoaded: false"
fi
rm -f /tmp/slan-launchd-system-status.$$

CONSOLE_UID="$(stat -f '%u' /dev/console 2>/dev/null || echo "")"
if [[ -n "$CONSOLE_UID" && "$CONSOLE_UID" != "0" ]]; then
  if launchctl print "gui/${CONSOLE_UID}/${LABEL}" >/tmp/slan-launchd-agent-status.$$ 2>&1; then
    echo "agentLaunchdLoaded: true"
    sed -n '1,80p' /tmp/slan-launchd-agent-status.$$
  else
    echo "agentLaunchdLoaded: false"
  fi
  rm -f /tmp/slan-launchd-agent-status.$$
else
  echo "agentLaunchdLoaded: skippedNoGuiUser"
fi

echo "processes:"
pgrep -fl "client-core-service" 2>/dev/null || echo "processListUnavailableOrEmpty: true"
