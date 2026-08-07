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
server="${SLAN_CONTROL_BASE_URL:-}"
authorization_key="${SLAN_DEVICE_AUTHORIZATION_KEY:-}"
package_url="${SLAN_CLIENT_PACKAGE_URL:-}"
package_path="${SLAN_CLIENT_PACKAGE_PATH:-}"
enable_network="false"

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

require_value() {
  option="$1"
  value="${2:-}"
  if [ -z "$value" ]; then
    echo "Missing value for $option" >&2
    exit 2
  fi
}

usage() {
  cat <<'EOF'
Usage: install.sh --server-url URL --authorization-key KEY [options]

Options:
  --server-url URL  Control-plane base URL. Required.
  --server URL      Alias for --server-url.
  --authorization-key KEY Device authorization key managed by Opt. Required.
  --package PATH    Install a local Linux client tarball.
  --package-url URL Download URL for the Linux client tarball.
  --enable-network  Enable the network after successful activation.
  --tray=enabled    Install the desktop shell with tray integration.
  --tray=disabled   Install service/helper only. This is the default.
  --root=PATH       Target application root.
  --config-dir=PATH Target configuration directory.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --tray=enabled)
      tray_mode="enabled"
      shift
      ;;
    --tray=disabled)
      tray_mode="disabled"
      shift
      ;;
    --root=*)
      install_root="${1#--root=}"
      shift
      ;;
    --config-dir=*)
      config_dir="${1#--config-dir=}"
      shift
      ;;
    --server=*|--server-url=*)
      server="${1#--server=}"
      server="${server#--server-url=}"
      shift
      ;;
    --server|--server-url)
      require_value "$1" "${2:-}"
      server="$2"
      shift 2
      ;;
    --authorization-key=*)
      authorization_key="${1#--authorization-key=}"
      shift
      ;;
    --authorization-key)
      require_value "$1" "${2:-}"
      authorization_key="$2"
      shift 2
      ;;
    --package=*)
      package_path="${1#--package=}"
      shift
      ;;
    --package)
      require_value "$1" "${2:-}"
      package_path="$2"
      shift 2
      ;;
    --package-url=*)
      package_url="${1#--package-url=}"
      shift
      ;;
    --package-url)
      require_value "$1" "${2:-}"
      package_url="$2"
      shift 2
      ;;
    --enable-network)
      enable_network="true"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
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

case "$server" in
  http://*|https://*)
    ;;
  *)
    echo "--server must be an http:// or https:// URL" >&2
    exit 2
    ;;
esac

case "$server" in
  *[[:space:]]*)
    echo "--server-url must not contain whitespace" >&2
    exit 2
    ;;
esac
server="${server%/}"

if [ -z "$authorization_key" ]; then
  echo "--authorization-key is required" >&2
  exit 2
fi
case "$authorization_key" in
  *[[:space:]]*)
    echo "--authorization-key must not contain whitespace" >&2
    exit 2
    ;;
esac

if [ -n "$package_path" ] && [ -n "$package_url" ]; then
  echo "Use only one of --package or --package-url" >&2
  exit 2
fi

if [ -z "$package_path" ] && [ -z "$package_url" ]; then
  echo "--package or --package-url is required" >&2
  exit 2
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "Run this installer as root (for example: sudo ./install.sh ...)" >&2
  exit 1
fi

mkdir -p "$config_dir"

if [ -n "$package_path" ]; then
  if [ ! -f "$package_path" ]; then
    echo "Linux client package not found: $package_path" >&2
    exit 1
  fi
  slan_linux_stop_runtime
  assert_safe_install_root
  rm -rf "$install_root"
  extract_package "$package_path"
elif command -v curl >/dev/null 2>&1; then
  tmp_pkg="$(mktemp /tmp/slan-client-linux.XXXXXX.tar.gz)"
  if curl -fsSL "$package_url" -o "$tmp_pkg"; then
    slan_linux_stop_runtime
    assert_safe_install_root
    rm -rf "$install_root"
    extract_package "$tmp_pkg"
  else
    rm -f "$tmp_pkg"
    echo "Unable to download Linux client package: $package_url" >&2
    exit 1
  fi
  rm -f "$tmp_pkg"
else
  echo "curl is required when --package-url is used" >&2
  exit 1
fi

umask 077
cat > "$config_dir/$SLAN_LINUX_INSTALL_ENV_NAME" <<EOF
SLAN_CLIENT_V2_INSTALL_ROOT=$install_root
SLAN_LINUX_TRAY_MODE=$tray_mode
EOF

cat > "$config_dir/$SLAN_LINUX_CONSOLE_ENV_NAME" <<EOF
SLAN_CONTROL_BASE_URL=$server
SLAN_DEVICE_AUTHORIZATION_KEY=$authorization_key
SLAN_PENDING_ENABLE_NETWORK=$enable_network
EOF
chmod 600 "$config_dir/$SLAN_LINUX_CONSOLE_ENV_NAME"

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
slan_linux_restart_service
