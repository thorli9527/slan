package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ClientDownloadHandler struct {
	Downloads servicepkg.DownloadUseCase
}

func (h ClientDownloadHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/client-downloads", h.ListClientDownloads),
	}
}

func (h ClientDownloadHandler) ListClientDownloads(w http.ResponseWriter, r *http.Request) {
	items, err := h.Downloads.ListClientDownloads(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, clientDownloadPayload))
}
