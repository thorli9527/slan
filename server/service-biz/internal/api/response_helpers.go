package api

import (
	"encoding/json"
	"errors"
	"net/http"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteEnvelope(w http.ResponseWriter, status int, key string, value any) {
	WriteJSON(w, status, map[string]any{key: value})
}

func WriteItems(w http.ResponseWriter, items any) {
	WriteEnvelope(w, http.StatusOK, "items", items)
}

func WriteItemsWithMembers(w http.ResponseWriter, items any, members any) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"items":   items,
		"members": members,
	})
}

func WriteOK(w http.ResponseWriter) {
	WriteEnvelope(w, http.StatusOK, "ok", true)
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func WriteError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, servicepkg.ErrInvalidArgument):
		status = http.StatusBadRequest
	case errors.Is(err, servicepkg.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, servicepkg.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, servicepkg.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, servicepkg.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, servicepkg.ErrNotImplemented):
		status = http.StatusNotImplemented
	}
	WriteJSON(w, status, map[string]any{"error": err.Error()})
}

func WriteNotImplemented(w http.ResponseWriter, layer, action string) {
	WriteJSON(w, http.StatusNotImplemented, map[string]any{
		"error":  "not implemented",
		"layer":  layer,
		"action": action,
	})
}
