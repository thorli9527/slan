package service

import (
	"context"
)

func (s OpsCatalogDownloadService) ListClientDownloads(ctx context.Context) ([]ClientDownloadView, error) {
	items, err := s.Catalog.ListClientDownloads(ctx)
	if err != nil {
		return nil, err
	}
	return clientDownloadViews(items), nil
}

func (s OpsCatalogDownloadService) UpsertClientDownload(ctx context.Context, input UpsertClientDownloadInput) (ClientDownloadView, error) {
	input = normalizeUpsertClientDownloadInput(input)
	if input.Name == "" && input.DownloadID == "" {
		return ClientDownloadView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := newClientDownload(s.Catalog, input, now)
	if current, ok, err := s.Catalog.GetClientDownload(ctx, item.DownloadID); err != nil {
		return ClientDownloadView{}, err
	} else if ok {
		item = mergeClientDownloadInput(current, item, input)
	}
	if err := s.Catalog.SaveClientDownload(ctx, item); err != nil {
		return ClientDownloadView{}, err
	}
	return clientDownloadView(item), nil
}

func (s OpsCatalogDownloadService) DeleteClientDownload(ctx context.Context, downloadID string) error {
	return s.Catalog.DeleteClientDownload(ctx, normalizeDownloadID(downloadID))
}
