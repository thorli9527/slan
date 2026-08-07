#!/usr/bin/env sh
set -eu

payload_marker="__SLAN_CLIENT_V2_PAYLOAD_BELOW__"
payload_line="$(awk -v marker="$payload_marker" '$0 == marker { print NR + 1; exit }' "$0")"
if [ -z "$payload_line" ]; then
  echo "Invalid SLAN Linux installer: embedded payload is missing" >&2
  exit 1
fi

tmp_dir="$(mktemp -d /tmp/slan-client-v2-installer.XXXXXX)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

payload="$tmp_dir/client.tar.gz"
payload_root="$tmp_dir/root"
mkdir -p "$payload_root"
tail -n "+$payload_line" "$0" > "$payload"

if ! tar -tzf "$payload" >/dev/null 2>&1; then
  echo "Invalid SLAN Linux installer: embedded payload is corrupt" >&2
  exit 1
fi
if tar -tzf "$payload" | sed 's#^\./##' | awk '
  /^\// || /(^|\/)\.\.($|\/)/ { invalid = 1 }
  END { exit invalid ? 0 : 1 }
'; then
  echo "Invalid SLAN Linux installer: unsafe payload path" >&2
  exit 1
fi

tar -xzf "$payload" -C "$payload_root"
installer="$payload_root/opt/slan-client-v2/install/install.sh"
if [ ! -f "$installer" ]; then
  echo "Invalid SLAN Linux installer: install entrypoint is missing" >&2
  exit 1
fi

install_lib="$payload_root/opt/slan-client-v2/install/lib/slan-linux-install.sh"
service_bin="$payload_root/opt/slan-client-v2/bin/client-core-service"
if [ ! -f "$install_lib" ] || [ ! -x "$service_bin" ]; then
  echo "Invalid SLAN Linux installer: runtime files are missing" >&2
  exit 1
fi

. "$install_lib"
host_arch="$(slan_linux_host_arch)"
architecture_file="$payload_root/opt/slan-client-v2/install/architecture"
service_arch=""
if [ -f "$architecture_file" ]; then
  service_arch="$(sed -n '1p' "$architecture_file" | tr -d '[:space:]')"
fi
if [ -z "$service_arch" ]; then
  service_arch="$(slan_linux_service_arch "$service_bin" || true)"
fi
if [ -z "$service_arch" ] || [ "$host_arch" != "$service_arch" ]; then
  echo "SLAN Linux installer architecture mismatch: host=$host_arch package=${service_arch:-unknown}" >&2
  exit 1
fi

sh "$installer" "$@" --package="$payload" --tray=disabled

echo "SLAN Client V2 console installation completed"
echo "service=slan-client-v2.service"
echo "statusCommand=systemctl status slan-client-v2.service"
exit 0

__SLAN_CLIENT_V2_PAYLOAD_BELOW__
