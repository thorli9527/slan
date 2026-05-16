#!/usr/bin/env sh
set -eu

tray_mode="disabled"
install_root="${SLAN_INSTALL_ROOT:-/opt/slan-client-v2}"
config_dir="${SLAN_CONFIG_DIR:-/etc/slan}"
server="${SLAN_CONTROL_BASE_URL:-http://api.dev.staticlss.com}"
session_key="${SLAN_SESSION_KEY:-}"
package_url="${SLAN_CLIENT_PACKAGE_URL:-}"
service_name="slan-client-v2.service"

stop_existing_runtime() {
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$service_name" >/dev/null 2>&1 || true
    systemctl disable "$service_name" >/dev/null 2>&1 || true
    systemctl reset-failed "$service_name" >/dev/null 2>&1 || true
  fi

  pkill -x "slan_client_v2" >/dev/null 2>&1 || true
  pkill -x "client-core-service" >/dev/null 2>&1 || true

  rm -f "/etc/systemd/system/$service_name" "/lib/systemd/system/$service_name"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi
}

restart_installed_service() {
  if command -v systemctl >/dev/null 2>&1 && \
    { [ -f "/etc/systemd/system/$service_name" ] || [ -f "/lib/systemd/system/$service_name" ]; }; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable "$service_name" >/dev/null 2>&1 || true
    systemctl restart "$service_name" >/dev/null 2>&1 || true
  fi
}

assert_safe_install_root() {
  case "$install_root" in
    ""|"/"|"/bin"|"/etc"|"/lib"|"/opt"|"/sbin"|"/usr"|"/var")
      echo "Refusing to remove unsafe install root: $install_root" >&2
      exit 2
      ;;
  esac
}

extract_package() {
  package_path="$1"
  if tar -tzf "$package_path" | grep -Eq '^(\./)?opt/slan-client-v2/'; then
    tar -xzf "$package_path" -C /
  else
    mkdir -p "$install_root"
    tar -xzf "$package_path" -C "$install_root"
  fi
}

usage() {
  cat <<'EOF'
Usage: install.sh [--server=http://api.dev.staticlss.com] [--session-key=sk_xxx] [--package-url=URL] [--tray=enabled|disabled] [--root=/opt/slan-client-v2] [--config-dir=/etc/slan]

Options:
  --server=URL      Control-plane base URL.
  --session-key=KEY One-time device bootstrap session key.
  --package-url=URL Download URL for the Linux client tarball.
  --tray=enabled    Install the desktop shell with tray integration.
  --tray=disabled   Install service/helper only. This is the default.
  --root=PATH       Target application root.
  --config-dir=PATH Target configuration directory.
EOF
}

for arg in "$@"; do
  case "$arg" in
    --tray=enabled)
      tray_mode="enabled"
      ;;
    --tray=disabled)
      tray_mode="disabled"
      ;;
    --root=*)
      install_root="${arg#--root=}"
      ;;
    --config-dir=*)
      config_dir="${arg#--config-dir=}"
      ;;
    --server=*)
      server="${arg#--server=}"
      ;;
    --session-key=*)
      session_key="${arg#--session-key=}"
      ;;
    --package-url=*)
      package_url="${arg#--package-url=}"
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$tray_mode" in
  enabled|disabled)
    ;;
  *)
    echo "Invalid tray mode: $tray_mode" >&2
    exit 2
    ;;
esac

mkdir -p "$config_dir"

if [ -z "$package_url" ]; then
  package_url="${server%/}/downloads/clients/slan-client-linux.tar.gz"
fi

if command -v curl >/dev/null 2>&1; then
  tmp_pkg="$(mktemp /tmp/slan-client-linux.XXXXXX.tar.gz)"
  if curl -fsSL "$package_url" -o "$tmp_pkg"; then
    stop_existing_runtime
    assert_safe_install_root
    rm -rf "$install_root"
    extract_package "$tmp_pkg"
  else
    echo "WARN: unable to download $package_url; only writing install policy" >&2
  fi
  rm -f "$tmp_pkg"
else
  echo "WARN: curl is not installed; only writing install policy" >&2
fi

cat > "$config_dir/client-v2-install.env" <<EOF
SLAN_CLIENT_V2_INSTALL_ROOT=$install_root
SLAN_LINUX_TRAY_MODE=$tray_mode
EOF

cat > "$config_dir/bootstrap.env" <<EOF
SLAN_CONTROL_BASE_URL=$server
SLAN_SESSION_KEY=$session_key
EOF

if [ "$tray_mode" = "enabled" ]; then
  cat > "$config_dir/client-v2-desktop.policy" <<EOF
desktopShell=enabled
closeBehavior=hide-to-tray
quitBehavior=shutdown-network-then-exit
EOF
else
  cat > "$config_dir/client-v2-desktop.policy" <<EOF
desktopShell=disabled
serviceOnly=true
EOF
fi

echo "SLAN Client V2 Linux install policy written to $config_dir"
echo "trayMode=$tray_mode"
echo "controlBaseUrl=$server"
if [ -x "$install_root/bin/client-core-service" ]; then
  "$install_root/bin/client-core-service" --ensure-device-id >/dev/null
fi
restart_installed_service
