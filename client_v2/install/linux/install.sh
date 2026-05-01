#!/usr/bin/env sh
set -eu

tray_mode="disabled"
install_root="${SLAN_INSTALL_ROOT:-/opt/slan-client-v2}"
config_dir="${SLAN_CONFIG_DIR:-/etc/slan}"

usage() {
  cat <<'EOF'
Usage: install.sh [--tray=enabled|disabled] [--root=/opt/slan-client-v2] [--config-dir=/etc/slan]

Options:
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

cat > "$config_dir/client-v2-install.env" <<EOF
SLAN_CLIENT_V2_INSTALL_ROOT=$install_root
SLAN_LINUX_TRAY_MODE=$tray_mode
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
