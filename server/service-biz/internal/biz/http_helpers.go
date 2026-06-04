package biz

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, errBadRequest)
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return auth
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	switch {
	case errors.Is(err, errBadRequest):
		status, code = http.StatusBadRequest, "BAD_REQUEST"
	case errors.Is(err, errUnauthorized):
		status, code = http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, errNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, errConflict):
		status, code = http.StatusConflict, "CONFLICT"
	case errors.Is(err, errUnavailable):
		status, code = http.StatusServiceUnavailable, "UNAVAILABLE"
	case errors.Is(err, errRateLimited):
		status, code = http.StatusTooManyRequests, "RATE_LIMITED"
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) recordRequestAudit(r *http.Request, event AuditEvent) {
	s.ensureServices()
	event.RemoteIP = clientIPFromRequest(r)
	s.services.Audit.Record(event)
}

func (s *Server) recordNetworkMutationAudit(r *http.Request, action, resourceType, resourceID, networkID, actorUserID, status string, details map[string]string) {
	if details == nil {
		details = map[string]string{}
	}
	if strings.TrimSpace(networkID) != "" {
		details["networkId"] = strings.TrimSpace(networkID)
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      strings.TrimSpace(actorUserID),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strings.TrimSpace(resourceID),
		Status:       status,
		Details:      details,
	})
}

func (s *Server) recordOpsAudit(r *http.Request, operator OperatorUser, action, resourceType, resourceID, status string, details map[string]string) {
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "operator",
		ActorID:      operator.OperatorID,
		ActorEmail:   operator.Email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strings.TrimSpace(resourceID),
		Status:       status,
		Details:      details,
	})
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func deviceLoginDeviceURL(deviceID string) string {
	base := strings.TrimSpace(os.Getenv("SLAN_WEB_CONSOLE_URL"))
	if base == "" {
		base = "http://47.245.40.231:24200/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	query.Set("auth", "login")
	if strings.TrimSpace(deviceID) != "" {
		query.Set("deviceId", deviceID)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
