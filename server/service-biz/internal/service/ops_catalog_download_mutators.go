package service

import (
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func newClientDownload(catalog repository.OpsRepository, input UpsertClientDownloadInput, now int64) model.ClientDownload {
	return model.ClientDownload{
		DownloadID:   opsDownloadID(catalog, input.DownloadID),
		Name:         input.Name,
		Platform:     input.Platform,
		Version:      input.Version,
		Arch:         input.Arch,
		Channel:      firstNonEmpty(input.Channel, "stable"),
		URL:          input.URL,
		SHA256:       input.SHA256,
		ReleaseNotes: input.ReleaseNotes,
		Status:       firstNonEmpty(input.Status, "active"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func mergeClientDownloadInput(current model.ClientDownload, next model.ClientDownload, input UpsertClientDownloadInput) model.ClientDownload {
	next.CreatedAt = current.CreatedAt
	if next.Name == "" {
		next.Name = current.Name
	}
	if next.Platform == "" {
		next.Platform = current.Platform
	}
	if next.Version == "" {
		next.Version = current.Version
	}
	if next.Arch == "" {
		next.Arch = current.Arch
	}
	if next.Channel == "" {
		next.Channel = current.Channel
	}
	if next.URL == "" {
		next.URL = current.URL
	}
	if next.SHA256 == "" {
		next.SHA256 = current.SHA256
	}
	if next.ReleaseNotes == "" {
		next.ReleaseNotes = current.ReleaseNotes
	}
	if input.Status == "" {
		next.Status = current.Status
	}
	return next
}
