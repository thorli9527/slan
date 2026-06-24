package service

import (
	"context"
	"github.com/slan/service-biz/internal/repository"
)

type ClientDownloadService struct {
	Catalog repository.OpsRepository
}

func (s ClientDownloadService) GetClientDownload(ctx context.Context, fileName string, requestPath string) (DownloadClientFileView, error) {
	items, err := s.Catalog.ListClientDownloads(ctx)
	if err != nil {
		return DownloadClientFileView{}, err
	}
	return resolveClientDownload(fileName, requestPath, items)
}

func (s ClientDownloadService) ListClientDownloads(ctx context.Context) ([]ClientDownloadView, error) {
	items, err := s.Catalog.ListClientDownloads(ctx)
	if err != nil {
		return nil, err
	}
	return clientDownloadViews(items), nil
}
