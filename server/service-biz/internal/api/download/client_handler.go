package download

import (
	"net/http"
	"os"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ClientHandler struct {
	Downloads servicepkg.DownloadUseCase
}

func (h ClientHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/downloads/clients/{fileName}", h.DownloadClientFile),
	}
}

func Routes(useCase servicepkg.DownloadUseCase) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		ClientHandler{Downloads: useCase}.Routes(),
	)
}

func (h ClientHandler) DownloadClientFile(w http.ResponseWriter, r *http.Request) {
	view, err := h.Downloads.GetClientDownload(r.Context(), r.PathValue("fileName"), r.URL.Path)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	if !view.Found {
		http.NotFound(w, r)
		return
	}
	if view.Script != "" {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		_, _ = w.Write([]byte(view.Script))
		return
	}
	if view.Path != "" {
		if _, err := os.Stat(view.Path); err == nil {
			http.ServeFile(w, r, view.Path)
			return
		}
	}
	if view.URL != "" {
		if view.URL == r.URL.Path {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, view.URL, http.StatusTemporaryRedirect)
		return
	}
	http.NotFound(w, r)
}
