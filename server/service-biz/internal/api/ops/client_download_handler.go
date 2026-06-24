package ops

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	"github.com/slan/service-biz/internal/pkg/downloadkit"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ClientDownloadHandler struct {
	OpsCatalogDownloads servicepkg.OpsCatalogDownloadUseCase
}

func (h ClientDownloadHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/client-downloads", h.OpsListClientDownloads),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/client-downloads", h.OpsUploadClientDownload),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/client-downloads/{downloadId}", h.OpsDeleteClientDownload),
	})
}

func (h ClientDownloadHandler) OpsListClientDownloads(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCatalogDownloads.ListClientDownloads(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, clientDownloadPayload))
}

func (h ClientDownloadHandler) OpsUploadClientDownload(w http.ResponseWriter, r *http.Request) {
	input, ok := h.decodeUploadRequest(w, r)
	if !ok {
		return
	}
	item, err := h.OpsCatalogDownloads.UpsertClientDownload(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, clientDownloadPayload(item))
}

func (h ClientDownloadHandler) decodeUploadRequest(w http.ResponseWriter, r *http.Request) (servicepkg.UpsertClientDownloadInput, bool) {
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return h.decodeMultipartUploadRequest(w, r)
	}
	var req upsertClientDownloadRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return servicepkg.UpsertClientDownloadInput{}, false
	}
	return req.toInput(), true
}

func (h ClientDownloadHandler) decodeMultipartUploadRequest(w http.ResponseWriter, r *http.Request) (servicepkg.UpsertClientDownloadInput, bool) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return servicepkg.UpsertClientDownloadInput{}, false
	}
	input := servicepkg.UpsertClientDownloadInput{
		Platform: r.FormValue("platform"),
		Version:  r.FormValue("version"),
		Status:   r.FormValue("status"),
	}
	if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()
		input.Name = header.Filename
		storedName, _, saveErr := downloadkit.SaveClientDownload(file, header.Filename, input.Platform, input.Version)
		if saveErr != nil {
			serviceapi.WriteError(w, saveErr)
			return servicepkg.UpsertClientDownloadInput{}, false
		}
		input.URL = "/downloads/clients/" + storedName
	}
	return input, true
}

func (h ClientDownloadHandler) OpsDeleteClientDownload(w http.ResponseWriter, r *http.Request) {
	downloadID := requestDownloadID(r)
	items, err := h.OpsCatalogDownloads.ListClientDownloads(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	var storedFile string
	for _, item := range items {
		if item.DownloadID != downloadID {
			continue
		}
		storedFile = filepath.Base(strings.TrimSpace(item.URL))
		break
	}
	if err := h.OpsCatalogDownloads.DeleteClientDownload(r.Context(), downloadID); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	if storedFile != "" {
		if removeErr := downloadkit.RemoveClientDownload(storedFile); removeErr != nil && !os.IsNotExist(removeErr) {
			serviceapi.WriteError(w, removeErr)
			return
		}
	}
	serviceapi.WriteNoContent(w)
}
