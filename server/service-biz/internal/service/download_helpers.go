package service

import (
	"path/filepath"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func normalizeDownloadFileName(fileName string) string {
	return filepath.Base(strings.TrimSpace(fileName))
}

func isValidDownloadFileName(fileName string) bool {
	return fileName != "." && fileName != "/" && fileName != ""
}

func normalizeDownloadRequestPath(requestPath string) string {
	return strings.TrimSpace(requestPath)
}

func downloadTargetName(item model.ClientDownload) string {
	targetName := filepath.Base(strings.TrimSpace(item.Name))
	if targetName != "" {
		return targetName
	}
	return filepath.Base(strings.TrimSpace(item.URL))
}

func downloadTargetURL(item model.ClientDownload) string {
	return strings.TrimSpace(item.URL)
}
