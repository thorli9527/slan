package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type relayTicket struct {
	TicketID  string    `json:"ticketId"`
	PeerID    string    `json:"peerId"`
	SessionID string    `json:"sessionId"`
	Path      string    `json:"path"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

type derpTicket struct {
	TicketID  string    `json:"ticketId"`
	PeerID    string    `json:"peerId"`
	NetworkID string    `json:"networkId"`
	Path      string    `json:"path"`
	RegionID  string    `json:"regionId"`
	NodeID    string    `json:"nodeId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	root, err := os.Getwd()
	must(err)

	wirePort := freeTCPPort()
	relayUDPPort := freeUDPPort()
	relayAdminPort := freeTCPPort()
	derpPort := freeTCPPort()
	derpAdminPort := freeTCPPort()

	procs := []*exec.Cmd{
		start(ctx, root, "server/server-wire", map[string]string{
			"SLAN_WIRE_LISTEN_ADDR": fmt.Sprintf("127.0.0.1:%d", wirePort),
		}),
		start(ctx, root, "server/server-wire-relay", map[string]string{
			"SLAN_WIRE_RELAY_LISTEN_ADDR":       fmt.Sprintf("127.0.0.1:%d", relayUDPPort),
			"SLAN_WIRE_RELAY_ADMIN_LISTEN_ADDR": fmt.Sprintf("127.0.0.1:%d", relayAdminPort),
		}),
		start(ctx, root, "server/server-wire-derp", map[string]string{
			"SLAN_WIRE_DERP_LISTEN_ADDR":       fmt.Sprintf("127.0.0.1:%d", derpPort),
			"SLAN_WIRE_DERP_ADMIN_LISTEN_ADDR": fmt.Sprintf("127.0.0.1:%d", derpAdminPort),
		}),
	}
	defer func() {
		for _, proc := range procs {
			if proc.Process != nil {
				_ = proc.Process.Kill()
			}
		}
	}()

	wireURL := fmt.Sprintf("http://127.0.0.1:%d", wirePort)
	waitHTTP(ctx, wireURL+"/healthz")
	waitHTTP(ctx, fmt.Sprintf("http://127.0.0.1:%d/healthz", relayAdminPort))
	waitHTTP(ctx, fmt.Sprintf("http://127.0.0.1:%d/healthz", derpAdminPort))

	postJSON(wireURL+"/v1/peers/register", map[string]any{
		"peer": map[string]any{
			"peerId":                  "peer-a",
			"networkId":               "net-a",
			"nodeId":                  "node-a",
			"publicKey":               "test-public-key",
			"supportsLanDirect":       true,
			"supportsIpv6Direct":      true,
			"supportsDirectUdp":       true,
			"supportsRelayUdp":        true,
			"supportsDerpTcpTls443":   true,
			"allowEndpointRoaming":    true,
			"allowFastReselection":    true,
			"allowRelayTicketRenewal": true,
		},
	}, nil)

	postJSON(wireURL+"/v1/peers/path-health", map[string]any{
		"peerId": "peer-a",
		"probes": []map[string]any{
			{"path": "relay_udp", "reachable": true, "rttMs": 20, "mtu": 1280},
			{"path": "derp_tcp_tls_443", "reachable": true, "rttMs": 70, "mtu": 1240},
		},
	}, nil)
	postJSON(wireURL+"/v1/peers/derp-health", map[string]any{
		"peerId": "peer-a",
		"samples": []map[string]any{
			{"regionId": "cn-east", "nodeId": "derp-cn-east-1", "reachable": true, "rttMs": 60},
		},
	}, nil)

	var relayResp struct {
		Ticket relayTicket `json:"ticket"`
	}
	postJSON(wireURL+"/v1/relay/tickets", map[string]any{"peerId": "peer-a", "ttlSeconds": 300}, &relayResp)
	if relayResp.Ticket.SessionID == "" || relayResp.Ticket.Path != "relay_udp" {
		fail("invalid relay ticket: %+v", relayResp.Ticket)
	}

	var derpResp struct {
		Ticket derpTicket `json:"ticket"`
	}
	postJSON(wireURL+"/v1/derp/tickets", map[string]any{"peerId": "peer-a", "ttlSeconds": 300}, &derpResp)
	if derpResp.Ticket.NodeID == "" || derpResp.Ticket.RegionID == "" || derpResp.Ticket.Path != "derp_tcp_tls_443" {
		fail("invalid derp ticket: %+v", derpResp.Ticket)
	}

	smokeRelay(relayUDPPort, relayResp.Ticket)
	smokeDerp(derpPort, derpResp.Ticket)

	fmt.Println("wire stack smoke passed")
}

func start(ctx context.Context, root, rel string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/"+filepath.Base(rel))
	cmd.Dir = filepath.Join(root, rel)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GO111MODULE=on")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Start())
	return cmd
}

func waitHTTP(ctx context.Context, url string) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	fail("timeout waiting for %s", url)
}

func postJSON(url string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(payload))
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("POST %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func smokeRelay(port int, ticket relayTicket) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	must(err)
	a := udpConn()
	defer a.Close()
	b := udpConn()
	defer b.Close()

	udpSend(a, addr, map[string]any{"kind": "attach", "participantId": "node-a", "transport": "relay_udp", "ticket": ticket})
	udpRecv(a)
	udpSend(b, addr, map[string]any{"kind": "attach", "participantId": "node-b", "transport": "relay_udp", "ticket": ticket})
	udpRecv(b)

	udpSend(a, addr, map[string]any{"kind": "forward", "sessionId": ticket.SessionID, "participantId": "node-a", "payload": []byte("relay-payload")})
	packet := udpRecv(b)
	if packet["kind"] != "packet" {
		fail("expected relay packet, got %+v", packet)
	}
	forwarded := udpRecv(a)
	if forwarded["kind"] != "forwarded" {
		fail("expected relay forwarded, got %+v", forwarded)
	}
}

func smokeDerp(port int, ticket derpTicket) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	must(err)
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	must(enc.Encode(map[string]any{
		"kind":     "connect",
		"peerId":   ticket.PeerID,
		"nodeId":   "node-a",
		"regionId": ticket.RegionID,
		"ticket":   ticket,
	}))
	var connected map[string]any
	must(dec.Decode(&connected))
	if connected["kind"] != "connected" {
		fail("expected derp connected, got %+v", connected)
	}
	sessionID, _ := connected["sessionId"].(string)
	if sessionID == "" {
		fail("missing derp session id: %+v", connected)
	}
	must(enc.Encode(map[string]any{"kind": "disconnect", "sessionId": sessionID}))
	var disconnected map[string]any
	must(dec.Decode(&disconnected))
	if disconnected["kind"] != "disconnected" {
		fail("expected derp disconnected, got %+v", disconnected)
	}
}

func udpConn() *net.UDPConn {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	must(err)
	return conn
}

func udpSend(conn *net.UDPConn, addr *net.UDPAddr, msg any) {
	payload, err := json.Marshal(msg)
	must(err)
	_, err = conn.WriteToUDP(payload, addr)
	must(err)
}

func udpRecv(conn *net.UDPConn) map[string]any {
	must(conn.SetReadDeadline(time.Now().Add(5 * time.Second)))
	buf := make([]byte, 64*1024)
	n, _, err := conn.ReadFromUDP(buf)
	must(err)
	var msg map[string]any
	must(json.Unmarshal(buf[:n], &msg))
	if msg["kind"] == "error" {
		fail("udp error response: %+v", msg)
	}
	return msg
}

func freeTCPPort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func freeUDPPort() int {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	must(err)
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func must(err error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
