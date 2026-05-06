#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
APP_PATH="${SLAN_MACOS_APP_PATH:-client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
SERVICE_IN_APP="${APP_PATH%/}/Contents/MacOS/client-core-service"
SESSION_FILE="/Library/Application Support/SLAN/client-v2-session.json"
SHUTDOWN_CHECK=0

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

if [[ "${1:-}" == "--shutdown-check" ]]; then
  SHUTDOWN_CHECK=1
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "missing app bundle: $APP_PATH" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi

if [[ ! -x "$SERVICE_IN_APP" ]]; then
  echo "missing bundled client-core-service: $SERVICE_IN_APP" >&2
  echo "run: make client-macos-build" >&2
  exit 1
fi

echo "appBundle: $APP_PATH"
echo "bundledService: $SERVICE_IN_APP"
echo "bundledServiceSha256: $(shasum -a 256 "$SERVICE_IN_APP" | awk '{print $1}')"
echo "bundledServiceInfo: $(service_info "$SERVICE_IN_APP")"

if ! launchctl print "system/${LABEL}" >/dev/null 2>&1; then
  echo "launchdLoaded: false"
  echo "install with: scripts/install_macos_service.sh --app \"$APP_PATH\""
  exit 0
fi

INSTALLED_SERVICE="/Library/Application Support/SLAN/client-core-service"
if [[ ! -x "$INSTALLED_SERVICE" ]]; then
  echo "installed service binary missing: $INSTALLED_SERVICE" >&2
  exit 1
fi
bundled_hash="$(shasum -a 256 "$SERVICE_IN_APP" | awk '{print $1}')"
installed_hash="$(shasum -a 256 "$INSTALLED_SERVICE" | awk '{print $1}')"
echo "installedServiceSha256: $installed_hash"
if [[ "$bundled_hash" != "$installed_hash" ]]; then
  echo "installed service does not match app bundle service" >&2
  echo "reinstall with: scripts/install_macos_service.sh --app \"$APP_PATH\"" >&2
  exit 1
fi
echo "installedServiceInfo: $(service_info "$INSTALLED_SERVICE")"
echo "installedServiceMatchesBundle: true"

echo "launchdLoaded: true"
LAUNCHD_STATUS="/tmp/slan-macos-service-status.$$"
./scripts/status_macos_service.sh >"$LAUNCHD_STATUS"
sed -n '1,140p' "$LAUNCHD_STATUS"

if ! grep -q "state = running" "$LAUNCHD_STATUS"; then
  rm -f "$LAUNCHD_STATUS"
  echo "launchd service is not running" >&2
  exit 1
fi

if ! awk '/^[[:space:]]*pid = / { found=1 } END { exit found ? 0 : 1 }' "$LAUNCHD_STATUS"; then
  rm -f "$LAUNCHD_STATUS"
  echo "launchd service has no pid" >&2
  exit 1
fi
rm -f "$LAUNCHD_STATUS"

python3 - <<'PY'
import json
import os
import socket
import sys

payload = json.dumps({
    "method": "localStateWatch",
    "args": {"lastRevision": 0, "timeoutMs": 1000},
}) + "\n"
try:
    with socket.create_connection(("127.0.0.1", 46392), timeout=2) as sock:
        sock.sendall(payload.encode())
        line = sock.makefile("r", encoding="utf-8").readline()
except PermissionError as error:
    print(f"localStateWatch: skipped permission error: {error}")
    sys.exit(0)
if not line:
    raise SystemExit("empty localStateWatch response")
data = json.loads(line)
if "revision" not in data or "state" not in data:
    raise SystemExit(f"invalid localStateWatch response: {data!r}")
print(f"localStateWatch: revision={data['revision']}")
PY

if [[ "$SHUTDOWN_CHECK" == "1" ]]; then
  before=""
  if [[ -f "$SESSION_FILE" ]]; then
    before="$(shasum -a 256 "$SESSION_FILE" | awk '{print $1}')"
  fi
  python3 - <<'PY'
import json
import socket
import sys

payload = json.dumps({"method": "localNetworkShutdown", "args": {}}) + "\n"
try:
    with socket.create_connection(("127.0.0.1", 46392), timeout=2) as sock:
        sock.sendall(payload.encode())
        line = sock.makefile("r", encoding="utf-8").readline()
except PermissionError as error:
    print(f"localNetworkShutdown: skipped permission error: {error}")
    sys.exit(0)
if not line:
    raise SystemExit("empty localNetworkShutdown response")
data = json.loads(line)
if data.get("networkEnabled") is True:
    raise SystemExit(f"shutdown did not disable network: {data!r}")
print("localNetworkShutdown: ok")
PY
  if [[ -n "$before" ]]; then
    after="$(shasum -a 256 "$SESSION_FILE" | awk '{print $1}')"
    if [[ "$before" != "$after" ]]; then
      echo "session file changed after localNetworkShutdown: $SESSION_FILE" >&2
      exit 1
    fi
    echo "sessionPreserved: true"
  fi
  if ! launchctl print "system/${LABEL}" | grep -q "state = running"; then
    echo "launchd service stopped after localNetworkShutdown" >&2
    exit 1
  fi
  echo "serviceStillRunningAfterShutdown: true"
fi
