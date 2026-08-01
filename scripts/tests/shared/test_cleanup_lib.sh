#!/usr/bin/env bash

slan_cleanup_remote_test_devices() {
  local biz_url="$1"
  local email="$2"
  local password="$3"
  local enabled="$4"
  [[ "$enabled" == "1" || "$enabled" == "true" || "$enabled" == "yes" ]] || return 0

  local web_base="${5:-${SLAN_WEB_BASE_URL:-${SLAN_DEFAULT_WEB_BASE_URL:-}}}"
  if [[ -z "$web_base" ]]; then
    web_base="$biz_url"
  fi

  local auth access_token user_id devices device_id
  auth="$(curl --silent --show-error --connect-timeout 5 --max-time 15 -X POST "${biz_url}/api/app/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\"}" 2>/dev/null || true)"
  access_token="$(printf '%s' "$auth" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
  user_id="$(printf '%s' "$auth" | sed -n 's/.*"userId":"\([^"]*\)".*/\1/p')"
  [[ -n "$access_token" && -n "$user_id" ]] || return 0

  devices="$(curl --silent --show-error --connect-timeout 5 --max-time 15 \
    -H "Authorization: Bearer ${access_token}" \
    "${web_base}/api/app/devices?userId=${user_id}" 2>/dev/null || true)"
  while IFS= read -r device_id; do
    [[ -n "$device_id" ]] || continue
    curl --silent --show-error --connect-timeout 5 --max-time 30 \
      -X DELETE "${web_base}/api/app/devices/${device_id}?actorUserId=${user_id}" \
      -H "Authorization: Bearer ${access_token}" >/dev/null 2>&1 || true
  done < <(printf '%s' "$devices" | grep -o '"deviceId":"[^"]*"' | sed 's/"deviceId":"//;s/"$//')
}

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
