package biz

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Server struct {
	store *Store
}

func NewServer() *Server {
	return &Server{store: NewStore()}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/register", s.registerUser)
	mux.HandleFunc("POST /api/auth/login", s.loginUser)
	mux.HandleFunc("GET /api/users", s.listUsers)
	mux.HandleFunc("PATCH /api/users/{userId}/password", s.changeUserPassword)

	mux.HandleFunc("GET /api/devices", s.listDevices)
	mux.HandleFunc("POST /api/devices/register", s.registerDevice)
	mux.HandleFunc("GET /api/devices/owner-logs", s.listDeviceOwnerLogs)
	mux.HandleFunc("GET /api/ipam/addresses", s.listIPAddresses)
	mux.HandleFunc("GET /api/ipam/subnets", s.listIPAMSubnets)
	mux.HandleFunc("GET /api/dns/global", s.globalDNS)

	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}", s.updateWorkspace)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/members", s.listWorkspaceMembers)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/invites", s.inviteWorkspaceMember)
	mux.HandleFunc("POST /api/workspaces/invites/{inviteId}/accept", s.acceptWorkspaceInvite)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/device-invites", s.createWorkspaceDeviceInvite)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/device-invites", s.listWorkspaceDeviceInvites)
	mux.HandleFunc("POST /api/workspaces/device-invites/accept", s.acceptWorkspaceDeviceInvite)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/devices", s.listWorkspaceDevices)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/devices", s.addWorkspaceDevice)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/devices/{deviceId}", s.updateWorkspaceDevice)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/dns/zones", s.listDNSZones)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/dns/zones", s.addDNSZone)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/dns/records", s.listDNSRecords)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/dns/records", s.addDNSRecord)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/security-groups", s.listSecurityGroups)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/security-groups", s.createSecurityGroup)
	mux.HandleFunc("GET /api/security-groups/{securityGroupId}/rules", s.listSecurityRules)
	mux.HandleFunc("POST /api/security-groups/{securityGroupId}/rules", s.addSecurityRule)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/network-config", s.networkConfig)
	return withCORS(mux)
}

func (s *Server) registerUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, member, workspace, err := s.store.RegisterUser(req.Email, req.Password, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"auth": auth, "defaultWorkspaceMember": member, "defaultWorkspace": workspace})
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.store.LoginUser(req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ChangeUserPassword(r.PathValue("userId"), req.OldPassword, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListUsers()})
}

func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID    string `json:"userId"`
		DeviceID  string `json:"deviceId"`
		Name      string `json:"name"`
		Platform  string `json:"platform"`
		OSName    string `json:"osName"`
		OSVersion string `json:"osVersion"`
		Alias     string `json:"alias"`
		PublicKey string `json:"publicKey"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	device, membership, err := s.store.RegisterDevice(req.UserID, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"device": device, "defaultWorkspaceDevice": membership})
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) listDeviceOwnerLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDeviceOwnerLogs(r.URL.Query().Get("deviceId"))})
}

func (s *Server) listIPAddresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListGlobalIPs()})
}

func (s *Server) listIPAMSubnets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListIPAMSubnets()})
}

func (s *Server) globalDNS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.GlobalDNS()})
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListWorkspaces(r.URL.Query().Get("userId"))})
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OwnerUserID string `json:"ownerUserId"`
		Name        string `json:"name"`
		Code        string `json:"code"`
		TemplateKey string `json:"templateKey"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	workspace, member, group, zone, err := s.store.CreateWorkspace(req.OwnerUserID, req.Name, req.Code, req.TemplateKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": workspace, "ownerMember": member, "defaultSecurityGroup": group, "defaultDNSZone": zone})
}

func (s *Server) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	workspace, err := s.store.UpdateWorkspace(r.PathValue("workspaceId"), req.Name, req.Status)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (s *Server) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListWorkspaceMembers(r.PathValue("workspaceId"))})
}

func (s *Server) inviteWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviterUserID string `json:"inviterUserId"`
		Email         string `json:"email"`
		Role          string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	invite, err := s.store.InviteWorkspaceMember(r.PathValue("workspaceId"), req.InviterUserID, req.Email, req.Role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (s *Server) acceptWorkspaceInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"userId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	member, err := s.store.AcceptWorkspaceInvite(r.PathValue("inviteId"), req.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) createWorkspaceDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviterUserID string `json:"inviterUserId"`
		TTLSeconds    int64  `json:"ttlSeconds"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	invite, err := s.store.CreateWorkspaceDeviceInvite(r.PathValue("workspaceId"), req.InviterUserID, req.TTLSeconds)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (s *Server) listWorkspaceDeviceInvites(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListWorkspaceDeviceInvites(r.PathValue("workspaceId"))})
}

func (s *Server) acceptWorkspaceDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"inviteCode"`
		DeviceID   string `json:"deviceId"`
		Alias      string `json:"alias"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	device, invite, err := s.store.AcceptWorkspaceDeviceInvite(req.InviteCode, req.DeviceID, req.Alias)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaceDevice": device, "invite": invite})
}

func (s *Server) listWorkspaceDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListWorkspaceDevices(r.PathValue("workspaceId"))})
}

func (s *Server) addWorkspaceDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID    string `json:"deviceId"`
		ActorUserID string `json:"actorUserId"`
		Alias       string `json:"alias"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	device, err := s.store.AddWorkspaceDevice(r.PathValue("workspaceId"), req.DeviceID, req.ActorUserID, req.Alias, enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, device)
}

func (s *Server) updateWorkspaceDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias string `json:"alias"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.store.UpdateWorkspaceDeviceAlias(r.PathValue("workspaceId"), r.PathValue("deviceId"), req.Alias)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) listDNSZones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSZones(r.PathValue("workspaceId"))})
}

func (s *Server) addDNSZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneName     string `json:"zoneName"`
		ExposeGlobal bool   `json:"exposeGlobal"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.store.AddDNSZone(r.PathValue("workspaceId"), req.ZoneName, req.ExposeGlobal)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, zone)
}

func (s *Server) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSRecords(r.PathValue("workspaceId"))})
}

func (s *Server) addDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneID         string `json:"zoneId"`
		Name           string `json:"name"`
		RecordType     string `json:"recordType"`
		TargetDeviceID string `json:"targetDeviceId"`
		TargetIP       string `json:"targetIp"`
		CNAME          string `json:"cname"`
		TTL            int    `json:"ttl"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.store.AddDNSRecord(r.PathValue("workspaceId"), req.ZoneID, req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.TTL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) listSecurityGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroups(r.PathValue("workspaceId"))})
}

func (s *Server) createSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		DefaultPolicy string `json:"defaultPolicy"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	group, err := s.store.CreateSecurityGroup(r.PathValue("workspaceId"), req.Name, req.Description, req.DefaultPolicy)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroupRules(r.PathValue("securityGroupId"))})
}

func (s *Server) addSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Direction   string `json:"direction"`
		Priority    int    `json:"priority"`
		Action      string `json:"action"`
		Protocol    string `json:"protocol"`
		PortFrom    int    `json:"portFrom"`
		PortTo      int    `json:"portTo"`
		PeerType    string `json:"peerType"`
		PeerValue   string `json:"peerValue"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.AddSecurityGroupRule(r.PathValue("securityGroupId"), req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) networkConfig(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	if deviceID == "" {
		writeError(w, errBadRequest)
		return
	}
	config, err := s.store.NetworkConfig(r.PathValue("workspaceId"), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, errBadRequest)
		return false
	}
	return true
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
	case errors.Is(err, errNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, errConflict):
		status, code = http.StatusConflict, "CONFLICT"
	}
	writeJSON(w, status, map[string]string{"error": code})
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
