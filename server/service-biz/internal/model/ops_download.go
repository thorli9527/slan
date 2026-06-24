package model

type ClientDownload struct {
	DownloadID   string `json:"downloadId"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	Channel      string `json:"channel"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	ReleaseNotes string `json:"releaseNotes"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}
