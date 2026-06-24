package bootstrap

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/downloadkit"
	"github.com/slan/service-biz/internal/repository"
)

func EnsureDownloadAssets(ctx context.Context, catalog repository.OpsRepository) {
	items, err := catalog.ListClientDownloads(ctx)
	if err != nil {
		return
	}
	for _, item := range items {
		ensureClientDownloadAsset(item)
	}
}

func ensureClientDownloadAsset(item model.ClientDownload) {
	fileName := filepath.Base(strings.TrimSpace(item.URL))
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(item.Name))
	}
	if fileName == "." || fileName == "/" || fileName == "" || fileName == "install.sh" {
		return
	}
	_ = downloadkit.EnsurePlaceholderAsset(fileName)
}
