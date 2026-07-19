#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)"
install_dir="$repo_root/client_v2/install/linux"
. "$install_dir/lib/slan-linux-install.sh"

flutter_build_dir=""
service_bin="$repo_root/client_v2/rust/target/release/client-core-service"
output_dir="$repo_root/client_v2/.tmp/installer/linux"
variant="all"
version="$SLAN_LINUX_PACKAGE_VERSION"
arch="${SLAN_CLIENT_V2_ARCH:-}"

usage() {
  cat <<'EOF'
Usage: package-linux.sh [options]

Options:
  --flutter-build-dir=PATH  Flutter Linux release bundle.
  --service-bin=PATH        client-core-service binary.
  --output-dir=PATH         Output directory.
  --variant=gui|console|all Package variant.
  --version=VERSION         Package version.
  -h, --help                Show this help.
EOF
}

for arg in "$@"; do
  case "$arg" in
    --flutter-build-dir=*)
      flutter_build_dir="${arg#--flutter-build-dir=}"
      ;;
    --service-bin=*)
      service_bin="${arg#--service-bin=}"
      ;;
    --output-dir=*)
      output_dir="${arg#--output-dir=}"
      ;;
    --variant=*)
      variant="${arg#--variant=}"
      ;;
    --version=*)
      version="${arg#--version=}"
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

case "$variant" in
  gui|console|all)
    ;;
  *)
    echo "Invalid variant: $variant" >&2
    exit 2
    ;;
esac

if [ ! -f "$service_bin" ]; then
  echo "Missing client-core-service binary: $service_bin" >&2
  exit 1
fi

detected_arch="$(slan_linux_service_arch "$service_bin" || true)"
if [ -z "$detected_arch" ]; then
  echo "Refusing to package non-Linux or unsupported client-core-service binary: $service_bin" >&2
  if command -v file >/dev/null 2>&1; then
    file "$service_bin" >&2 || true
  fi
  exit 1
fi

if [ -z "$arch" ]; then
  arch="$detected_arch"
else
  arch="$(slan_linux_normalize_arch "$arch")"
  if [ "$arch" != "$detected_arch" ]; then
    echo "Linux package arch mismatch: requested $arch, service binary is $detected_arch ($service_bin)" >&2
    exit 1
  fi
fi

if [ -z "$flutter_build_dir" ]; then
  flutter_build_dir="$repo_root/client_v2/app_flutter/build/linux/$(slan_linux_flutter_arch_dir "$arch")/release/bundle"
fi

if [ "$variant" != "console" ] && [ ! -d "$flutter_build_dir" ]; then
  echo "Missing Flutter Linux release bundle: $flutter_build_dir" >&2
  exit 1
fi

stage_dir="$output_dir/stage"
root_dir="$stage_dir/root"
package_name="$SLAN_LINUX_PACKAGE_NAME"
tar_path="$output_dir/SLAN-Client-V2-linux-$arch.tar.gz"
deb_path="$output_dir/${package_name}_${version}_${arch}.deb"

rm -rf "$stage_dir"
rm -f "$tar_path" "$deb_path"
mkdir -p "$root_dir$SLAN_LINUX_INSTALL_ROOT/bin"
mkdir -p "$root_dir$SLAN_LINUX_LIB_DIR"
mkdir -p "$root_dir$SLAN_LINUX_CONFIG_DIR"
mkdir -p "$root_dir/usr/lib/systemd/system"
mkdir -p "$root_dir/usr/bin"
mkdir -p "$output_dir"

cp "$service_bin" "$root_dir$SLAN_LINUX_SERVICE_BIN"
chmod 755 "$root_dir$SLAN_LINUX_SERVICE_BIN"
cp "$install_dir/lib/slan-linux-install.sh" "$root_dir$SLAN_LINUX_INSTALL_LIB"
chmod 644 "$root_dir$SLAN_LINUX_INSTALL_LIB"
cat > "$root_dir$SLAN_LINUX_SERVICE_UNIT_PATH" <<EOF
[Unit]
Description=$SLAN_LINUX_APP_NAME Core Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-$SLAN_LINUX_CONFIG_DIR/$SLAN_LINUX_RUNTIME_ENV_NAME
EnvironmentFile=-$SLAN_LINUX_CONFIG_DIR/$SLAN_LINUX_CONSOLE_ENV_NAME
ExecStart=$SLAN_LINUX_SERVICE_BIN
Restart=on-failure
RestartSec=3
RuntimeDirectory=slan-client-v2
StateDirectory=slan
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF
cp "$install_dir/bin/slan-client-v2-console" "$root_dir$SLAN_LINUX_CONSOLE_BIN"
chmod 755 "$root_dir$SLAN_LINUX_CONSOLE_BIN"

slan_linux_write_runtime_env_example "$root_dir$SLAN_LINUX_CONFIG_DIR/$SLAN_LINUX_RUNTIME_ENV_NAME.example"

if [ "$variant" != "console" ]; then
  mkdir -p "$root_dir$SLAN_LINUX_INSTALL_ROOT/gui"
  cp -R "$flutter_build_dir/." "$root_dir$SLAN_LINUX_INSTALL_ROOT/gui/"
  mkdir -p "$root_dir/usr/share/applications"
  cat > "$root_dir$SLAN_LINUX_DESKTOP_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=$SLAN_LINUX_APP_NAME
Comment=SLAN desktop client
Exec=$SLAN_LINUX_GUI_BIN
Icon=slan-client-v2
Terminal=false
Categories=Network;
EOF
fi

tar -C "$root_dir" -czf "$tar_path" .

if command -v dpkg-deb >/dev/null 2>&1; then
  deb_root="$stage_dir/deb"
  mkdir -p "$deb_root/DEBIAN"
  cp -R "$root_dir/." "$deb_root/"
  installed_size="$(du -sk "$deb_root" | awk '{print $1}')"
  cat > "$deb_root/DEBIAN/control" <<EOF
Package: $package_name
Version: $version
Section: net
Priority: optional
Architecture: $arch
Maintainer: SLAN
Installed-Size: $installed_size
Description: SLAN Client V2 desktop and console runtime
EOF
  cat > "$deb_root/DEBIAN/preinst" <<EOF
#!/usr/bin/env sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop $SLAN_LINUX_SERVICE_NAME || true
  systemctl disable $SLAN_LINUX_SERVICE_NAME || true
  systemctl reset-failed $SLAN_LINUX_SERVICE_NAME || true
fi
pkill -x slan_client_v2 >/dev/null 2>&1 || true
pkill -x $SLAN_LINUX_SERVICE_BIN_NAME >/dev/null 2>&1 || true
pkill -f '^$SLAN_LINUX_SERVICE_BIN([[:space:]]|$)' >/dev/null 2>&1 || true
pkill -f '^$SLAN_LINUX_GUI_BIN([[:space:]]|$)' >/dev/null 2>&1 || true
rm -f $SLAN_LINUX_ETC_SERVICE_UNIT_PATH $SLAN_LINUX_SERVICE_UNIT_PATH
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
exit 0
EOF
  cat > "$deb_root/DEBIAN/postinst" <<EOF
#!/usr/bin/env sh
set -e
$SLAN_LINUX_SERVICE_BIN --ensure-device-id >/dev/null
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable $SLAN_LINUX_SERVICE_NAME || true
  systemctl restart $SLAN_LINUX_SERVICE_NAME || true
fi
exit 0
EOF
  cat > "$deb_root/DEBIAN/prerm" <<EOF
#!/usr/bin/env sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop $SLAN_LINUX_SERVICE_NAME || true
  systemctl disable $SLAN_LINUX_SERVICE_NAME || true
fi
pkill -x slan_client_v2 >/dev/null 2>&1 || true
pkill -x $SLAN_LINUX_SERVICE_BIN_NAME >/dev/null 2>&1 || true
pkill -f '^$SLAN_LINUX_SERVICE_BIN([[:space:]]|$)' >/dev/null 2>&1 || true
pkill -f '^$SLAN_LINUX_GUI_BIN([[:space:]]|$)' >/dev/null 2>&1 || true
exit 0
EOF
  chmod 755 "$deb_root/DEBIAN/preinst" "$deb_root/DEBIAN/postinst" "$deb_root/DEBIAN/prerm"
  dpkg-deb --build "$deb_root" "$deb_path" >/dev/null
  echo "Linux deb: $deb_path"
else
  echo "dpkg-deb not found; skipped .deb generation"
fi

echo "Linux tarball: $tar_path"
echo "Linux stage: $root_dir"
