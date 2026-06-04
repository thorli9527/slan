package biz

import (
	"net/http"
	"strings"
)

func (s *Server) listPublicMappings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListPublicMappings(r.PathValue("networkId"))})
}

func (s *Server) createPublicMapping(w http.ResponseWriter, r *http.Request) {
	mapping, ok := s.decodePublicMapping(w, r, "")
	if !ok {
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.add", "public_mapping", mapping.MappingID, mapping.NetworkID, "", "succeeded", map[string]string{"deviceId": mapping.DeviceID, "publicDomain": mapping.PublicDomain, "protocol": mapping.Protocol})
	writeJSON(w, http.StatusCreated, mapping)
}

func (s *Server) updatePublicMapping(w http.ResponseWriter, r *http.Request) {
	mapping, ok := s.decodePublicMapping(w, r, r.PathValue("mappingId"))
	if !ok {
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.update", "public_mapping", mapping.MappingID, mapping.NetworkID, "", "succeeded", map[string]string{"deviceId": mapping.DeviceID, "publicDomain": mapping.PublicDomain, "protocol": mapping.Protocol, "status": mapping.Status})
	writeJSON(w, http.StatusOK, mapping)
}

func (s *Server) decodePublicMapping(w http.ResponseWriter, r *http.Request, mappingID string) (PublicDomainMapping, bool) {
	var req PublicMappingRequest
	if !decodeJSON(w, r, &req) {
		return PublicDomainMapping{}, false
	}
	mapping, err := s.services.Network.UpsertPublicMapping(r.PathValue("networkId"), mappingID, req)
	if err != nil {
		action := "public_mapping.add"
		if strings.TrimSpace(mappingID) != "" {
			action = "public_mapping.update"
		}
		s.recordNetworkMutationAudit(r, action, "public_mapping", mappingID, r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "deviceId": req.DeviceID, "publicDomain": req.PublicDomain, "protocol": req.Protocol})
		writeError(w, err)
		return PublicDomainMapping{}, false
	}
	return mapping, true
}

func (s *Server) deletePublicMapping(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	mappingID := r.PathValue("mappingId")
	if err := s.services.Network.DeletePublicMapping(networkID, mappingID); err != nil {
		s.recordNetworkMutationAudit(r, "public_mapping.delete", "public_mapping", mappingID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.delete", "public_mapping", mappingID, networkID, "", "succeeded", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
