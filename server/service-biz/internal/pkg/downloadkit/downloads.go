package downloadkit

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ClientDownloadDir() string {
	if value := strings.TrimSpace(os.Getenv("SLAN_CLIENT_DOWNLOAD_DIR")); value != "" {
		return value
	}
	return filepath.Join(os.TempDir(), "slan-client-downloads")
}

func SaveClientDownload(file multipart.File, originalName, platform, version string) (string, int64, error) {
	if err := os.MkdirAll(ClientDownloadDir(), 0o755); err != nil {
		return "", 0, err
	}
	ext := filepath.Ext(strings.TrimSpace(originalName))
	storedName := fmt.Sprintf("%s-%s-%d%s", sanitizeToken(platform), sanitizeToken(version), time.Now().UnixNano(), ext)
	targetPath := filepath.Join(ClientDownloadDir(), storedName)
	target, err := os.Create(targetPath)
	if err != nil {
		return "", 0, err
	}
	size, copyErr := io.Copy(target, file)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return "", 0, closeErr
	}
	return storedName, size, nil
}

func ClientDownloadPath(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	return filepath.Join(ClientDownloadDir(), base)
}

func RemoveClientDownload(fileName string) error {
	targetPath := ClientDownloadPath(fileName)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func EnsurePlaceholderAsset(fileName string) error {
	targetPath := ClientDownloadPath(fileName)
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(ClientDownloadDir(), 0o755); err != nil {
		return err
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".zip":
		return writePlaceholderZip(targetPath, fileName)
	case ".gz":
		if strings.HasSuffix(strings.ToLower(fileName), ".tar.gz") {
			return writePlaceholderTarGz(targetPath, fileName)
		}
	}
	return os.WriteFile(targetPath, []byte("placeholder download asset\n"), 0o644)
}

func sanitizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "download"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "download"
	}
	return result
}

func writePlaceholderTarGz(targetPath, fileName string) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	content := []byte("placeholder archive for " + fileName + "\n")
	header := &tar.Header{
		Name: "README.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	return nil
}

func writePlaceholderZip(targetPath, fileName string) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	writer, err := zw.Create("README.txt")
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte("placeholder archive for " + fileName + "\n")); err != nil {
		return err
	}
	return nil
}
