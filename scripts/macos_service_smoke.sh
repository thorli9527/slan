#!/usr/bin/env bash
set -euo pipefail

LABEL="dev.slan.client-core-service"
APP_PATH="${SLAN_MACOS_APP_PATH:-client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app}"
SERVICE_IN_APP="${APP_PATH%/}/Contents/MacOS/client-core-service"
SESSION_FILE="/Library/Application Support/SLAN/client-v2-session.json"
SHUTDOWN_CHECK=0

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

if ! launchctl print "system/${LABEL}" >/dev/null 2>&1; then
  echo "launchdLoaded: false"
  echo "install with: scripts/install_macos_service.sh --app \"$APP_PATH\""
  exit 0
fi

echo "launchdLoaded: true"
./scripts/status_macos_service.sh >/tmp/slan-macos-service-status.$$
sed -n '1,120p' /tmp/slan-macos-service-status.$$
rm -f /tmp/slan-macos-service-status.$$

if ! pgrep -f "client-core-service" >/dev/null; then
  echo "client-core-service process not found" >&2
  exit 1
fi

python3 - <<'PY'
import json
import socket

payload = json.dumps({
    "method": "localStateWatch",
    "args": {"lastRevision": 0, "timeoutMs": 1000},
}) + "\n"
with socket.create_connection(("127.0.0.1", 46392), timeout=2) as sock:
    sock.sendall(payload.encode())
    line = sock.makefile("r", encoding="utf-8").readline()
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

payload = json.dumps({"method": "localNetworkShutdown", "args": {}}) + "\n"
with socket.create_connection(("127.0.0.1", 46392), timeout=2) as sock:
    sock.sendall(payload.encode())
    line = sock.makefile("r", encoding="utf-8").readline()
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
  if ! pgrep -f "client-core-service" >/dev/null; then
    echo "client-core-service stopped after localNetworkShutdown" >&2
    exit 1
  fi
  echo "serviceStillRunningAfterShutdown: true"
fi
