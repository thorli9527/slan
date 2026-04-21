#!/bin/sh
set -eu

echo "== Basic Interfaces =="
ifconfig -a | awk '
/^[a-z0-9]+:/ { iface=$1; sub(":", "", iface) }
/^[[:space:]]+inet / || /^[[:space:]]+inet6 / || /^[[:space:]]+status: / || /^[[:space:]]+tunnel / {
  print iface " " $0
}
' | sed 's/^[[:space:]]*//'

echo
echo "== Tunnel-like Interfaces =="
for iface in $(ifconfig -l); do
  case "$iface" in
    gif*|stf*|utun*|bridge*|tap*|tun*)
      echo "-- $iface --"
      ifconfig "$iface" || true
      echo
      ;;
  esac
done

echo "== Routes Using Tunnel-like Interfaces =="
netstat -rn | awk '/gif|utun|bridge|tap|tun/ { print }'

echo
echo "== Network Services =="
networksetup -listallnetworkservices 2>/dev/null || true

echo
echo "== VPN Connections =="
scutil --nc list 2>/dev/null || true

echo
echo "== Network Extensions =="
systemextensionsctl list 2>/dev/null || true

echo
echo "== Suspected Processes =="
ps -ef | grep -iE 'vpn|wireguard|tailscale|zerotier|openvpn|clash|surge|v2ray|tun|tunnel|packettunnel|networkextension' | grep -v grep || true

echo
echo "== Recent NetworkExtension Logs (last 10m) =="
log show --last 10m --style compact --predicate 'subsystem == "com.apple.networkextension"' 2>/dev/null || true
