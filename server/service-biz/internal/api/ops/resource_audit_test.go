package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type resourceAuditStub struct {
	records []servicepkg.RecordOpsAuditEventInput
}

func (s *resourceAuditStub) ListAuditEvents(context.Context, int) ([]servicepkg.OpsAuditEventView, error) {
	return nil, nil
}

func (s *resourceAuditStub) RecordAuditEvent(_ context.Context, input servicepkg.RecordOpsAuditEventInput) error {
	s.records = append(s.records, input)
	return nil
}

func TestResourceAuditRecordsOnlySuccessfulMutations(t *testing.T) {
	audit := &resourceAuditStub{}
	routes := withResourceAudit([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/devices/{deviceId}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/dns/records/{recordId}", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "failed", http.StatusConflict) }),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	}, audit)

	request := httptest.NewRequest(http.MethodPost, "/api/ops/networks/net-1/devices/dev-1", nil)
	request.SetPathValue("networkId", "net-1")
	request.SetPathValue("deviceId", "dev-1")
	request = request.WithContext(serviceapi.WithAuthenticatedOperator(request.Context(), "operator-1"))
	findRouteHandler(t, routes, "POST /api/ops/networks/{networkId}/devices/{deviceId}")(httptest.NewRecorder(), request)

	failed := httptest.NewRequest(http.MethodDelete, "/api/ops/dns/records/record-1", nil)
	failed.SetPathValue("recordId", "record-1")
	findRouteHandler(t, routes, "DELETE /api/ops/dns/records/{recordId}")(httptest.NewRecorder(), failed)
	findRouteHandler(t, routes, "GET /api/ops/networks")(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ops/networks", nil))

	if len(audit.records) != 1 {
		t.Fatalf("expected one audit record, got %#v", audit.records)
	}
	got := audit.records[0]
	if got.ActorID != "operator-1" || got.Action != "create" || got.ResourceType != "network_device" || got.ResourceID != "dev-1" {
		t.Fatalf("unexpected audit record: %#v", got)
	}
}
