package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkDNSHandler struct {
	NetworkDNS servicepkg.NetworkDNSUseCase
}

func (h NetworkDNSHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/dns/zones", h.ListDNSZones),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/dns/zones", h.AddDNSZone),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}/dns/zones/{zoneId}", h.UpdateDNSZone),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/dns/zones/{zoneId}", h.DeleteDNSZone),
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/dns/records", h.ListDNSRecords),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/dns/records", h.AddDNSRecord),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}/dns/records/{recordId}", h.UpdateDNSRecord),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/dns/records/{recordId}", h.DeleteDNSRecord),
	}
}

func (h NetworkDNSHandler) ListDNSZones(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkDNS.ListDNSZones(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, dnsZonePayload))
}

func (h NetworkDNSHandler) AddDNSZone(w http.ResponseWriter, r *http.Request) {
	var req createDNSZoneRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setNetworkAndActor(r, &input.NetworkID, &input.ActorUserID)
	item, err := h.NetworkDNS.AddDNSZone(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, dnsZonePayload(item))
}

func (h NetworkDNSHandler) UpdateDNSZone(w http.ResponseWriter, r *http.Request) {
	var req updateDNSZoneRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setZoneAndActor(r, &input.ZoneID, &input.ActorUserID)
	item, err := h.NetworkDNS.UpdateDNSZone(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, dnsZonePayload(item))
}

func (h NetworkDNSHandler) DeleteDNSZone(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteDNSZoneInput{
		ZoneID:      requestZoneID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkDNS.DeleteDNSZone(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

func (h NetworkDNSHandler) ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	networkID := requestNetworkID(r)
	items, err := h.NetworkDNS.ListDNSRecords(r.Context(), networkID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	zoneNames := h.dnsZoneNamesByID(r, networkID)
	serviceapi.WriteItems(w, dnsRecordPayloads(items, zoneNames))
}

func (h NetworkDNSHandler) AddDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req createDNSRecordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setNetworkAndActor(r, &input.NetworkID, &input.ActorUserID)
	item, err := h.NetworkDNS.AddDNSRecord(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, dnsRecordPayload(item, h.dnsZoneName(r, input.NetworkID, item.ZoneID)))
}

func (h NetworkDNSHandler) UpdateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req updateDNSRecordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setRecordAndActor(r, &input.RecordID, &input.ActorUserID)
	item, err := h.NetworkDNS.UpdateDNSRecord(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, dnsRecordPayload(item, h.dnsZoneName(r, requestNetworkID(r), item.ZoneID)))
}

func (h NetworkDNSHandler) DeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteDNSRecordInput{
		RecordID:    requestRecordID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkDNS.DeleteDNSRecord(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

func (h NetworkDNSHandler) dnsZoneNamesByID(r *http.Request, networkID string) map[string]string {
	zones, err := h.NetworkDNS.ListDNSZones(r.Context(), networkID)
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(zones))
	for _, zone := range zones {
		result[zone.ZoneID] = zone.Name
	}
	return result
}

func (h NetworkDNSHandler) dnsZoneName(r *http.Request, networkID, zoneID string) string {
	if zoneID == "" {
		return ""
	}
	return h.dnsZoneNamesByID(r, networkID)[zoneID]
}
