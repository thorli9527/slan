#!/usr/bin/env bash
set -euo pipefail

SERVICE_HOST="${SLAN_CLIENT_CORE_SERVICE_HOST:-127.0.0.1:46392}"
SERVICE_ADDR="${SERVICE_HOST%:*}"
SERVICE_PORT="${SERVICE_HOST##*:}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

request_json() {
  local method="$1"
  local args="${2-}"
  if [[ -z "$args" ]]; then
    args='{}'
  fi
  jq -cn --arg method "$method" --argjson args "$args" '
    {
      method: $method,
      args: $args
    }
  ' | nc -w 5 "$SERVICE_ADDR" "$SERVICE_PORT"
}

wait_local_api() {
  local timeout_seconds="${1:-90}"
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    if request_json localStatus >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "remote linux local api did not become ready" >&2
  return 1
}

password_login() {
  local email="$1"
  local password="$2"
  local payload
  payload="$(jq -cn --arg email "$email" --arg password "$password" '
    {
      type: "loginWithPassword",
      payload: {
        email: $email,
        password: $password
      }
    }
  ')"
  request_json dispatch "$payload"
}

wait_signed_in_or_login() {
  local email="$1"
  local password="$2"
  local timeout_seconds="${3:-120}"
  local deadline=$((SECONDS + timeout_seconds))
  local state_json=''
  while (( SECONDS < deadline )); do
    state_json="$(request_json localState || true)"
    if [[ -n "$state_json" ]] && jq -e --arg email "$email" '
      .signedIn == true and
      (.deviceId // "" | length > 0) and
      (.userLabel // "" | ascii_downcase) == ($email | ascii_downcase)
    ' >/dev/null <<<"$state_json"; then
      printf '%s\n' "$state_json"
      return 0
    fi
    if [[ -n "$state_json" ]] && jq -e --arg email "$email" '
      .signedIn == true and
      (.userLabel // "" | ascii_downcase) != ($email | ascii_downcase)
    ' >/dev/null <<<"$state_json"; then
      request_json localLogout >/dev/null 2>&1 || true
      sleep 1
    fi
    password_login "$email" "$password" >/dev/null 2>&1 || true
    sleep 2
  done
  printf '%s\n' "$state_json"
  echo "remote linux did not reach signed-in state" >&2
  return 1
}

wait_control_ready() {
  local timeout_seconds="${1:-90}"
  local deadline=$((SECONDS + timeout_seconds))
  local status_json=''
  local mqtt_connect_attempted=0
  while (( SECONDS < deadline )); do
    status_json="$(request_json localControlStatus || true)"
    if [[ -n "$status_json" ]] && jq -e '.ready == true' >/dev/null <<<"$status_json"; then
      printf '%s\n' "$status_json"
      return 0
    fi
    if [[ "$mqtt_connect_attempted" != "1" ]] && [[ -n "$status_json" ]] && jq -e '
      (.missing // []) | index("mqtt") != null
    ' >/dev/null <<<"$status_json"; then
      request_json localEnsureDevice >/dev/null 2>&1 || true
      request_json localConnectControlMqtt >/dev/null 2>&1 || true
      mqtt_connect_attempted=1
      continue
    fi
    sleep 1
  done
  printf '%s\n' "$status_json"
  echo "remote linux control transport did not become ready" >&2
  return 1
}

ensure_network_ready() {
  local timeout_seconds="${1:-120}"
  local deadline=$((SECONDS + timeout_seconds))
  local response=''
  while (( SECONDS < deadline )); do
    response="$(request_json localNetworkActivate || true)"
    if [[ -n "$response" ]] && jq -e '
      .networkEnabled == true and
      (.virtualIp // "" | length > 0) and
      ((.error // "") | length == 0)
    ' >/dev/null <<<"$response"; then
      printf '%s\n' "$response"
      return 0
    fi
    response="$(request_json localStatus || true)"
    if [[ -n "$response" ]] && jq -e '
      .networkEnabled == true and
      (.virtualIp // "" | length > 0) and
      ((.error // "") | length == 0)
    ' >/dev/null <<<"$response"; then
      printf '%s\n' "$response"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$response"
  echo "remote linux network did not become ready" >&2
  return 1
}

wait_network_module() {
  local min_peers="$1"
  local min_dns="$2"
  local min_rules="$3"
  local timeout_seconds="${4:-120}"
  local deadline=$((SECONDS + timeout_seconds))
  local module_json='' peer_count=0 dns_count=0 rule_count=0
  while (( SECONDS < deadline )); do
    module_json="$(request_json localNetworkModule || true)"
    if [[ -n "$module_json" ]]; then
      peer_count="$(jq -r '(.peerCount // ([.configs[]?.peers[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      dns_count="$(jq -r '(.resolverRecordCount // ([.configs[]?.resolverRecords[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
      rule_count="$(jq -r '(.securityRuleCount // ([.configs[]?.rules[]?] | length) // 0)' <<<"$module_json" 2>/dev/null || printf '0\n')"
    fi
    if [[ -n "$module_json" ]] && (( peer_count >= min_peers && dns_count >= min_dns && rule_count >= min_rules )); then
      printf '%s\n' "$module_json"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$module_json"
  echo "remote linux network module did not receive expected dns/acl config" >&2
  return 1
}

send_client_message() {
  local target_device_id="$1"
  local body="$2"
  local payload response
  payload="$(jq -cn --arg target_device_id "$target_device_id" --arg body "$body" '
    {
      targetDeviceId: $target_device_id,
      body: $body,
      metadata: {
        smoke: "remote-linux-android"
      }
    }
  ')"
  response="$(request_json localSendClientMessage "$payload")"
  jq -e '(.messageId // "" | length > 0)' >/dev/null <<<"$response" || {
    printf '%s\n' "$response"
    echo "remote linux send client message failed" >&2
    return 1
  }
  printf '%s\n' "$response"
}

wait_client_message() {
  local from_device_id="$1"
  local body="$2"
  local timeout_seconds="${3:-90}"
  local deadline=$((SECONDS + timeout_seconds))
  local response=''
  while (( SECONDS < deadline )); do
    response="$(request_json localState || true)"
    if [[ -n "$response" ]] && jq -e --arg from "$from_device_id" --arg body "$body" '
      .lastClientMessageFromDeviceId == $from and .lastClientMessageBody == $body
    ' >/dev/null <<<"$response"; then
      printf '%s\n' "$response"
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "$response"
  echo "remote linux did not receive client message from ${from_device_id}" >&2
  return 1
}

start_echo_server() {
  local udp_port="$1"
  local tcp_port="$2"
  local log_path="${3:-/tmp/slan-remote-echo.log}"
  pkill -f "python3 - ${udp_port} ${tcp_port}" >/dev/null 2>&1 || true
  rm -f "$log_path"
  nohup python3 - "$udp_port" "$tcp_port" >"$log_path" 2>&1 <<'PY' &
import socket
import sys
import threading
import time

udp_port = int(sys.argv[1])
tcp_port = int(sys.argv[2])

def log(message):
    print(message, flush=True)

def run_udp():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(("0.0.0.0", udp_port))
    sock.settimeout(0.5)
    log(f"SOCKET_ECHO_UDP_READY={udp_port}")
    while True:
      try:
        data, addr = sock.recvfrom(2048)
      except socket.timeout:
        continue
      body = data.decode()
      log(f"SOCKET_ECHO_UDP_RECEIVED={addr[0]}:{addr[1]} body={body}")
      sock.sendto(f"echo:{body}".encode(), addr)

def handle_tcp(conn, addr):
    try:
      conn.settimeout(10)
      data = b""
      while not data.endswith(b"\n"):
        chunk = conn.recv(2048)
        if not chunk:
          break
        data += chunk
      body = data.decode().rstrip("\n")
      log(f"SOCKET_ECHO_TCP_RECEIVED={addr[0]}:{addr[1]} body={body}")
      conn.sendall(f"echo:{body}\n".encode())
      time.sleep(0.2)
    finally:
      conn.close()

def run_tcp():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("0.0.0.0", tcp_port))
    sock.listen()
    sock.settimeout(0.5)
    log(f"SOCKET_ECHO_TCP_READY={tcp_port}")
    while True:
      try:
        conn, addr = sock.accept()
      except socket.timeout:
        continue
      threading.Thread(target=handle_tcp, args=(conn, addr), daemon=True).start()

threading.Thread(target=run_udp, daemon=True).start()
threading.Thread(target=run_tcp, daemon=True).start()
while True:
    time.sleep(3600)
PY
  sleep 1
}

wait_echo_ready() {
  local udp_port="$1"
  local tcp_port="$2"
  local log_path="${3:-/tmp/slan-remote-echo.log}"
  local timeout_seconds="${4:-30}"
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    if grep -q "SOCKET_ECHO_UDP_READY=${udp_port}" "$log_path" 2>/dev/null &&
      grep -q "SOCKET_ECHO_TCP_READY=${tcp_port}" "$log_path" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  cat "$log_path" 2>/dev/null || true
  echo "remote linux echo server did not become ready" >&2
  return 1
}

send_udp_echo() {
  local host="$1"
  local port="$2"
  local body="$3"
  python3 - "$host" "$port" "$body" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
body = sys.argv[3]
expected = f"echo:{body}".encode()

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.settimeout(12)
sock.sendto(body.encode(), (host, port))
data, _ = sock.recvfrom(2048)
sock.close()
if data != expected:
    raise SystemExit(f"unexpected UDP echo response: {data!r} want={expected!r}")
print(f"REMOTE_UDP_ECHO_OK={host}:{port}")
PY
}

send_tcp_echo() {
  local host="$1"
  local port="$2"
  local body="$3"
  python3 - "$host" "$port" "$body" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
body = sys.argv[3]
expected = f"echo:{body}"

sock = socket.create_connection((host, port), timeout=12)
sock.settimeout(12)
sock.sendall((body + "\n").encode())
data = b""
while not data.endswith(b"\n"):
    chunk = sock.recv(2048)
    if not chunk:
        break
    data += chunk
sock.close()
received = data.decode().rstrip("\n")
if received != expected:
    raise SystemExit(f"unexpected TCP echo response: {received!r} want={expected!r}")
print(f"REMOTE_TCP_ECHO_OK={host}:{port}")
PY
}

need jq
need nc
need python3

command="${1:-}"
shift || true

case "$command" in
  request_json) request_json "$@" ;;
  wait_local_api) wait_local_api "$@" ;;
  password_login) password_login "$@" ;;
  wait_signed_in_or_login) wait_signed_in_or_login "$@" ;;
  wait_control_ready) wait_control_ready "$@" ;;
  ensure_network_ready) ensure_network_ready "$@" ;;
  wait_network_module) wait_network_module "$@" ;;
  send_client_message) send_client_message "$@" ;;
  wait_client_message) wait_client_message "$@" ;;
  start_echo_server) start_echo_server "$@" ;;
  wait_echo_ready) wait_echo_ready "$@" ;;
  send_udp_echo) send_udp_echo "$@" ;;
  send_tcp_echo) send_tcp_echo "$@" ;;
  *)
    echo "usage: $0 <request_json|wait_local_api|password_login|wait_signed_in_or_login|wait_control_ready|ensure_network_ready|wait_network_module|send_client_message|wait_client_message|start_echo_server|wait_echo_ready|send_udp_echo|send_tcp_echo> ..." >&2
    exit 2
    ;;
esac
