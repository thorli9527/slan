package biz

import (
	"net/http"
)

func (s *Server) listDNSZones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListDNSZones(r.PathValue("networkId"))})
}

func (s *Server) addDNSZone(w http.ResponseWriter, r *http.Request) {
	var req DNSZoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.services.Network.AddDNSZone(r.PathValue("networkId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.add", "dns_zone", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "zoneName": req.ZoneName})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.add", "dns_zone", zone.ZoneID, zone.NetworkID, "", "succeeded", map[string]string{"zoneName": zone.ZoneName, "exposeGlobal": boolString(zone.ExposeGlobal)})
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_added", "dns_zone", "add", zone.ZoneID, "")
	writeJSON(w, http.StatusCreated, zone)
}

func (s *Server) updateDNSZone(w http.ResponseWriter, r *http.Request) {
	var req DNSZoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.services.Network.UpdateDNSZone(r.PathValue("networkId"), r.PathValue("zoneId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.update", "dns_zone", r.PathValue("zoneId"), r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "zoneName": req.ZoneName})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.update", "dns_zone", zone.ZoneID, zone.NetworkID, "", "succeeded", map[string]string{"zoneName": zone.ZoneName, "exposeGlobal": boolString(zone.ExposeGlobal)})
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_updated", "dns_zone", "update", zone.ZoneID, "")
	writeJSON(w, http.StatusOK, zone)
}

func (s *Server) deleteDNSZone(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	zoneID := r.PathValue("zoneId")
	if err := s.services.Network.DeleteDNSZone(networkID, zoneID); err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.delete", "dns_zone", zoneID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.delete", "dns_zone", zoneID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "dns_zone_removed", "dns_zone", "remove", zoneID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListDNSRecords(r.PathValue("networkId"))})
}

func (s *Server) addDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req AddDNSRecordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.services.Network.AddDNSRecord(r.PathValue("networkId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.add", "dns_record", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "recordType": req.RecordType})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.add", "dns_record", record.RecordID, record.NetworkID, "", "succeeded", map[string]string{"fqdn": record.FQDN, "recordType": record.RecordType, "targetDeviceId": record.TargetDeviceID})
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_added", "dns_record", "add", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) updateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req DNSRecordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.services.Network.UpdateDNSRecord(r.PathValue("networkId"), r.PathValue("recordId"), req)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.update", "dns_record", r.PathValue("recordId"), r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "recordType": req.RecordType})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.update", "dns_record", record.RecordID, record.NetworkID, "", "succeeded", map[string]string{"fqdn": record.FQDN, "recordType": record.RecordType, "targetDeviceId": record.TargetDeviceID})
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_updated", "dns_record", "update", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) deleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	recordID := r.PathValue("recordId")
	if err := s.services.Network.DeleteDNSRecord(networkID, recordID); err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.delete", "dns_record", recordID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.delete", "dns_record", recordID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "dns_record_removed", "dns_record", "remove", recordID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
