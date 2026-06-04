package biz

import (
	"net/http"
	"strconv"
)

func (s *Server) opsLogin(w http.ResponseWriter, r *http.Request) {
	var req OpsLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.services.Ops.Login(req, clientIPFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) opsChangePassword(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req OpsChangePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.services.Ops.ChangePassword(operator.OperatorID, req); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) opsDashboard(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.services.Ops.Dashboard())
}

func (s *Server) opsListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	events, err := s.services.Ops.AuditEvents(AuditEventFilter{
		ActorType:    query.Get("actorType"),
		ActorID:      query.Get("actorId"),
		Action:       query.Get("action"),
		ResourceType: query.Get("resourceType"),
		ResourceID:   query.Get("resourceId"),
		Status:       query.Get("status"),
		Limit:        limit,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) opsListOperators(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListOperators()})
}

func (s *Server) opsCreateOperator(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OperatorUser
	if !decodeJSON(w, r, &req) {
		return
	}
	operator, err := s.services.Ops.UpsertOperator(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, operator)
}

func (s *Server) opsUpdateOperator(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OperatorUser
	if !decodeJSON(w, r, &req) {
		return
	}
	req.OperatorID = r.PathValue("operatorId")
	operator, err := s.services.Ops.UpsertOperator(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, operator)
}

func (s *Server) opsSetOperatorPassword(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsSetOperatorPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.services.Ops.SetOperatorPassword(r.PathValue("operatorId"), req); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
