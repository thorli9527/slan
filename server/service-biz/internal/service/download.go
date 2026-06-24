package service

import (
	"context"
)

type DownloadClientFileView struct {
	FileName string `json:"fileName"`
	Script   string `json:"script,omitempty"`
	Path     string `json:"path,omitempty"`
	URL      string `json:"url,omitempty"`
	Found    bool   `json:"found"`
}

type DownloadUseCase interface {
	GetClientDownload(ctx context.Context, fileName string, requestPath string) (DownloadClientFileView, error)
	ListClientDownloads(ctx context.Context) ([]ClientDownloadView, error)
}
