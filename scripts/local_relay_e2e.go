package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type authResponse struct {
	UserID      string `json:"userId"`
	AccessToken string `json:"accessToken"`
}

type deviceResponse struct {
	DeviceID string `json:"deviceId"`
}

type networkResponse struct {
	NetworkID string `json:"networkId"`
}

type nodeResponse struct {
	NodeID string `json:"nodeId"`
}

type relayTicket struct {
	TicketID           string   `json:"ticketId"`
	NetworkID          string   `json:"networkId"`
	SessionID          string   `json:"sessionId"`
	SrcNodeID          string   `json:"srcNodeId"`
	DstNodeID          string   `json:"dstNodeId"`
	DerpClusterID      string   `json:"derpClusterId,omitempty"`
	CountryCode        string   `json:"countryCode,omitempty"`
	CityCode           string   `json:"cityCode,omitempty"`
	AllowedDerpNodeIDs []string `json:"allowedDerpNodeIds,omitempty"`
	RelayURL           string   `json:"relayUrl"`
	ExpiresAt          string   `json:"expiresAt"`
	SessionKey         string   `json:"sessionKey,omitempty"`
	Signature          string   `json:"signature"`
}

type relayTicketWire struct {
	TicketID           string   `json:"ticket_id"`
	NetworkID          string   `json:"network_id"`
	SessionID          string   `json:"session_id"`
	SrcNodeID          string   `json:"src_node_id"`
	DstNodeID          string   `json:"dst_node_id"`
	DerpClusterID      string   `json:"derp_cluster_id,omitempty"`
	CountryCode        string   `json:"country_code,omitempty"`
	CityCode           string   `json:"city_code,omitempty"`
	AllowedDerpNodeIDs []string `json:"allowed_derp_node_ids,omitempty"`
	RelayURL           string   `json:"relay_url"`
	ExpiresAt          string   `json:"expires_at"`
	SessionKey         string   `json:"session_key,omitempty"`
	Signature          string   `json:"signature"`
}

type clientRequest struct {
	Kind              string          `json:"kind"`
	ParticipantID     string          `json:"participant_id,omitempty"`
	Ticket            *relayTicketWire `json:"ticket,omitempty"`
	SessionID         string          `json:"session_id,omitempty"`
	FromParticipantID string          `json:"from_participant_id,omitempty"`
	PayloadB64        string          `json:"payload_b64,omitempty"`
}

type serverResponse struct {
	Kind              string `json:"kind"`
	SessionID         string `json:"session_id,omitempty"`
	PeerParticipantID string `json:"peer_participant_id,omitempty"`
	ToParticipantID   string `json:"to_participant_id,omitempty"`
	BytesForwarded    int    `json:"bytes_forwarded,omitempty"`
	FromParticipantID string `json:"from_participant_id,omitempty"`
	PayloadB64        string `json:"payload_b64,omitempty"`
	Code              string `json:"code,omitempty"`
	Message           string `json:"message,omitempty"`
}

func main() {
	baseURL := getenv("SLAN_SMOKE_BIZ_URL", "http://127.0.0.1:28080")
	relayAddr := getenv("SLAN_SMOKE_RELAY_ADDR", "127.0.0.1:19000")
	stamp := time.Now().UnixNano()

	email := fmt.Sprintf("relay-e2e-%d@local.slan", stamp)
	password := "relay-e2e-password-2026"
	machineA := fmt.Sprintf("relay-machine-a-%d", stamp)
	machineB := fmt.Sprintf("relay-machine-b-%d", stamp)
	nodeA := fmt.Sprintf("relay-node-a-%d", stamp)
	nodeB := fmt.Sprintf("relay-node-b-%d", stamp)

	auth := mustPOST[authResponse](baseURL+"/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
	})

	deviceA := mustPOST[deviceResponse](baseURL+"/devices/register", auth.AccessToken, map[string]any{
		"name":      "Relay E2E A",
		"platform":  "macos",
		"machineId": machineA,
		"publicKey": "relay-e2e-device-pub-a",
	})
	deviceB := mustPOST[deviceResponse](baseURL+"/devices/register", auth.AccessToken, map[string]any{
		"name":      "Relay E2E B",
		"platform":  "macos",
		"machineId": machineB,
		"publicKey": "relay-e2e-device-pub-b",
	})

	network := mustPOST[networkResponse](baseURL+"/networks", auth.AccessToken, map[string]any{
		"name":        "Relay E2E Net",
		"description": "relay data plane smoke",
		"cidr":        "100.97.0.0/24",
	})

	nodeRespA := mustPOST[nodeResponse](baseURL+"/nodes/register", auth.AccessToken, map[string]any{
		"deviceId":      deviceA.DeviceID,
		"nodeId":        nodeA,
		"nodePublicKey": "relay-node-pub-a",
		"capabilities":  []string{"desktop"},
	})
	nodeRespB := mustPOST[nodeResponse](baseURL+"/nodes/register", auth.AccessToken, map[string]any{
		"deviceId":      deviceB.DeviceID,
		"nodeId":        nodeB,
		"nodePublicKey": "relay-node-pub-b",
		"capabilities":  []string{"desktop"},
	})

	mustPOST[map[string]any](baseURL+"/networks/"+network.NetworkID+"/join", auth.AccessToken, map[string]any{
		"deviceId": deviceA.DeviceID,
	})
	mustPOST[map[string]any](baseURL+"/networks/"+network.NetworkID+"/join", auth.AccessToken, map[string]any{
		"deviceId": deviceB.DeviceID,
	})

	ticket := mustPOST[relayTicket](baseURL+"/relay/tickets", auth.AccessToken, map[string]any{
		"networkId": network.NetworkID,
		"srcNodeId": nodeRespA.NodeID,
		"dstNodeId": nodeRespB.NodeID,
		"reason":    "relay-data-e2e",
	})

	wire := relayTicketWire{
		TicketID:           ticket.TicketID,
		NetworkID:          ticket.NetworkID,
		SessionID:          ticket.SessionID,
		SrcNodeID:          ticket.SrcNodeID,
		DstNodeID:          ticket.DstNodeID,
		DerpClusterID:      ticket.DerpClusterID,
		CountryCode:        ticket.CountryCode,
		CityCode:           ticket.CityCode,
		AllowedDerpNodeIDs: ticket.AllowedDerpNodeIDs,
		RelayURL:           ticket.RelayURL,
		ExpiresAt:          ticket.ExpiresAt,
		SessionKey:         ticket.SessionKey,
		Signature:          ticket.Signature,
	}

	connA := mustDialUDP(relayAddr)
	defer connA.Close()
	connB := mustDialUDP(relayAddr)
	defer connB.Close()

	mustSendJSON(connA, clientRequest{
		Kind:          "attach",
		ParticipantID: nodeRespA.NodeID,
		Ticket:        &wire,
	})
	attachA := mustReadJSON(connA)
	expect(attachA.Kind == "attached", "attach A failed: %+v", attachA)

	mustSendJSON(connB, clientRequest{
		Kind:          "attach",
		ParticipantID: nodeRespB.NodeID,
		Ticket:        &wire,
	})
	attachB := mustReadJSON(connB)
	expect(attachB.Kind == "attached", "attach B failed: %+v", attachB)

	payload := []byte("relay-e2e-ping")
	mustSendJSON(connA, clientRequest{
		Kind:              "forward",
		SessionID:         ticket.SessionID,
		FromParticipantID: nodeRespA.NodeID,
		PayloadB64:        base64.StdEncoding.EncodeToString(payload),
	})
	ack := mustReadJSON(connA)
	expect(ack.Kind == "forwarded", "forward ack failed: %+v", ack)
	packet := mustReadJSON(connB)
	expect(packet.Kind == "packet", "peer packet failed: %+v", packet)
	decoded, err := base64.StdEncoding.DecodeString(packet.PayloadB64)
	if err != nil {
		fatal("decode packet payload: %v", err)
	}
	expect(bytes.Equal(decoded, payload), "unexpected payload: %q", string(decoded))

	mustSendJSON(connA, clientRequest{
		Kind:          "detach",
		SessionID:     ticket.SessionID,
		ParticipantID: nodeRespA.NodeID,
	})
	detachA := mustReadJSON(connA)
	expect(detachA.Kind == "detached", "detach A failed: %+v", detachA)

	mustSendJSON(connB, clientRequest{
		Kind:          "detach",
		SessionID:     ticket.SessionID,
		ParticipantID: nodeRespB.NodeID,
	})
	detachB := mustReadJSON(connB)
	expect(detachB.Kind == "detached", "detach B failed: %+v", detachB)

	fmt.Printf("USER_ID=%s\n", auth.UserID)
	fmt.Printf("NETWORK_ID=%s\n", network.NetworkID)
	fmt.Printf("NODE_A=%s\n", nodeRespA.NodeID)
	fmt.Printf("NODE_B=%s\n", nodeRespB.NodeID)
	fmt.Printf("SESSION_ID=%s\n", ticket.SessionID)
	fmt.Printf("RELAY_URL=%s\n", ticket.RelayURL)
	fmt.Printf("ATTACH_A=%+v\n", attachA)
	fmt.Printf("ATTACH_B=%+v\n", attachB)
	fmt.Printf("FORWARD_ACK=%+v\n", ack)
	fmt.Printf("PACKET=%+v\n", packet)
	fmt.Printf("DETACH_A=%+v\n", detachA)
	fmt.Printf("DETACH_B=%+v\n", detachB)
}

func mustPOST[T any](url, token string, body map[string]any) T {
	var zero T
	buf, err := json.Marshal(body)
	if err != nil {
		fatal("marshal %s: %v", url, err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		fatal("new request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatal("post %s returned %d: %s", url, resp.StatusCode, string(data))
	}
	if err := json.Unmarshal(data, &zero); err != nil {
		fatal("decode %s: %v body=%s", url, err, string(data))
	}
	return zero
}

func mustDialUDP(addr string) *net.UDPConn {
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		fatal("resolve relay addr: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		fatal("dial relay: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		fatal("set deadline: %v", err)
	}
	return conn
}

func mustSendJSON(conn *net.UDPConn, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fatal("marshal udp request: %v", err)
	}
	if _, err := conn.Write(data); err != nil {
		fatal("udp write: %v", err)
	}
}

func mustReadJSON(conn *net.UDPConn) serverResponse {
	var resp serverResponse
	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil {
		fatal("udp read: %v", err)
	}
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		fatal("decode udp response: %v body=%s", err, string(buf[:n]))
	}
	return resp
}

func expect(ok bool, format string, args ...any) {
	if !ok {
		fatal(format, args...)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
