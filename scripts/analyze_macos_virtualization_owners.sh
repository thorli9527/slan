#!/bin/sh
set -eu

echo "== Installed Apps =="
system_profiler SPApplicationsDataType 2>/dev/null | grep -iE 'Docker|OrbStack|Tailscale|VMware|Parallels|UTM|VirtualBox' || true

echo
echo "== Launchd Services =="
launchctl list 2>/dev/null | grep -iE 'docker|orbstack|tailscale|vmnet|vpn|wireguard' || true

echo
echo "== vmnet / virtualization logs (last 1h) =="
/usr/bin/log show --last 1h --style compact --predicate '(process == "vmnetd" OR process == "Docker" OR process == "OrbStack" OR eventMessage CONTAINS[c] "feth" OR eventMessage CONTAINS[c] "vmnet" OR eventMessage CONTAINS[c] "bridge0")' 2>/dev/null || true

echo
echo "== Tailscale status =="
if command -v tailscale >/dev/null 2>&1; then
  tailscale status 2>/dev/null || true
else
  echo "tailscale CLI not found"
fi

echo
echo "== Docker contexts =="
if command -v docker >/dev/null 2>&1; then
  docker context ls 2>/dev/null || true
  echo
  docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Networks}}' 2>/dev/null || true
else
  echo "docker CLI not found"
fi

echo
echo "== OrbStack CLI =="
if command -v orb >/dev/null 2>&1; then
  orb list 2>/dev/null || true
else
  echo "orb CLI not found"
fi

echo
echo "== Interface owners via lsof (may require Full Disk Access / sudo) =="
lsof -nP 2>/dev/null | grep -E 'feth|utun|gif0|bridge0' || true
