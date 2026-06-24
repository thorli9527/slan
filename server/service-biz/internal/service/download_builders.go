package service

import "github.com/slan/service-biz/internal/model"

func clientDownloadViews(items []model.ClientDownload) []ClientDownloadView {
	views := make([]ClientDownloadView, 0, len(items))
	for _, item := range items {
		views = append(views, clientDownloadView(item))
	}
	return views
}
