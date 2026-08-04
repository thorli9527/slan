#!/usr/bin/env bash

slan_macos_local_status_json() {
  local host="${1:-127.0.0.1:46392}"
  local local_host="${host%:*}"
  local local_port="${host##*:}"
  python3 - "$local_host" "$local_port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
payload = b'{"method":"localStatus","args":{}}\n'
sock = socket.create_connection((host, port), timeout=5)
try:
    sock.sendall(payload)
    sock.shutdown(socket.SHUT_WR)
    print(sock.recv(65535).decode())
finally:
    sock.close()
PY
}

slan_wait_macos_peer_route_ready() {
  local target_ip="$1"
  local expected_source_ip="$2"
  local service_host="${3:-127.0.0.1:46392}"
  local timeout_seconds="${4:-120}"
  local deadline=$(( $(date +%s) + timeout_seconds ))
  local stable_hits=0
  local route_output=''
  local status_output=''
  while (( $(date +%s) < deadline )); do
    route_output="$(route -n get "$target_ip" 2>/dev/null || true)"
    status_output="$(slan_macos_local_status_json "$service_host" 2>/dev/null || true)"
    if [[ "$route_output" == *"interface: utun"* ]] &&
       [[ "$status_output" == *"\"virtualIp\":\"${expected_source_ip}\""* ]] &&
       [[ "$status_output" == *"\"networkEnabled\":true"* ]]; then
      stable_hits=$((stable_hits + 1))
      if (( stable_hits >= 3 )); then
        return 0
      fi
    else
      stable_hits=0
    fi
    sleep 2
  done
  echo "route_output=${route_output:-<empty>}" >&2
  echo "status_output=${status_output:-<empty>}" >&2
  return 1
}
