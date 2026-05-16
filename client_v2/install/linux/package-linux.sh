#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)"
flutter_build_dir=""
service_bin="$repo_root/client_v2/rust/target/release/client-core-service"
output_dir="$repo_root/client_v2/.tmp/installer/linux"
variant="all"
version="${SLAN_CLIENT_V2_VERSION:-0.1.0}"
arch="${SLAN_CLIENT_V2_ARCH:-}"

host_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    armv7l|armhf)
      echo "armhf"
      ;;
    *)
      uname -m
      ;;
  esac
}

flutter_arch_dir() {
  case "$1" in
    amd64)
      echo "x64"
      ;;
    arm64)
      echo "arm64"
      ;;
    armhf)
      echo "arm"
      ;;
    *)
      echo "$1"
      ;;
  esac
}

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

if [ -z "$arch" ]; then
  arch="$(host_arch)"
fi

if [ -z "$flutter_build_dir" ]; then
  flutter_build_dir="$repo_root/client_v2/app_flutter/build/linux/$(flutter_arch_dir "$arch")/release/bundle"
fi

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

if [ "$variant" != "console" ] && [ ! -d "$flutter_build_dir" ]; then
  echo "Missing Flutter Linux release bundle: $flutter_build_dir" >&2
  exit 1
fi

stage_dir="$output_dir/stage"
root_dir="$stage_dir/root"
package_name="slan-client-v2"
tar_path="$output_dir/SLAN-Client-V2-linux-$arch.tar.gz"
deb_path="$output_dir/${package_name}_${version}_${arch}.deb"

rm -rf "$stage_dir"
mkdir -p "$root_dir/opt/slan-client-v2/bin"
mkdir -p "$root_dir/etc/slan"
mkdir -p "$root_dir/lib/systemd/system"
mkdir -p "$root_dir/usr/bin"
mkdir -p "$output_dir"

cp "$service_bin" "$root_dir/opt/slan-client-v2/bin/client-core-service"
chmod 755 "$root_dir/opt/slan-client-v2/bin/client-core-service"
cp "$repo_root/client_v2/install/linux/slan-client-v2.service" "$root_dir/lib/systemd/system/slan-client-v2.service"
cp "$repo_root/client_v2/install/linux/bin/slan-client-v2-console" "$root_dir/usr/bin/slan-client-v2-console"
chmod 755 "$root_dir/usr/bin/slan-client-v2-console"

cat > "$root_dir/etc/slan/client-v2.env.example" <<EOF
SLAN_CONTROL_BASE_URL=http://api.dev.staticlss.com
SLAN_CLIENT_CORE_SERVICE_HOST=127.0.0.1:46392
SLAN_LINUX_TUN_NAME=slan0
EOF

if [ "$variant" != "console" ]; then
  mkdir -p "$root_dir/opt/slan-client-v2/gui"
  cp -R "$flutter_build_dir/." "$root_dir/opt/slan-client-v2/gui/"
  mkdir -p "$root_dir/usr/share/applications"
  cp "$repo_root/client_v2/install/linux/slan-client-v2.desktop" "$root_dir/usr/share/applications/slan-client-v2.desktop"
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
  cat > "$deb_root/DEBIAN/preinst" <<'EOF'
#!/usr/bin/env sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop slan-client-v2.service || true
  systemctl disable slan-client-v2.service || true
  systemctl reset-failed slan-client-v2.service || true
fi
pkill -x slan_client_v2 >/dev/null 2>&1 || true
pkill -x client-core-service >/dev/null 2>&1 || true
rm -f /etc/systemd/system/slan-client-v2.service /lib/systemd/system/slan-client-v2.service
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
exit 0
EOF
  cat > "$deb_root/DEBIAN/postinst" <<'EOF'
#!/usr/bin/env sh
set -e
/opt/slan-client-v2/bin/client-core-service --ensure-device-id >/dev/null
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable slan-client-v2.service || true
  systemctl restart slan-client-v2.service || true
fi
exit 0
EOF
  cat > "$deb_root/DEBIAN/prerm" <<'EOF'
#!/usr/bin/env sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop slan-client-v2.service || true
  systemctl disable slan-client-v2.service || true
fi
pkill -x slan_client_v2 >/dev/null 2>&1 || true
pkill -x client-core-service >/dev/null 2>&1 || true
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
