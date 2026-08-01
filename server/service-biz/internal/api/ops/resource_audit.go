package ops

import (
	"log"
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type auditStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditStatusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func withResourceAudit(routes []serviceapi.Route, audit servicepkg.OpsAuditUseCase) []serviceapi.Route {
	if audit == nil {
		return routes
	}
	out := make([]serviceapi.Route, 0, len(routes))
	for _, route := range routes {
		current := route
		out = append(out, serviceapi.NewRoute(current.Method, current.Path, func(w http.ResponseWriter, r *http.Request) {
			writer := &auditStatusWriter{ResponseWriter: w}
			current.Handler(writer, r)
			status := writer.status
			if status == 0 {
				status = http.StatusOK
			}
			if current.Method == http.MethodGet || status < 200 || status >= 300 {
				return
			}
			input := resourceAuditInput(r, current)
			if err := audit.RecordAuditEvent(r.Context(), input); err != nil {
				log.Printf("ops resource audit failed action=%s resourceType=%s resourceId=%s err=%v", input.Action, input.ResourceType, input.ResourceID, err)
			}
		}))
	}
	return out
}

func resourceAuditInput(r *http.Request, route serviceapi.Route) servicepkg.RecordOpsAuditEventInput {
	resourceType := "network"
	switch {
	case strings.Contains(route.Path, "/dns/"):
		resourceType = "dns"
	case strings.Contains(route.Path, "/security-"):
		resourceType = "security"
	case strings.Contains(route.Path, "/device-groups") || strings.HasSuffix(route.Path, "/groups"):
		resourceType = "device_group"
	case strings.Contains(route.Path, "/devices/"):
		resourceType = "network_device"
	}
	resourceID := serviceapi.FirstNonEmpty(r.PathValue("recordId"), r.PathValue("zoneId"), r.PathValue("ruleId"), r.PathValue("securityGroupId"), r.PathValue("groupId"), r.PathValue("deviceId"), r.PathValue("networkId"))
	action := map[string]string{http.MethodPost: "create", http.MethodPatch: "update", http.MethodPut: "update", http.MethodDelete: "delete"}[route.Method]
	return servicepkg.RecordOpsAuditEventInput{ActorID: serviceapi.AuthenticatedOperatorID(r.Context()), Action: action, ResourceType: resourceType, ResourceID: resourceID, Status: "success"}
}
