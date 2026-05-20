package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func clientDownloadDir() string {
	if value := strings.TrimSpace(os.Getenv("SLAN_CLIENT_DOWNLOAD_DIR")); value != "" {
		return value
	}
	return filepath.Join(os.TempDir(), "slan-client-downloads")
}

func (s *Server) listClientDownloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListClientDownloads(false)})
}

func (s *Server) downloadClientFile(w http.ResponseWriter, r *http.Request) {
	fileName := filepath.Base(r.PathValue("fileName"))
	if fileName == "." || fileName == "/" || fileName == "" {
		writeError(w, errBadRequest)
		return
	}
	if fileName == "install.sh" {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		_, _ = w.Write([]byte(clientBootstrapInstallScript()))
		return
	}
	http.ServeFile(w, r, filepath.Join(clientDownloadDir(), fileName))
}

func clientBootstrapInstallScript() string {
	return `#!/usr/bin/env sh
set -eu

server="http://api.dev.staticlss.com"
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

func (s *Server) opsListClientDownloads(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListClientDownloads(true)})
}

func (s *Server) opsUploadClientDownload(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeError(w, errBadRequest)
		return
	}
	platform := normalizeClientPlatform(r.FormValue("platform"))
	version := strings.TrimSpace(r.FormValue("version"))
	channel := strings.TrimSpace(r.FormValue("channel"))
	arch := strings.TrimSpace(r.FormValue("arch"))
	releaseNotes := strings.TrimSpace(r.FormValue("releaseNotes"))
	status := strings.TrimSpace(r.FormValue("status"))
	if platform == "" || version == "" {
		writeError(w, errBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, errBadRequest)
		return
	}
	defer file.Close()
	if header.Size <= 0 {
		writeError(w, errBadRequest)
		return
	}
	if err := os.MkdirAll(clientDownloadDir(), 0o755); err != nil {
		writeError(w, err)
		return
	}
	ext := filepath.Ext(header.Filename)
	storedName := fmt.Sprintf("%s-%s-%d%s", platform, sanitizeDNSLabel(version), time.Now().UnixNano(), ext)
	targetPath := filepath.Join(clientDownloadDir(), storedName)
	target, err := os.Create(targetPath)
	if err != nil {
		writeError(w, err)
		return
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(target, hash), file)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		writeError(w, copyErr)
		return
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		writeError(w, closeErr)
		return
	}
	download, err := s.store.UpsertClientDownload(ClientDownload{
		Platform:     platform,
		Version:      version,
		Arch:         arch,
		Channel:      channel,
		FileName:     header.Filename,
		FileSize:     size,
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
		DownloadURL:  "/downloads/clients/" + storedName,
		ReleaseNotes: releaseNotes,
		Status:       status,
	})
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, download)
}

func (s *Server) opsDeleteClientDownload(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	item, err := s.store.GetClientDownload(r.PathValue("downloadId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.DeleteClientDownload(item.DownloadID); err != nil {
		writeError(w, err)
		return
	}
	if fileName := strings.TrimPrefix(item.DownloadURL, "/downloads/clients/"); fileName != item.DownloadURL && fileName != "" {
		_ = os.Remove(filepath.Join(clientDownloadDir(), filepath.Base(fileName)))
	}
	w.WriteHeader(http.StatusNoContent)
}
