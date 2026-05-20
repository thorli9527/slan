#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
INSTALL_DIR="/Library/Application Support/SLAN"
LOG_DIR="/Library/Logs/SLAN"
PLIST="/Library/LaunchDaemons/${LABEL}.plist"
SERVICE_BIN="${INSTALL_DIR}/client-core-service"
SERVICE_PID="${INSTALL_DIR}/client-core-service.pid"
SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46392}"
CONTROL_BASE_URL="${SLAN_CONTROL_BASE_URL:-}"
MACOS_NETWORK_MOCK="${SLAN_MACOS_NETWORK_MOCK:-0}"
APP_PATH=""
SOURCE_BIN=""
ORIGINAL_ARGS=("$@")

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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)
      APP_PATH="${2:-}"
      shift 2
      ;;
    --binary)
      SOURCE_BIN="${2:-}"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "$SOURCE_BIN" && -n "$APP_PATH" ]]; then
  SOURCE_BIN="${APP_PATH%/}/Contents/MacOS/client-core-service"
fi

if [[ -z "$SOURCE_BIN" ]]; then
  SOURCE_BIN="client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app/Contents/MacOS/client-core-service"
fi

if [[ ! -x "$SOURCE_BIN" ]]; then
  echo "client-core-service source binary not executable: $SOURCE_BIN" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi
echo "sourceServiceInfo: $(service_info "$SOURCE_BIN")"

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo "$0" ${ORIGINAL_ARGS[@]+"${ORIGINAL_ARGS[@]}"}
fi

mkdir -p "$INSTALL_DIR" "$LOG_DIR"
launchctl bootout "system/${LABEL}" >/dev/null 2>&1 || true
launchctl disable "system/${LABEL}" >/dev/null 2>&1 || true
pkill -x "client-core-service" >/dev/null 2>&1 || true
rm -f "$PLIST" "$SERVICE_BIN" "$SERVICE_PID"

cp "$SOURCE_BIN" "$SERVICE_BIN"
chown root:wheel "$SERVICE_BIN"
chmod 755 "$SERVICE_BIN"
chown root:wheel "$INSTALL_DIR" "$LOG_DIR"
chmod 755 "$INSTALL_DIR" "$LOG_DIR"

DEVICE_ID="$("$SERVICE_BIN" --ensure-device-id)"
echo "deviceId: $DEVICE_ID"

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${SERVICE_BIN}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>${INSTALL_DIR}</string>
  <key>StandardOutPath</key>
  <string>${LOG_DIR}/client-core-service.stdout.log</string>
  <key>StandardErrorPath</key>
  <string>${LOG_DIR}/client-core-service.stderr.log</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SLAN_CLIENT_CORE_SERVICE_HOST</key>
    <string>${SERVICE_HOST}</string>
    <key>SLAN_MACOS_NETWORK_MOCK</key>
    <string>${MACOS_NETWORK_MOCK}</string>
$(if [[ -n "$CONTROL_BASE_URL" ]]; then
  cat <<ENV
    <key>SLAN_CONTROL_BASE_URL</key>
    <string>${CONTROL_BASE_URL}</string>
ENV
fi)
  </dict>
</dict>
</plist>
PLIST

chown root:wheel "$PLIST"
chmod 644 "$PLIST"

launchctl enable "system/${LABEL}" >/dev/null 2>&1 || true
launchctl bootstrap system "$PLIST"
launchctl kickstart -k "system/${LABEL}"

echo "installed ${LABEL}"
echo "binary: $SERVICE_BIN"
echo "serviceInfo: $(service_info "$SERVICE_BIN")"
echo "host: $SERVICE_HOST"
[[ -n "$CONTROL_BASE_URL" ]] && echo "controlBaseUrl: $CONTROL_BASE_URL"
echo "plist: $PLIST"
echo "logs: $LOG_DIR"
