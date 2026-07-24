package clientlogkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const MaxStoredBundlesPerDevice = 20

func Dir() string {
	if value := strings.TrimSpace(os.Getenv("SLAN_CLIENT_LOG_DIR")); value != "" {
		return value
	}
	return filepath.Join(os.TempDir(), "slan-client-logs")
}

func Save(deviceID string, payload any) (string, int64, error) {
	deviceID = sanitize(deviceID)
	deviceDir := filepath.Join(Dir(), deviceID)
	if err := os.MkdirAll(deviceDir, 0o750); err != nil {
		return "", 0, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	name := fmt.Sprintf("logs-%d.json", time.Now().UnixNano())
	target := filepath.Join(deviceDir, name)
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return "", 0, err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return "", 0, err
	}
	prune(deviceDir)
	return name, int64(len(data)), nil
}

func sanitize(value string) string {
	var result strings.Builder
	for _, char := range strings.TrimSpace(value) {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			result.WriteRune(char)
		}
	}
	if result.Len() == 0 {
		return "unknown-device"
	}
	return result.String()
}

func prune(deviceDir string) {
	entries, err := os.ReadDir(deviceDir)
	if err != nil {
		return
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "logs-") && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for len(files) > MaxStoredBundlesPerDevice {
		_ = os.Remove(filepath.Join(deviceDir, files[0]))
		files = files[1:]
	}
}
