package service

import (
	"sort"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/downloadkit"
)

func resolveClientDownload(fileName string, requestPath string, items []model.ClientDownload) (DownloadClientFileView, error) {
	fileName = normalizeDownloadFileName(fileName)
	requestPath = normalizeDownloadRequestPath(requestPath)
	if !isValidDownloadFileName(fileName) {
		return DownloadClientFileView{}, ErrInvalidArgument
	}
	if fileName == "install.sh" {
		return DownloadClientFileView{
			FileName: fileName,
			Script:   clientBootstrapInstallScript(items),
			Found:    true,
		}, nil
	}
	if targetPath := downloadkit.ClientDownloadPath(fileName); targetPath != "" {
		return DownloadClientFileView{
			FileName: fileName,
			Path:     targetPath,
			Found:    true,
		}, nil
	}
	for _, item := range items {
		targetName := downloadTargetName(item)
		if targetName != fileName {
			continue
		}
		targetURL := downloadTargetURL(item)
		if targetURL == "" || targetURL == requestPath {
			return DownloadClientFileView{FileName: fileName, Found: false}, nil
		}
		return DownloadClientFileView{
			FileName: fileName,
			URL:      targetURL,
			Found:    true,
		}, nil
	}
	return DownloadClientFileView{FileName: fileName, Found: false}, nil
}

func clientBootstrapInstallScript(items []model.ClientDownload) string {
	packageURL := "/downloads/clients/slan-client-linux.tar.gz"
	amd64PackageURL := packageURL
	arm64PackageURL := packageURL
	if latest := latestActiveClientDownload(items, "linux"); latest != nil {
		if resolved := strings.TrimSpace(downloadTargetURL(*latest)); resolved != "" {
			packageURL = resolved
			amd64PackageURL = resolved
			arm64PackageURL = resolved
		}
	}
	if latest := latestActiveClientDownloadByArch(items, "linux", "amd64"); latest != nil {
		if resolved := strings.TrimSpace(downloadTargetURL(*latest)); resolved != "" {
			amd64PackageURL = resolved
		}
	}
	if latest := latestActiveClientDownloadByArch(items, "linux", "arm64"); latest != nil {
		if resolved := strings.TrimSpace(downloadTargetURL(*latest)); resolved != "" {
			arm64PackageURL = resolved
		}
	}
	return `#!/usr/bin/env sh
set -eu

server="http://127.0.0.1:28080"
installation_key=""
package_url="` + packageURL + `"
package_url_amd64="` + amd64PackageURL + `"
package_url_arm64="` + arm64PackageURL + `"
install_root="${SLAN_INSTALL_ROOT:-/opt/slan-client-v2}"
config_dir="${SLAN_CONFIG_DIR:-/etc/slan}"
tray_mode="disabled"

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
  install_root_prefix="${install_root#/}/"
  if tar -tzf "$package_path" | sed 's#^\./##' | awk -v prefix="$install_root_prefix" 'index($0, prefix) == 1 { found = 1; exit } END { exit found ? 0 : 1 }'; then
    tar -xzf "$package_path" -C /
  else
    mkdir -p "$install_root"
    tar -xzf "$package_path" -C "$install_root"
  fi
}

for arg in "$@"; do
  case "$arg" in
    --server=*) server="${arg#--server=}" ;;
    --installation-key=*) installation_key="${arg#--installation-key=}" ;;
    --session-key=*) installation_key="${arg#--session-key=}" ;;
    --package-url=*) package_url="${arg#--package-url=}" ;;
    --root=*) install_root="${arg#--root=}" ;;
    --config-dir=*) config_dir="${arg#--config-dir=}" ;;
    --tray=*) tray_mode="${arg#--tray=}" ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

mkdir -p "$config_dir"
if [ -z "$package_url" ]; then
  arch="$(uname -m 2>/dev/null || true)"
  case "$arch" in
    x86_64|amd64)
      package_url="$package_url_amd64"
      ;;
    aarch64|arm64)
      package_url="$package_url_arm64"
      ;;
    *)
      package_url="$package_url_amd64"
      ;;
  esac
fi
if [ "${package_url#/}" != "$package_url" ]; then
  package_url="${server%/}${package_url}"
fi
if command -v curl >/dev/null 2>&1; then
  tmp_pkg="$(mktemp /tmp/slan-client-linux.XXXXXX.tar.gz)"
  if curl -fsSL "$package_url" -o "$tmp_pkg"; then
    assert_safe_install_root
    rm -rf "$install_root"
    extract_package "$tmp_pkg"
  else
    echo "WARN: unable to download $package_url; only writing bootstrap config" >&2
  fi
  rm -f "$tmp_pkg"
fi
cat > "$config_dir/bootstrap.env" <<EOF
SLAN_CONTROL_BASE_URL=$server
SLAN_INSTALLATION_KEY=$installation_key
SLAN_SESSION_KEY=$installation_key
EOF
cat > "$config_dir/client-v2-install.env" <<EOF
SLAN_CLIENT_V2_INSTALL_ROOT=$install_root
SLAN_LINUX_TRAY_MODE=$tray_mode
EOF
if command -v systemctl >/dev/null 2>&1; then
  systemctl enable --now slan-client-v2 2>/dev/null || true
fi
echo "SLAN Client V2 bootstrap config written to $config_dir"
`
}

func latestActiveClientDownload(items []model.ClientDownload, platform string) *model.ClientDownload {
	filtered := make([]model.ClientDownload, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Platform), platform) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			continue
		}
		targetURL := strings.TrimSpace(downloadTargetURL(item))
		if targetURL == "" || strings.EqualFold(targetURL, "/downloads/clients/install.sh") {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt != filtered[j].UpdatedAt {
			return filtered[i].UpdatedAt > filtered[j].UpdatedAt
		}
		if filtered[i].CreatedAt != filtered[j].CreatedAt {
			return filtered[i].CreatedAt > filtered[j].CreatedAt
		}
		return filtered[i].DownloadID > filtered[j].DownloadID
	})
	return &filtered[0]
}

func latestActiveClientDownloadByArch(items []model.ClientDownload, platform string, arch string) *model.ClientDownload {
	filtered := make([]model.ClientDownload, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Platform), platform) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Arch), arch) {
			continue
		}
		targetURL := strings.TrimSpace(downloadTargetURL(item))
		if targetURL == "" || strings.EqualFold(targetURL, "/downloads/clients/install.sh") {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt != filtered[j].UpdatedAt {
			return filtered[i].UpdatedAt > filtered[j].UpdatedAt
		}
		if filtered[i].CreatedAt != filtered[j].CreatedAt {
			return filtered[i].CreatedAt > filtered[j].CreatedAt
		}
		return filtered[i].DownloadID > filtered[j].DownloadID
	})
	return &filtered[0]
}
