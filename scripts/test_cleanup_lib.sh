#!/usr/bin/env bash

slan_cleanup_remote_test_devices() {
  local biz_url="$1"
  local email="$2"
  local password="$3"
  local enabled="$4"
  [[ "$enabled" == "1" || "$enabled" == "true" || "$enabled" == "yes" ]] || return 0

  local auth user_id devices device_id
  auth="$(curl --silent --show-error --connect-timeout 5 --max-time 15 -X POST "${biz_url}/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${password}\"}" 2>/dev/null || true)"
  user_id="$(printf '%s' "$auth" | sed -n 's/.*"userId":"\([^"]*\)".*/\1/p')"
  [[ -n "$user_id" ]] || return 0

  devices="$(curl --silent --show-error --connect-timeout 5 --max-time 15 "${biz_url}/api/devices?userId=${user_id}" 2>/dev/null || true)"
  while IFS= read -r device_id; do
    [[ -n "$device_id" ]] || continue
    curl --silent --show-error --connect-timeout 5 --max-time 30 -X DELETE "${biz_url}/api/devices/${device_id}?actorUserId=${user_id}" >/dev/null 2>&1 || true
  done < <(printf '%s' "$devices" | grep -o '"deviceId":"[^"]*"' | sed 's/"deviceId":"//;s/"$//')
}
