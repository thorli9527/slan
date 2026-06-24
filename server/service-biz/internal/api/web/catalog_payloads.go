package web

import servicepkg "github.com/slan/service-biz/internal/service"

func clientDownloadPayload(view servicepkg.ClientDownloadView) map[string]any {
	return map[string]any{
		"downloadId":   view.DownloadID,
		"platform":     view.Platform,
		"platformName": view.PlatformName,
		"version":      view.Version,
		"channel":      view.Channel,
		"fileName":     view.Name,
		"fileSize":     view.FileSize,
		"downloadUrl":  view.URL,
		"status":       view.Status,
		"createdAt":    view.CreatedAt,
		"updatedAt":    view.UpdatedAt,
	}
}
