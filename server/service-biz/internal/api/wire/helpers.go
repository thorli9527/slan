package wire

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func authorizeOrError(w http.ResponseWriter, r *http.Request, useCase servicepkg.WireUseCase) bool {
	if err := useCase.Authorize(r.Context(), r.Header.Get("X-Slan-Internal-Token")); err != nil {
		serviceapi.WriteError(w, err)
		return false
	}
	return true
}
