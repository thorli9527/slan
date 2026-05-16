package biz

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var clientDownloadPlatforms = map[string]string{
	"macos":   "Mac",
	"windows": "Windows",
	"ios":     "iOS",
	"linux":   "Linux",
	"android": "Android",
}

func normalizeClientPlatform(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "mac", "darwin", "macos":
		return "macos"
	case "win", "windows":
		return "windows"
	case "iphone", "ipad", "ios":
		return "ios"
	case "linux":
		return "linux"
	case "apk", "android":
		return "android"
	default:
		return value
	}
}

func clientPlatformName(platform string) string {
	if name := clientDownloadPlatforms[platform]; name != "" {
		return name
	}
	return platform
}

func (s *Store) ListClientDownloads(includeOffline bool) []ClientDownload {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ClientDownload, 0, len(s.clientDownloads))
	for _, item := range s.clientDownloads {
		if !includeOffline && item.Status != "active" {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Platform == out[j].Platform {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Platform < out[j].Platform
	})
	return out
}

func (s *Store) UpsertClientDownload(input ClientDownload) (ClientDownload, error) {
	platform := normalizeClientPlatform(input.Platform)
	if platform == "" || input.DownloadURL == "" || input.FileName == "" {
		return ClientDownload{}, errBadRequest
	}
	if _, ok := clientDownloadPlatforms[platform]; !ok {
		return ClientDownload{}, errBadRequest
	}
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel == "" {
		channel = "stable"
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "" {
		status = "active"
	}
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	id := strings.TrimSpace(input.DownloadID)
	if id == "" {
		id = fmt.Sprintf("download-%06d", s.nextDownloadSeq)
		s.nextDownloadSeq++
	}
	item := input
	item.DownloadID = id
	item.Platform = platform
	item.PlatformName = clientPlatformName(platform)
	item.Channel = channel
	item.Status = status
	item.Version = strings.TrimSpace(item.Version)
	item.Arch = strings.TrimSpace(item.Arch)
	item.ReleaseNotes = strings.TrimSpace(item.ReleaseNotes)
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	s.clientDownloads[id] = item
	return item, nil
}

func (s *Store) GetClientDownload(downloadID string) (ClientDownload, error) {
	downloadID = strings.TrimSpace(downloadID)
	if downloadID == "" {
		return ClientDownload{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.clientDownloads[downloadID]
	if !ok {
		return ClientDownload{}, errNotFound
	}
	return item, nil
}

func (s *Store) DeleteClientDownload(downloadID string) error {
	downloadID = strings.TrimSpace(downloadID)
	if downloadID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clientDownloads[downloadID]; !ok {
		return errNotFound
	}
	delete(s.clientDownloads, downloadID)
	return nil
}
