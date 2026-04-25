package serverbiztest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	httpapi "github.com/slan/server/server-biz/api/http"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

func TestPhase1Flow(t *testing.T) {
	app := newPhase1TestApp()

	auth := postAuth(t, app, "/auth/register", "", map[string]string{
		"email":    "owner@example.com",
		"password": "OwnerPass-2026",
	}, http.StatusCreated, &dto.AuthResponse{})
	login := postAuth(t, app, "/auth/login", "", map[string]string{
		"email":    "owner@example.com",
		"password": "OwnerPass-2026",
	}, http.StatusOK, &dto.AuthResponse{})
	if auth.UserID != login.UserID {
		t.Fatalf("expected login user %s to match registered user %s", login.UserID, auth.UserID)
	}
	refreshed := postAuth(t, app, "/auth/refresh", "", dto.RefreshTokenRequest{
		RefreshToken: login.RefreshToken,
	}, http.StatusOK, &dto.AuthResponse{})
	if refreshed.UserID != auth.UserID || refreshed.AccessToken == "" {
		t.Fatalf("expected refresh to return new auth response, got %+v", refreshed)
	}

	device := postAuth(t, app, "/devices/register", login.AccessToken, dto.RegisterDeviceRequest{
		Name:      "Owner Laptop",
		Platform:  "windows",
		MachineID: "owner-machine",
		PublicKey: "owner-device-key",
	}, http.StatusCreated, &dto.Device{})
	node := postAuth(t, app, "/nodes/register", login.AccessToken, dto.RegisterNodeRequest{
		DeviceID:      device.DeviceID,
		NodeID:        "owner-node",
		NodePublicKey: "owner-node-key",
	}, http.StatusCreated, &dto.Node{})
	network := postAuth(t, app, "/networks", login.AccessToken, dto.CreateNetworkRequest{
		Name:         "Owner Network",
		CIDR:         "100.64.0.0/24",
		BindDeviceID: device.DeviceID,
	}, http.StatusCreated, &dto.Network{})
	if network.NetworkID == "" || network.DefaultSubnetID == "" {
		t.Fatalf("expected created network with default subnet, got %+v", network)
	}

	bootstrap := postAuth(t, app, "/bootstrap", login.AccessToken, dto.BootstrapRequest{
		NodeID:    node.NodeID,
		NetworkID: network.NetworkID,
	}, http.StatusOK, &dto.BootstrapResponse{})
	if bootstrap.SessionToken == "" || bootstrap.ControlSessionID == "" {
		t.Fatalf("expected bootstrap control session, got %+v", bootstrap)
	}
	if bootstrap.Device.Device.DeviceID != device.DeviceID || len(bootstrap.Device.Attachments) != 1 {
		t.Fatalf("expected bootstrap device attachment, got %+v", bootstrap.Device)
	}
	if bootstrap.NetworkMap.NetworkID != network.NetworkID || bootstrap.NetworkMap.SelfNodeID != node.NodeID {
		t.Fatalf("expected bootstrap network map for node/network, got %+v", bootstrap.NetworkMap)
	}
}

func TestPhase1JoinExistingNetworkFlow(t *testing.T) {
	app := newPhase1TestApp()

	owner := app.createUserDeviceNodeNetwork(t, "owner@example.com", "owner")
	if putAuth(t, app, "/networks/"+owner.networkID+"/join-key", owner.accessToken, dto.UpdateNetworkJoinKeyRequest{
		JoinKey: "shared-join-key",
	}, http.StatusOK, &dto.NetworkDetail{}).JoinKey != "shared-join-key" {
		t.Fatalf("expected owner join key to be returned")
	}
	dnsDetail := putAuth(t, app, "/networks/"+owner.networkID+"/dns", owner.accessToken, dto.UpdateNetworkDNSRequest{
		Servers:       []string{"100.100.100.100"},
		SearchDomains: []string{"slan.local"},
	}, http.StatusOK, &dto.NetworkDetail{})
	if len(dnsDetail.DNS.Servers) != 1 || dnsDetail.DNS.Servers[0] != "100.100.100.100" {
		t.Fatalf("expected owner DNS update to be returned, got %+v", dnsDetail.DNS)
	}

	guestAuth := postAuth(t, app, "/auth/register", "", map[string]string{
		"email":    "guest@example.com",
		"password": "GuestPass-2026",
	}, http.StatusCreated, &dto.AuthResponse{})
	guestDevice := postAuth(t, app, "/devices/register", guestAuth.AccessToken, dto.RegisterDeviceRequest{
		Name:      "Guest Laptop",
		Platform:  "linux",
		MachineID: "guest-machine",
		PublicKey: "guest-device-key",
	}, http.StatusCreated, &dto.Device{})
	guestNode := postAuth(t, app, "/nodes/register", guestAuth.AccessToken, dto.RegisterNodeRequest{
		DeviceID:      guestDevice.DeviceID,
		NodeID:        "guest-node",
		NodePublicKey: "guest-node-key",
	}, http.StatusCreated, &dto.Node{})

	byEmail := postAuth(t, app, "/networks/join-by-owner-email", guestAuth.AccessToken, dto.JoinNetworkByOwnerEmailRequest{
		OwnerEmail: "owner@example.com",
		DeviceID:   guestDevice.DeviceID,
	}, http.StatusOK, &dto.NetworkJoinByOwnerEmailResult{})
	if byEmail.Network.NetworkID != owner.networkID || byEmail.Attachment.VirtualIP == "" {
		t.Fatalf("expected join-by-owner-email to activate guest device, got %+v", byEmail)
	}
	remark := putAuth(t, app, "/networks/"+owner.networkID+"/attachments/"+byEmail.Attachment.AttachmentID+"/remark", guestAuth.AccessToken, dto.UpdateAttachmentRemarkRequest{
		Remark: "Guest Laptop Alias",
	}, http.StatusOK, &dto.NetworkAssignment{})
	if remark.Remark != "Guest Laptop Alias" || remark.DeviceID != guestDevice.DeviceID {
		t.Fatalf("expected guest to update own attachment remark, got %+v", remark)
	}

	byKey := postAuth(t, app, "/networks/join-by-key", guestAuth.AccessToken, dto.JoinNetworkByKeyRequest{
		JoinKey:  "shared-join-key",
		DeviceID: guestDevice.DeviceID,
	}, http.StatusOK, &dto.NetworkJoinResult{})
	if byKey.Member.NetworkID != owner.networkID || byKey.Attachment.AttachmentID != byEmail.Attachment.AttachmentID {
		t.Fatalf("expected join-by-key to be idempotent for same device, got %+v", byKey)
	}

	visibleNetworks := getAuth(t, app, "/networks", guestAuth.AccessToken, http.StatusOK, &struct {
		Items []dto.Network `json:"items"`
	}{})
	if !hasNetworkID(visibleNetworks.Items, owner.networkID) {
		t.Fatalf("expected guest visible networks to include joined network %s, got %+v", owner.networkID, visibleNetworks.Items)
	}

	explicitJoin := postAuth(t, app, "/networks/"+owner.networkID+"/join", guestAuth.AccessToken, dto.JoinNetworkRequest{
		DeviceID: guestDevice.DeviceID,
	}, http.StatusOK, &dto.NetworkJoinResult{})
	if explicitJoin.Attachment.AttachmentID != byEmail.Attachment.AttachmentID {
		t.Fatalf("expected explicit join to reuse existing attachment, got %+v", explicitJoin)
	}

	reactivated := postAuth(t, app, "/networks/"+owner.networkID+"/activate", guestAuth.AccessToken, dto.JoinNetworkRequest{
		DeviceID: guestDevice.DeviceID,
	}, http.StatusOK, &dto.NetworkJoinResult{})
	if reactivated.Attachment.AttachmentID != byEmail.Attachment.AttachmentID {
		t.Fatalf("expected activate to reuse existing membership and attachment, got %+v", reactivated)
	}

	bootstrap := postAuth(t, app, "/bootstrap", guestAuth.AccessToken, dto.BootstrapRequest{
		NodeID:    guestNode.NodeID,
		NetworkID: owner.networkID,
	}, http.StatusOK, &dto.BootstrapResponse{})
	if bootstrap.NetworkMap.NetworkID != owner.networkID || bootstrap.NetworkMap.SelfNodeID != guestNode.NodeID {
		t.Fatalf("expected guest bootstrap for owner network, got %+v", bootstrap.NetworkMap)
	}
}

func TestBootstrapPreconditions(t *testing.T) {
	app := newPhase1TestApp()
	user := app.createUserDeviceNodeNetwork(t, "preconditions@example.com", "pre")

	tests := []struct {
		name string
		body dto.BootstrapRequest
		code int
	}{
		{name: "missing node", body: dto.BootstrapRequest{NetworkID: user.networkID}, code: http.StatusBadRequest},
		{name: "unknown node", body: dto.BootstrapRequest{NodeID: "missing-node", NetworkID: user.networkID}, code: http.StatusNotFound},
		{name: "unknown network", body: dto.BootstrapRequest{NodeID: user.nodeID, NetworkID: "missing-network"}, code: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errResp dto.ErrorResponse
			postAuth(t, app, "/bootstrap", user.accessToken, tt.body, tt.code, &errResp)
			if errResp.Code == "" {
				t.Fatalf("expected structured error response")
			}
		})
	}

	detached := app.createUserDeviceNode(t, "detached@example.com", "detached")
	var errResp dto.ErrorResponse
	postAuth(t, app, "/bootstrap", detached.accessToken, dto.BootstrapRequest{
		NodeID:    detached.nodeID,
		NetworkID: user.networkID,
	}, http.StatusForbidden, &errResp)
	if errResp.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN for non-member bootstrap, got %+v", errResp)
	}
}

func TestRelayTicketFlow(t *testing.T) {
	app := newPhase1TestApp()
	owner := app.createUserDeviceNodeNetwork(t, "relay-owner@example.com", "relay-owner")
	peer := app.createUserDeviceNode(t, "relay-peer@example.com", "relay-peer")
	postAuth(t, app, "/networks/"+owner.networkID+"/join", peer.accessToken, dto.JoinNetworkRequest{
		DeviceID: peer.deviceID,
	}, http.StatusOK, &dto.NetworkJoinResult{})

	req := dto.RelayTicketRequest{
		NetworkID:            owner.networkID,
		SrcNodeID:            owner.nodeID,
		DstNodeID:            peer.nodeID,
		PreferredDerpNodeIDs: []string{"derp-b", "missing", "derp-a"},
		Reason:               "direct path failed",
		DerpClusterID:        "cluster-a",
	}
	first := postAuth(t, app, "/relay/tickets", owner.accessToken, req, http.StatusCreated, &dto.RelayTicket{})
	second := postAuth(t, app, "/relay/tickets", owner.accessToken, req, http.StatusCreated, &dto.RelayTicket{})
	if first.TicketID == "" || first.Signature == "" || first.RelayURL == "" {
		t.Fatalf("expected signed relay ticket, got %+v", first)
	}
	if first.TicketID != second.TicketID {
		t.Fatalf("expected identical relay ticket from cache, got %s then %s", first.TicketID, second.TicketID)
	}
	if got := strings.Join(first.AllowedDerpNodeIDs, ","); got != "derp-a,derp-b" {
		t.Fatalf("expected normalized relay node ids, got %q", got)
	}

	var errResp dto.ErrorResponse
	other := app.createUserDeviceNodeNetwork(t, "relay-other@example.com", "relay-other")
	postAuth(t, app, "/relay/tickets", owner.accessToken, dto.RelayTicketRequest{
		NetworkID: other.networkID,
		SrcNodeID: owner.nodeID,
		DstNodeID: peer.nodeID,
		Reason:    "direct path failed",
	}, http.StatusForbidden, &errResp)
	if errResp.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden for unrelated network ticket, got %+v", errResp)
	}
}

type phase1Actor struct {
	accessToken string
	deviceID    string
	nodeID      string
	networkID   string
}

type phase1TestApp struct {
	router http.Handler
	svc    *phase1Services
}

func newPhase1TestApp() *phase1TestApp {
	gin.SetMode(gin.TestMode)
	svc := newPhase1Services()
	cfg := configs.DefaultConfig()
	deps := httpapi.NewRouterDeps(
		phase1AuthService{svc: svc},
		phase1DeviceService{svc: svc},
		svc,
		phase1NodeService{svc: svc},
		svc,
		svc,
		svc,
		nil,
		nil,
		nil,
	)
	return &phase1TestApp{
		router: httpapi.NewPublicRouter(cfg, deps),
		svc:    svc,
	}
}

func (app *phase1TestApp) createUserDeviceNodeNetwork(t *testing.T, email, prefix string) phase1Actor {
	t.Helper()
	actor := app.createUserDeviceNode(t, email, prefix)
	network := postAuth(t, app, "/networks", actor.accessToken, dto.CreateNetworkRequest{
		Name:         prefix + " network",
		CIDR:         "100.64.0.0/24",
		BindDeviceID: actor.deviceID,
	}, http.StatusCreated, &dto.Network{})
	actor.networkID = network.NetworkID
	return actor
}

func (app *phase1TestApp) createUserDeviceNode(t *testing.T, email, prefix string) phase1Actor {
	t.Helper()
	auth := postAuth(t, app, "/auth/register", "", map[string]string{
		"email":    email,
		"password": prefix + "-Pass-2026",
	}, http.StatusCreated, &dto.AuthResponse{})
	device := postAuth(t, app, "/devices/register", auth.AccessToken, dto.RegisterDeviceRequest{
		Name:      prefix + " device",
		Platform:  "windows",
		MachineID: prefix + "-machine",
		PublicKey: prefix + "-device-key",
	}, http.StatusCreated, &dto.Device{})
	nodeID := prefix + "-node"
	node := postAuth(t, app, "/nodes/register", auth.AccessToken, dto.RegisterNodeRequest{
		DeviceID:      device.DeviceID,
		NodeID:        nodeID,
		NodePublicKey: prefix + "-node-key",
	}, http.StatusCreated, &dto.Node{})
	return phase1Actor{
		accessToken: auth.AccessToken,
		deviceID:    device.DeviceID,
		nodeID:      node.NodeID,
	}
}

func putAuth[T any](t *testing.T, app *phase1TestApp, path, accessToken string, body any, wantStatus int, out *T) T {
	t.Helper()
	return requestAuth(t, app, http.MethodPut, path, accessToken, body, wantStatus, out)
}

func postAuth[T any](t *testing.T, app *phase1TestApp, path, accessToken string, body any, wantStatus int, out *T) T {
	t.Helper()
	return requestAuth(t, app, http.MethodPost, path, accessToken, body, wantStatus, out)
}

func getAuth[T any](t *testing.T, app *phase1TestApp, path, accessToken string, wantStatus int, out *T) T {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("GET %s expected %d, got %d body=%s", path, wantStatus, rec.Code, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response for %s: %v body=%s", path, err, rec.Body.String())
		}
		return *out
	}
	var zero T
	return zero
}

func requestAuth[T any](t *testing.T, app *phase1TestApp, method, path, accessToken string, body any, wantStatus int, out *T) T {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s expected %d, got %d body=%s", method, path, wantStatus, rec.Code, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response for %s: %v body=%s", path, err, rec.Body.String())
		}
		return *out
	}
	var zero T
	return zero
}

func hasNetworkID(networks []dto.Network, networkID string) bool {
	for _, network := range networks {
		if network.NetworkID == networkID {
			return true
		}
	}
	return false
}

type phase1AuthService struct {
	svc *phase1Services
}

func (s phase1AuthService) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	return s.svc.Register(req)
}

func (s phase1AuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	return s.svc.Login(req)
}

func (s phase1AuthService) Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
	return s.svc.Refresh(req)
}

func (s phase1AuthService) GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error) {
	return s.svc.GetCallbackStatus(callbackID)
}

func (s phase1AuthService) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	return s.svc.CompleteCallback(callbackID, req)
}

func (s phase1AuthService) MarkCallbackReceived(callbackID string) error {
	return s.svc.MarkCallbackReceived(callbackID)
}

type phase1DeviceService struct {
	svc *phase1Services
}

func (s phase1DeviceService) Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error) {
	return s.svc.registerDevice(userID, req)
}

func (s phase1DeviceService) ListByUser(userID string) ([]dto.Device, error) {
	return s.svc.ListByUser(userID)
}

type phase1NodeService struct {
	svc *phase1Services
}

func (s phase1NodeService) Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error) {
	return s.svc.registerNode(userID, req)
}

type phase1Services struct {
	mu sync.Mutex

	nextID int

	usersByID     map[string]*phase1User
	usersByEmail  map[string]*phase1User
	accessTokens  map[string]string
	refreshTokens map[string]string
	devices       map[string]*phase1Device
	nodes         map[string]*phase1Node
	networks      map[string]*phase1Network
	members       map[string]map[string]*dto.NetworkMember
	attachments   map[string]map[string]*dto.SubnetAttachment
	relayTickets  map[string]dto.RelayTicket
}

type phase1User struct {
	userID string
	email  string
}

type phase1Device struct {
	dto.Device
	userID string
}

type phase1Node struct {
	dto.Node
	userID string
}

type phase1Network struct {
	dto.Network
	ownerUserID string
	joinKey     string
	dns         dto.DNSConfig
}

func newPhase1Services() *phase1Services {
	return &phase1Services{
		usersByID:     make(map[string]*phase1User),
		usersByEmail:  make(map[string]*phase1User),
		accessTokens:  make(map[string]string),
		refreshTokens: make(map[string]string),
		devices:       make(map[string]*phase1Device),
		nodes:         make(map[string]*phase1Node),
		networks:      make(map[string]*phase1Network),
		members:       make(map[string]map[string]*dto.NetworkMember),
		attachments:   make(map[string]map[string]*dto.SubnetAttachment),
		relayTickets:  make(map[string]dto.RelayTicket),
	}
}

func (s *phase1Services) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || strings.TrimSpace(req.Password) == "" {
		return dto.AuthResponse{}, service.ErrInvalidArgument
	}
	if _, exists := s.usersByEmail[email]; exists {
		return dto.AuthResponse{}, service.ErrConflict
	}
	userID := s.newID("user")
	s.usersByID[userID] = &phase1User{userID: userID, email: email}
	s.usersByEmail[email] = s.usersByID[userID]
	return s.issueAuthLocked(userID), nil
}

func (s *phase1Services) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.usersByEmail[strings.ToLower(strings.TrimSpace(req.Email))]
	if user == nil {
		return dto.AuthResponse{}, service.ErrUnauthorized
	}
	return s.issueAuthLocked(user.userID), nil
}

func (s *phase1Services) Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID := s.refreshTokens[req.RefreshToken]
	if userID == "" {
		return dto.AuthResponse{}, service.ErrUnauthorized
	}
	return s.issueAuthLocked(userID), nil
}

func (s *phase1Services) GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error) {
	return dto.AuthCallbackStatusResponse{CallbackID: callbackID}, nil
}

func (s *phase1Services) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	return nil
}

func (s *phase1Services) MarkCallbackReceived(callbackID string) error {
	return nil
}

func (s *phase1Services) Authenticate(accessToken string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID := s.accessTokens[accessToken]
	if userID == "" {
		return "", service.ErrUnauthorized
	}
	return userID, nil
}

func (s *phase1Services) registerDevice(userID string, req dto.RegisterDeviceRequest) (dto.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usersByID[userID] == nil || strings.TrimSpace(req.MachineID) == "" {
		return dto.Device{}, service.ErrInvalidArgument
	}
	deviceID := s.newID("dev")
	device := &phase1Device{
		Device: dto.Device{
			DeviceID:  deviceID,
			Name:      req.Name,
			Platform:  req.Platform,
			MachineID: req.MachineID,
			PublicKey: req.PublicKey,
			Status:    "offline",
		},
		userID: userID,
	}
	s.devices[deviceID] = device
	return device.Device, nil
}

func (s *phase1Services) ListByUser(userID string) ([]dto.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []dto.Device{}
	for _, device := range s.devices {
		if device.userID == userID {
			items = append(items, device.Device)
		}
	}
	return items, nil
}

func (s *phase1Services) registerNode(userID string, req dto.RegisterNodeRequest) (dto.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[req.DeviceID]
	if device == nil || device.userID != userID || strings.TrimSpace(req.NodeID) == "" {
		return dto.Node{}, service.ErrForbidden
	}
	node := &phase1Node{
		Node: dto.Node{
			NodeID:        req.NodeID,
			DeviceID:      req.DeviceID,
			NodePublicKey: req.NodePublicKey,
			Capabilities:  append([]string(nil), req.Capabilities...),
		},
		userID: userID,
	}
	s.nodes[req.NodeID] = node
	return node.Node, nil
}

func (s *phase1Services) Home(userID string) (dto.NetworkHome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	home := dto.NetworkHome{}
	for _, network := range s.networks {
		if network.ownerUserID == userID && home.OwnedNetwork == nil {
			n := network.Network
			home.OwnedNetwork = &n
		}
		if s.userHasNetworkLocked(userID, network.NetworkID) && home.ActiveNetwork == nil {
			n := network.Network
			home.ActiveNetwork = &n
		}
	}
	home.HasNetwork = home.ActiveNetwork != nil || home.OwnedNetwork != nil
	return home, nil
}

func (s *phase1Services) List(userID string) ([]dto.Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []dto.Network{}
	for _, network := range s.networks {
		if network.ownerUserID == userID || s.userHasNetworkLocked(userID, network.NetworkID) {
			items = append(items, network.Network)
		}
	}
	return items, nil
}

func (s *phase1Services) Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.CIDR) == "" {
		return dto.Network{}, service.ErrInvalidArgument
	}
	networkID := s.newID("net")
	subnetID := s.newID("subnet")
	network := &phase1Network{
		Network: dto.Network{
			NetworkID:         networkID,
			Name:              req.Name,
			Description:       req.Description,
			DefaultSubnetID:   subnetID,
			DefaultSubnetCIDR: req.CIDR,
		},
		ownerUserID: userID,
	}
	s.networks[networkID] = network
	if req.BindDeviceID != "" {
		if _, _, err := s.ensureJoinedLocked(userID, networkID, req.BindDeviceID, "owner"); err != nil {
			return dto.Network{}, err
		}
	}
	return network.Network, nil
}

func (s *phase1Services) Update(userID, networkID string, req dto.UpdateNetworkRequest) (dto.Network, error) {
	return dto.Network{}, service.ErrInvalidArgument
}

func (s *phase1Services) UpdateDNS(userID, networkID string, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	network, err := s.requireOwnedNetworkLocked(userID, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	network.dns = dto.DNSConfig{
		Servers:       append([]string(nil), req.Servers...),
		SearchDomains: append([]string(nil), req.SearchDomains...),
	}
	return s.networkDetailLocked(userID, networkID), nil
}

func (s *phase1Services) UpdateJoinKey(userID, networkID string, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	network, err := s.requireOwnedNetworkLocked(userID, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	network.joinKey = req.JoinKey
	return s.networkDetailLocked(userID, networkID), nil
}

func (s *phase1Services) Get(userID, networkID string) (dto.NetworkDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.requireVisibleNetworkLocked(userID, networkID); err != nil {
		return dto.NetworkDetail{}, err
	}
	return s.networkDetailLocked(userID, networkID), nil
}

func (s *phase1Services) Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, err := s.ensureJoinedLocked(userID, networkID, req.DeviceID, "member")
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return s.joinResultLocked(networkID, req.DeviceID), nil
}

func (s *phase1Services) JoinByOwnerEmail(userID string, req dto.JoinNetworkByOwnerEmailRequest) (dto.NetworkJoinByOwnerEmailResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := s.usersByEmail[strings.ToLower(strings.TrimSpace(req.OwnerEmail))]
	if owner == nil {
		return dto.NetworkJoinByOwnerEmailResult{}, service.ErrNotFound
	}
	for _, network := range s.networks {
		if network.ownerUserID != owner.userID {
			continue
		}
		if _, _, err := s.ensureJoinedLocked(userID, network.NetworkID, req.DeviceID, "member"); err != nil {
			return dto.NetworkJoinByOwnerEmailResult{}, err
		}
		result := s.joinResultLocked(network.NetworkID, req.DeviceID)
		return dto.NetworkJoinByOwnerEmailResult{
			Network:    network.Network,
			Member:     result.Member,
			Attachment: result.Attachment,
		}, nil
	}
	return dto.NetworkJoinByOwnerEmailResult{}, service.ErrNotFound
}

func (s *phase1Services) JoinByKey(userID string, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, network := range s.networks {
		if network.joinKey == req.JoinKey && network.joinKey != "" {
			if _, _, err := s.ensureJoinedLocked(userID, network.NetworkID, req.DeviceID, "member"); err != nil {
				return dto.NetworkJoinResult{}, err
			}
			return s.joinResultLocked(network.NetworkID, req.DeviceID), nil
		}
	}
	return dto.NetworkJoinResult{}, service.ErrNotFound
}

func (s *phase1Services) Switch(userID, networkID string, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error) {
	return s.Join(userID, networkID, dto.JoinNetworkRequest{DeviceID: req.DeviceID})
}

func (s *phase1Services) Activate(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
	return s.Join(userID, networkID, req)
}

func (s *phase1Services) Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error {
	return nil
}

func (s *phase1Services) ListMembers(userID, networkID string) ([]dto.NetworkMember, error) {
	return nil, nil
}

func (s *phase1Services) UpdateMemberStatus(userID, networkID, memberID string, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error) {
	return dto.NetworkMember{}, service.ErrInvalidArgument
}

func (s *phase1Services) ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.requireVisibleNetworkLocked(userID, networkID); err != nil {
		return nil, err
	}
	return s.assignmentsLocked(networkID), nil
}

func (s *phase1Services) CreateSubnet(userID, networkID string, req dto.CreateSubnetRequest) (dto.Subnet, error) {
	return dto.Subnet{}, service.ErrInvalidArgument
}

func (s *phase1Services) ListSubnets(userID, networkID string) ([]dto.Subnet, error) {
	return nil, nil
}

func (s *phase1Services) AttachDevice(userID, networkID, subnetID string, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error) {
	return dto.SubnetAttachment{}, service.ErrInvalidArgument
}

func (s *phase1Services) UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
	return dto.SubnetAttachment{}, service.ErrInvalidArgument
}

func (s *phase1Services) UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	network, err := s.requireVisibleNetworkLocked(userID, networkID)
	if err != nil {
		return dto.NetworkAssignment{}, err
	}
	attachment, deviceID := s.attachmentByIDLocked(networkID, attachmentID)
	if attachment == nil {
		return dto.NetworkAssignment{}, service.ErrNotFound
	}
	if network.ownerUserID != userID {
		device := s.devices[deviceID]
		if device == nil || device.userID != userID {
			return dto.NetworkAssignment{}, service.ErrForbidden
		}
	}
	attachment.Remark = strings.TrimSpace(req.Remark)
	for _, item := range s.assignmentsLocked(networkID) {
		if item.AttachmentID == attachmentID {
			return item, nil
		}
	}
	return dto.NetworkAssignment{}, service.ErrNotFound
}

func (s *phase1Services) CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
	bootstrap, err := s.Bootstrap(userID, dto.BootstrapRequest{NodeID: req.NodeID, NetworkID: req.NetworkID})
	if err != nil {
		return dto.ControlSessionResponse{}, err
	}
	return dto.ControlSessionResponse{
		ControlSessionID: bootstrap.ControlSessionID,
		SessionToken:     bootstrap.SessionToken,
		ControlPlane:     bootstrap.ControlPlane,
		NetworkMap:       bootstrap.NetworkMap,
	}, nil
}

func (s *phase1Services) Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, network, attachment, err := s.requireRuntimeLocked(userID, req.NodeID, req.NetworkID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	ctrlID := s.newID("ctrl")
	token := s.newID("control-token")
	return dto.BootstrapResponse{
		ControlSessionID: ctrlID,
		SessionToken:     token,
		Device: dto.DeviceBootstrap{
			Device:      s.devices[node.DeviceID].Device,
			Attachments: []dto.SubnetAttachment{*attachment},
		},
		Networks: []dto.NetworkDetail{s.networkDetailLocked(userID, network.NetworkID)},
		ControlPlane: dto.ControlPlaneConfig{
			WSURL:            "/control/ws",
			HeartbeatSeconds: 30,
		},
		STUNServers: []string{"stun:example.org:3478"},
		Relay: dto.RelayConfig{
			DefaultClusterID: "cluster-a",
		},
		DerpMap: dto.DerpMap{
			ProbeIntervalSeconds: 30,
			Clusters: []dto.DerpCluster{{
				ClusterID: "cluster-a",
				Nodes: []dto.DerpNode{
					{NodeID: "derp-a", Host: "relay-a.example.org", Port: 443, Transport: "tcp"},
					{NodeID: "derp-b", Host: "relay-b.example.org", Port: 443, Transport: "tcp"},
				},
			}},
		},
		NetworkMap: s.networkMapLocked(userID, node, network.NetworkID),
	}, nil
}

func (s *phase1Services) IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(req.NetworkID) == "" || strings.TrimSpace(req.SrcNodeID) == "" || strings.TrimSpace(req.DstNodeID) == "" || strings.TrimSpace(req.Reason) == "" {
		return dto.RelayTicket{}, service.ErrInvalidArgument
	}
	src, _, _, err := s.requireRuntimeLocked(userID, req.SrcNodeID, req.NetworkID)
	if err != nil {
		return dto.RelayTicket{}, err
	}
	dst := s.nodes[req.DstNodeID]
	if dst == nil {
		return dto.RelayTicket{}, service.ErrNotFound
	}
	if _, _, err := s.requireAttachedDeviceLocked(req.NetworkID, dst.DeviceID); err != nil {
		return dto.RelayTicket{}, service.ErrNotFound
	}
	cacheKey := req.NetworkID + ":" + src.NodeID + ":" + dst.NodeID + ":" + strings.Join(normalizeDerpNodeIDs(req.PreferredDerpNodeIDs), ",")
	if ticket, ok := s.relayTickets[cacheKey]; ok {
		return ticket, nil
	}
	ticket := dto.RelayTicket{
		TicketID:           s.newID("ticket"),
		NetworkID:          req.NetworkID,
		SessionID:          s.newID("relay-session"),
		SrcNodeID:          src.NodeID,
		DstNodeID:          dst.NodeID,
		DerpClusterID:      "cluster-a",
		AllowedDerpNodeIDs: normalizeDerpNodeIDs(req.PreferredDerpNodeIDs),
		RelayURL:           "tcp://relay-a.example.org:443",
		ExpiresAt:          "2026-04-25T12:00:00Z",
		SessionKey:         s.newID("session-key"),
		Signature:          "signed",
	}
	s.relayTickets[cacheKey] = ticket
	return ticket, nil
}

func (s *phase1Services) Handshake(hello controlws.NodeHello) (controlws.NodeHelloAck, dto.NetworkMap, error) {
	return controlws.NodeHelloAck{}, dto.NetworkMap{}, service.ErrInvalidArgument
}

func (s *phase1Services) NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, _, _, err := s.requireRuntimeLocked(userID, nodeID, networkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}
	return s.networkMapLocked(userID, node, networkID), nil
}

func (s *phase1Services) ReportEndpoints(userID string, report controlws.EndpointReport) (dto.NetworkMap, error) {
	return dto.NetworkMap{}, service.ErrInvalidArgument
}

func (s *phase1Services) ReportConnectionState(userID, nodeID string, state controlws.ConnectionState) error {
	return nil
}

func (s *phase1Services) ReportPathHealth(userID, nodeID string, report controlws.PathHealthReport) error {
	return nil
}

func (s *phase1Services) Disconnect(userID, nodeID string, notice controlws.DisconnectNotice) error {
	return nil
}

func (s *phase1Services) Heartbeat(userID, nodeID, networkID string) error {
	return nil
}

func (s *phase1Services) CloseSession(userID, nodeID, networkID string) error {
	return nil
}

func (s *phase1Services) PeerSnapshot(userID, nodeID, networkID, peerNodeID string) (dto.Peer, error) {
	return dto.Peer{}, service.ErrInvalidArgument
}

func (s *phase1Services) ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	return controlws.ConnectPlan{}, service.ErrInvalidArgument
}

func (s *phase1Services) ConnectPlanByNode(nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	return controlws.ConnectPlan{}, service.ErrInvalidArgument
}

func (s *phase1Services) issueAuthLocked(userID string) dto.AuthResponse {
	accessToken := s.newID("access")
	refreshToken := s.newID("refresh")
	s.accessTokens[accessToken] = userID
	s.refreshTokens[refreshToken] = userID
	return dto.AuthResponse{
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}
}

func (s *phase1Services) ensureJoinedLocked(userID, networkID, deviceID, role string) (*dto.NetworkMember, *dto.SubnetAttachment, error) {
	device := s.devices[deviceID]
	if device == nil || device.userID != userID {
		return nil, nil, service.ErrForbidden
	}
	if _, err := s.requireVisibleNetworkLocked(userID, networkID); err != nil && role != "owner" {
		if s.networks[networkID] == nil {
			return nil, nil, err
		}
	}
	if role == "owner" {
		if _, err := s.requireOwnedNetworkLocked(userID, networkID); err != nil {
			return nil, nil, err
		}
	}
	if s.members[networkID] == nil {
		s.members[networkID] = make(map[string]*dto.NetworkMember)
	}
	member := s.members[networkID][deviceID]
	if member == nil {
		member = &dto.NetworkMember{
			MemberID:  s.newID("member"),
			NetworkID: networkID,
			DeviceID:  deviceID,
			Role:      role,
			Status:    "active",
		}
		s.members[networkID][deviceID] = member
	}
	if s.attachments[networkID] == nil {
		s.attachments[networkID] = make(map[string]*dto.SubnetAttachment)
	}
	attachment := s.attachments[networkID][deviceID]
	if attachment == nil {
		attachment = &dto.SubnetAttachment{
			AttachmentID: s.newID("att"),
			NetworkID:    networkID,
			SubnetID:     s.networks[networkID].DefaultSubnetID,
			DeviceID:     deviceID,
			VirtualIP:    "100.64.0." + s.nextOctetLocked(),
			Status:       "active",
		}
		s.attachments[networkID][deviceID] = attachment
	}
	return member, attachment, nil
}

func (s *phase1Services) joinResultLocked(networkID, deviceID string) dto.NetworkJoinResult {
	return dto.NetworkJoinResult{
		Member:     *s.members[networkID][deviceID],
		Attachment: *s.attachments[networkID][deviceID],
	}
}

func (s *phase1Services) requireRuntimeLocked(userID, nodeID, networkID string) (*phase1Node, *phase1Network, *dto.SubnetAttachment, error) {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return nil, nil, nil, service.ErrInvalidArgument
	}
	node := s.nodes[nodeID]
	if node == nil {
		return nil, nil, nil, service.ErrNotFound
	}
	if node.userID != userID {
		return nil, nil, nil, service.ErrForbidden
	}
	network, err := s.requireVisibleNetworkLocked(userID, networkID)
	if err != nil {
		return nil, nil, nil, err
	}
	_, attachment, err := s.requireAttachedDeviceLocked(networkID, node.DeviceID)
	if err != nil {
		return nil, nil, nil, service.ErrForbidden
	}
	return node, network, attachment, nil
}

func (s *phase1Services) requireAttachedDeviceLocked(networkID, deviceID string) (*dto.NetworkMember, *dto.SubnetAttachment, error) {
	member := s.members[networkID][deviceID]
	if member == nil || member.Status != "active" {
		return nil, nil, service.ErrForbidden
	}
	attachment := s.attachments[networkID][deviceID]
	if attachment == nil || attachment.Status != "active" || attachment.VirtualIP == "" {
		return nil, nil, service.ErrForbidden
	}
	return member, attachment, nil
}

func (s *phase1Services) requireOwnedNetworkLocked(userID, networkID string) (*phase1Network, error) {
	network := s.networks[networkID]
	if network == nil {
		return nil, service.ErrNotFound
	}
	if network.ownerUserID != userID {
		return nil, service.ErrForbidden
	}
	return network, nil
}

func (s *phase1Services) requireVisibleNetworkLocked(userID, networkID string) (*phase1Network, error) {
	network := s.networks[networkID]
	if network == nil {
		return nil, service.ErrNotFound
	}
	if network.ownerUserID == userID || s.userHasNetworkLocked(userID, networkID) {
		return network, nil
	}
	return nil, service.ErrForbidden
}

func (s *phase1Services) userHasNetworkLocked(userID, networkID string) bool {
	for deviceID, member := range s.members[networkID] {
		device := s.devices[deviceID]
		if device != nil && device.userID == userID && member.Status == "active" {
			return true
		}
	}
	return false
}

func (s *phase1Services) attachmentByIDLocked(networkID, attachmentID string) (*dto.SubnetAttachment, string) {
	for deviceID, attachment := range s.attachments[networkID] {
		if attachment.AttachmentID == attachmentID {
			return attachment, deviceID
		}
	}
	return nil, ""
}

func (s *phase1Services) assignmentsLocked(networkID string) []dto.NetworkAssignment {
	assignments := []dto.NetworkAssignment{}
	for deviceID, attachment := range s.attachments[networkID] {
		device := s.devices[deviceID]
		member := s.members[networkID][deviceID]
		if device == nil || member == nil {
			continue
		}
		user := s.usersByID[device.userID]
		userEmail := ""
		if user != nil {
			userEmail = user.email
		}
		assignments = append(assignments, dto.NetworkAssignment{
			AttachmentID: attachment.AttachmentID,
			NetworkID:    attachment.NetworkID,
			SubnetID:     attachment.SubnetID,
			DeviceID:     attachment.DeviceID,
			DeviceName:   device.Name,
			UserID:       device.userID,
			UserEmail:    userEmail,
			Role:         member.Role,
			Remark:       attachment.Remark,
			VirtualIP:    attachment.VirtualIP,
			Status:       attachment.Status,
		})
	}
	return assignments
}

func (s *phase1Services) networkDetailLocked(userID, networkID string) dto.NetworkDetail {
	network := s.networks[networkID]
	detail := dto.NetworkDetail{
		Network:            network.Network,
		OwnedByCurrentUser: network.ownerUserID == userID,
		JoinKey:            network.joinKey,
		DNS:                network.dns,
		Members:            []dto.NetworkMember{},
		Subnets: []dto.Subnet{{
			SubnetID:  network.DefaultSubnetID,
			NetworkID: networkID,
			Name:      "default",
			CIDR:      network.DefaultSubnetCIDR,
			IsDefault: true,
			Status:    "active",
		}},
	}
	for _, member := range s.members[networkID] {
		detail.Members = append(detail.Members, *member)
	}
	if !detail.OwnedByCurrentUser {
		detail.JoinKey = ""
	}
	return detail
}

func (s *phase1Services) networkMapLocked(userID string, node *phase1Node, networkID string) dto.NetworkMap {
	peers := []dto.Peer{}
	for _, other := range s.nodes {
		if other.NodeID == node.NodeID {
			continue
		}
		member, attachment, err := s.requireAttachedDeviceLocked(networkID, other.DeviceID)
		if err != nil || member == nil || attachment == nil {
			continue
		}
		peers = append(peers, dto.Peer{
			NodeID:       other.NodeID,
			DeviceID:     other.DeviceID,
			PublicKey:    other.NodePublicKey,
			Status:       "online",
			RelayAllowed: true,
			VirtualIPs:   []string{attachment.VirtualIP},
		})
	}
	return dto.NetworkMap{
		SelfUserID:       userID,
		SelfDeviceID:     node.DeviceID,
		SelfNodeID:       node.NodeID,
		NetworkID:        networkID,
		Revision:         1,
		HeartbeatSeconds: 30,
		STUNServers:      []string{"stun:example.org:3478"},
		Peers:            peers,
		RelayRegions: []dto.RelayRegion{{
			RegionID:   "cluster-a",
			RegionName: "Default",
			ClusterID:  "cluster-a",
			Endpoints: []dto.RelayEndpoint{{
				EndpointID: "derp-a",
				Transport:  "tcp",
				Address:    "relay-a.example.org:443",
			}},
		}},
	}
}

func (s *phase1Services) newID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%d", prefix, s.nextID)
}

func (s *phase1Services) nextOctetLocked() string {
	s.nextID++
	return fmt.Sprintf("%d", s.nextID)
}

func normalizeDerpNodeIDs(values []string) []string {
	allowed := map[string]struct{}{"derp-a": {}, "derp-b": {}}
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{"derp-a", "derp-b"}
	}
	if len(out) == 2 && out[0] > out[1] {
		out[0], out[1] = out[1], out[0]
	}
	return out
}
