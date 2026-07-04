#!/usr/bin/env sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if [ -f "$script_dir/lib/slan-linux-install.sh" ]; then
  . "$script_dir/lib/slan-linux-install.sh"
else
  . "/opt/slan-client-v2/lib/slan-linux-install.sh"
fi

tray_mode="disabled"
install_root="$SLAN_LINUX_INSTALL_ROOT"
config_dir="$SLAN_LINUX_CONFIG_DIR"
server="$SLAN_LINUX_DEFAULT_CONTROL_BASE_URL"
installation_key="${SLAN_INSTALLATION_KEY:-${SLAN_SESSION_KEY:-}}"
package_url="${SLAN_CLIENT_PACKAGE_URL:-}"

assert_safe_install_root() {
  if ! slan_linux_safe_install_root "$install_root"; then
    echo "Refusing to remove unsafe install root: $install_root" >&2
    exit 2
  fi
}

extract_package() {
  package_path="$1"
  install_root_prefix="${install_root#/}/"
  if tar -tzf "$package_path" | sed 's#^\./##' | awk -v prefix="$install_root_prefix" 'index($0, prefix) == 1 { found = 1; exit } END { exit found ? 0 : 1 }'; then
    tar -xzf "$package_path" -C /
  else
    mkdir -p "$install_root"
    tar -xzf "$package_path" -C "$install_root"
  fi
}

usage() {
  cat <<'EOF'
Usage: install.sh [--server=URL] [--installation-key=ik_xxx] [--session-key=ik_xxx] [--package-url=URL] [--tray=enabled|disabled] [--root=PATH] [--config-dir=PATH]

Options:
  --server=URL      Control-plane base URL.
  --installation-key=KEY One-time device bootstrap installation key.
  --session-key=KEY Legacy alias for the installation key.
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
    --installation-key=*)
      installation_key="${arg#--installation-key=}"
      ;;
    --session-key=*)
      installation_key="${arg#--session-key=}"
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
    slan_linux_stop_runtime
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

cat > "$config_dir/$SLAN_LINUX_INSTALL_ENV_NAME" <<EOF
SLAN_CLIENT_V2_INSTALL_ROOT=$install_root
SLAN_LINUX_TRAY_MODE=$tray_mode
EOF

cat > "$config_dir/$SLAN_LINUX_BOOTSTRAP_ENV_NAME" <<EOF
SLAN_CONTROL_BASE_URL=$server
SLAN_INSTALLATION_KEY=$installation_key
SLAN_SESSION_KEY=$installation_key
EOF

if [ "$tray_mode" = "enabled" ]; then
  cat > "$config_dir/$SLAN_LINUX_DESKTOP_POLICY_NAME" <<EOF
desktopShell=enabled
closeBehavior=hide-to-tray
quitBehavior=shutdown-network-then-exit
EOF
else
  cat > "$config_dir/$SLAN_LINUX_DESKTOP_POLICY_NAME" <<EOF
desktopShell=disabled
serviceOnly=true
EOF
fi

echo "SLAN Client V2 Linux install policy written to $config_dir"
echo "trayMode=$tray_mode"
echo "controlBaseUrl=$server"
if [ -x "$install_root/bin/$SLAN_LINUX_SERVICE_BIN_NAME" ]; then
  "$install_root/bin/$SLAN_LINUX_SERVICE_BIN_NAME" --ensure-device-id >/dev/null
fi
slan_linux_restart_service
