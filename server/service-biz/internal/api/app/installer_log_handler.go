package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	"github.com/slan/service-biz/internal/pkg/clientlogkit"
	servicepkg "github.com/slan/service-biz/internal/service"
)

const (
	maxInstallerLogUploadBytes = 256 << 10
	installerLogIdentityLimit  = 2
	installerLogIPLimit        = 20
)

type InstallerLogHandler struct {
	Limiter *deviceRequestLimiter
}

type installerLogUpload struct {
	InstallationID string            `json:"installationId"`
	Platform       string            `json:"platform"`
	Version        string            `json:"version,omitempty"`
	CapturedAt     int64             `json:"capturedAt"`
	Stage          string            `json:"stage"`
	Error          string            `json:"error"`
	Files          map[string]string `json:"files"`
}

func (h InstallerLogHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/diagnostics/installer", h.Upload),
	}
}

func (h InstallerLogHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxInstallerLogUploadBytes)
	defer r.Body.Close()
	var input installerLogUpload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input.InstallationID = strings.TrimSpace(input.InstallationID)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.Stage = strings.TrimSpace(input.Stage)
	input.Error = strings.TrimSpace(input.Error)
	if !validInstallerLogUpload(input) {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	remoteIP := serviceapi.RemoteIP(r)
	identity := deviceRequestIdentity("installer", input.InstallationID)
	if h.Limiter != nil && !h.Limiter.Allow(remoteIP, identity, time.Now()) {
		serviceapi.WriteError(w, servicepkg.ErrRateLimited)
		return
	}
	name, size, err := clientlogkit.Save("installer-"+input.InstallationID, input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"uploadId": name,
		"size":     size,
	})
}

func validInstallerLogUpload(input installerLogUpload) bool {
	if len(input.InstallationID) < 16 || len(input.InstallationID) > 64 ||
		input.Platform != "windows" || input.CapturedAt <= 0 ||
		input.Stage == "" || len(input.Stage) > 64 ||
		input.Error == "" || len(input.Error) > 4096 ||
		len(input.Version) > 64 || len(input.Files) == 0 || len(input.Files) > 4 {
		return false
	}
	for _, char := range input.InstallationID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}
	for name, content := range input.Files {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 64 || len(content) > 128<<10 {
			return false
		}
	}
	return true
}
