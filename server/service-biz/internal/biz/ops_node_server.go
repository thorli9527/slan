package biz

import "net/http"

func (s *Server) opsListRelayNodes(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListRelayNodes()})
}

func (s *Server) opsCreateRelayNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsRelayNode
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Ops.UpsertRelayNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) opsUpdateRelayNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsRelayNode
	if !decodeJSON(w, r, &req) {
		return
	}
	req.NodeID = r.PathValue("nodeId")
	node, err := s.services.Ops.UpsertRelayNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) opsUpdateRelayNodeStatus(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Ops.UpdateRelayNodeStatus(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) opsDeleteRelayNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	if err := s.services.Ops.DeleteRelayNode(r.PathValue("nodeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) opsListPunchNodes(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListPunchNodes()})
}

func (s *Server) opsCreatePunchNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsPunchNode
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Ops.UpsertPunchNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) opsUpdatePunchNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsPunchNode
	if !decodeJSON(w, r, &req) {
		return
	}
	req.NodeID = r.PathValue("nodeId")
	node, err := s.services.Ops.UpsertPunchNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) opsUpdatePunchNodeStatus(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Ops.UpdatePunchNodeStatus(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) opsDeletePunchNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	if err := s.services.Ops.DeletePunchNode(r.PathValue("nodeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
