package serverbiztest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	httpapi "github.com/slan/server/server-biz/api/http"
	"github.com/slan/server/server-biz/internal/infra"
	"github.com/slan/server/server-biz/internal/service"
)

// TestPhase1Flow 覆盖当前关键 HTTP 流程：
// 注册、设备注册、节点注册、建网、加入网络、bootstrap 和 relay 票据签发。
func TestPhase1Flow(t *testing.T) {
	router := httpapi.NewRouter(service.NewInMemoryServices(infra.DefaultConfig()))

	registerResp := performJSON[dto.AuthResponse](t, router, http.MethodPost, "/auth/register", dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "password123",
	}, "")

	device := performJSON[dto.Device](t, router, http.MethodPost, "/devices/register", dto.RegisterDeviceRequest{
		Name:      "thor-mac",
		Platform:  "macos",
		MachineID: "machine-1",
		PublicKey: "pubkey-1",
	}, registerResp.AccessToken)

	peerDevice := performJSON[dto.Device](t, router, http.MethodPost, "/devices/register", dto.RegisterDeviceRequest{
		Name:      "thor-linux",
		Platform:  "linux",
		MachineID: "machine-2",
		PublicKey: "pubkey-2",
	}, registerResp.AccessToken)

	network := performJSON[dto.Network](t, router, http.MethodPost, "/networks", dto.CreateNetworkRequest{
		Name: "home",
		CIDR: "10.10.0.0/24",
	}, registerResp.AccessToken)

	join := performJSON[dto.NetworkJoinResult](t, router, http.MethodPost, "/networks/"+network.NetworkID+"/join", dto.JoinNetworkRequest{
		DeviceID: device.DeviceID,
	}, registerResp.AccessToken)

	if join.Attachment.VirtualIP == "" {
		t.Fatal("expected allocated virtual IP")
	}

	performJSON[dto.NetworkJoinResult](t, router, http.MethodPost, "/networks/"+network.NetworkID+"/join", dto.JoinNetworkRequest{
		DeviceID: peerDevice.DeviceID,
	}, registerResp.AccessToken)

	node := performJSON[dto.Node](t, router, http.MethodPost, "/nodes/register", dto.RegisterNodeRequest{
		DeviceID:      device.DeviceID,
		NodeID:        "node-1",
		NodePublicKey: "node-pubkey-1",
	}, registerResp.AccessToken)

	peerNode := performJSON[dto.Node](t, router, http.MethodPost, "/nodes/register", dto.RegisterNodeRequest{
		DeviceID:      peerDevice.DeviceID,
		NodeID:        "node-2",
		NodePublicKey: "node-pubkey-2",
	}, registerResp.AccessToken)

	detail := performJSON[dto.NetworkDetail](t, router, http.MethodGet, "/networks/"+network.NetworkID, nil, registerResp.AccessToken)
	if len(detail.Subnets) != 1 || len(detail.Members) != 2 {
		t.Fatalf("unexpected network detail: %+v", detail)
	}

	bootstrap := performJSON[dto.BootstrapResponse](t, router, http.MethodPost, "/bootstrap", dto.BootstrapRequest{
		NodeID:    node.NodeID,
		NetworkID: network.NetworkID,
	}, registerResp.AccessToken)
	if bootstrap.ControlSessionID == "" || bootstrap.ControlPlane.WSURL == "" {
		t.Fatalf("unexpected bootstrap control session payload: %+v", bootstrap)
	}
	if len(bootstrap.Device.Attachments) != 1 {
		t.Fatalf("unexpected attachments: %+v", bootstrap.Device.Attachments)
	}
	if bootstrap.NetworkMap.NetworkID != network.NetworkID || len(bootstrap.NetworkMap.Peers) != 1 {
		t.Fatalf("unexpected network map: %+v", bootstrap.NetworkMap)
	}
	if len(bootstrap.DerpMap.Clusters) != 1 {
		t.Fatalf("unexpected derp map: %+v", bootstrap.DerpMap)
	}
	if bootstrap.Relay.DefaultClusterID != "cn-local-a" {
		t.Fatalf("unexpected relay config: %+v", bootstrap.Relay)
	}
	if len(bootstrap.Relay.Countries) != 1 || len(bootstrap.Relay.Countries[0].Cities) != 1 {
		t.Fatalf("unexpected relay topology: %+v", bootstrap.Relay)
	}

	controlSession := performJSON[dto.ControlSessionResponse](t, router, http.MethodPost, "/control/sessions", dto.CreateControlSessionRequest{
		NodeID:    node.NodeID,
		NetworkID: network.NetworkID,
	}, registerResp.AccessToken)
	if controlSession.ControlSessionID == "" || controlSession.SessionToken == "" {
		t.Fatalf("unexpected control session: %+v", controlSession)
	}
	if controlSession.NetworkMap.SelfNodeID != node.NodeID {
		t.Fatalf("unexpected control session network map: %+v", controlSession.NetworkMap)
	}

	relay := performJSON[dto.RelayTicket](t, router, http.MethodPost, "/relay/tickets", dto.RelayTicketRequest{
		NetworkID:            network.NetworkID,
		SrcNodeID:            node.NodeID,
		DstNodeID:            peerNode.NodeID,
		DerpClusterID:        "cn-local-a",
		PreferredDerpNodeIDs: []string{"relay-cn-local-udp", "relay-cn-local-tcp"},
		Reason:               "timeout",
	}, registerResp.AccessToken)
	if relay.RelayURL == "" || relay.Signature == "" || relay.SessionID == "" || relay.DerpClusterID != "cn-local-a" {
		t.Fatalf("unexpected relay ticket: %+v", relay)
	}
}

// performJSON 用统一方式执行一次 JSON HTTP 请求并解码响应。
func performJSON[T any](t *testing.T, router http.Handler, method, path string, body any, token string) T {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s failed: status=%d body=%s", method, path, rec.Code, rec.Body.String())
	}

	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return out
}
