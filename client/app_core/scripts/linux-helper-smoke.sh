#!/usr/bin/env sh
set -eu

interface_name="${SLAN_LINUX_SMOKE_INTERFACE:-slan0}"
local_ip="${SLAN_LINUX_SMOKE_LOCAL_IP:-100.64.0.10}"
peer_ip="${SLAN_LINUX_SMOKE_PEER_IP:-100.64.0.2}"
endpoint="${SLAN_LINUX_SMOKE_ENDPOINT:-127.0.0.1:51820}"

private_key="$(wg genkey)"
public_key="$(printf '%s' "$private_key" | wg pubkey)"
peer_private_key="$(wg genkey)"
peer_public_key="$(printf '%s' "$peer_private_key" | wg pubkey)"

cleanup() {
  ip link delete "$interface_name" 2>/dev/null || true
}

cleanup
trap cleanup EXIT

request_apply=$(cat <<EOF
{"method":"applyTunnelConfiguration","args":{"transport":"relay","localVirtualIp":"$local_ip","peerVirtualIp":"$peer_ip","wireguardInterface":{"interfaceName":"$interface_name","keyPair":{"publicKey":"$public_key","privateKey":"$private_key"},"listenPort":51820,"mtu":1280,"addresses":["$local_ip/32"],"dnsServers":[],"peers":[]},"wireguardPeer":{"peerNodeId":"linux-smoke-peer","publicKey":"$peer_public_key","endpoint":"$endpoint","allowedIps":[{"cidr":"$peer_ip/32"}],"persistentKeepaliveSeconds":15}}}
EOF
)

request_up='{"method":"bringTunnelUp","args":{}}'
request_runtime=$(cat <<EOF
{"method":"tunnelRuntimeView","args":{"peerVirtualIp":"$peer_ip"}}
EOF
)

responses="$(
  {
    printf '%s\n' "$request_apply"
    printf '%s\n' "$request_up"
    printf '%s\n' "$request_runtime"
  } | SLAN_CONTROL_BASE_URL="${SLAN_CONTROL_BASE_URL:-http://127.0.0.1:28080}" /app/app-core-helper
)"

printf '%s\n' "$responses"

if printf '%s\n' "$responses" | grep -q '"ok":false'; then
  echo "linux-helper-smoke failed: helper returned an error" >&2
  exit 1
fi

printf '%s\n' "$responses" | grep -q '"ok":true'
ip link show "$interface_name" >/dev/null
ip address show dev "$interface_name" | grep -q "$local_ip/32"
wg show "$interface_name" >/dev/null

echo "linux-helper-smoke ok: $interface_name has $local_ip/32"
