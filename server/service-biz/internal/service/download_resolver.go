package service

import (
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
			Script:   clientBootstrapInstallScript(),
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

func clientBootstrapInstallScript() string {
	return `#!/usr/bin/env sh
set -eu

server="http://127.0.0.1:28080"
session_key=""
package_url=""
install_root="${SLAN_INSTALL_ROOT:-/opt/slan-client-v2}"
config_dir="${SLAN_CONFIG_DIR:-/etc/slan}"
tray_mode="disabled"

for arg in "$@"; do
  case "$arg" in
    --server=*) server="${arg#--server=}" ;;
    --session-key=*) session_key="${arg#--session-key=}" ;;
    --package-url=*) package_url="${arg#--package-url=}" ;;
    --root=*) install_root="${arg#--root=}" ;;
    --config-dir=*) config_dir="${arg#--config-dir=}" ;;
    --tray=*) tray_mode="${arg#--tray=}" ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

mkdir -p "$config_dir"
if [ -z "$package_url" ]; then
  package_url="${server%/}/downloads/clients/slan-client-linux.tar.gz"
fi
if command -v curl >/dev/null 2>&1; then
  tmp_pkg="$(mktemp /tmp/slan-client-linux.XXXXXX.tar.gz)"
  if curl -fsSL "$package_url" -o "$tmp_pkg"; then
    mkdir -p "$install_root"
    tar -xzf "$tmp_pkg" -C "$install_root"
  else
    echo "WARN: unable to download $package_url; only writing bootstrap config" >&2
  fi
  rm -f "$tmp_pkg"
fi
cat > "$config_dir/bootstrap.env" <<EOF
SLAN_CONTROL_BASE_URL=$server
SLAN_SESSION_KEY=$session_key
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
