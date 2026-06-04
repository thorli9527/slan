package biz

// DownloadService 承载客户端安装包元数据业务实现。
type DownloadService struct {
	store BusinessStore
}

func (s DownloadService) ListClientDownloads(includeOffline bool) []ClientDownload {
	return s.store.ListClientDownloads(includeOffline)
}

func (s DownloadService) UpsertClientDownload(input ClientDownload) (ClientDownload, error) {
	return s.store.UpsertClientDownload(input)
}

func (s DownloadService) GetClientDownload(downloadID string) (ClientDownload, error) {
	return s.store.GetClientDownload(downloadID)
}

func (s DownloadService) DeleteClientDownload(downloadID string) error {
	return s.store.DeleteClientDownload(downloadID)
}
