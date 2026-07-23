package wirekit

import (
	"net"
	"strconv"
	"strings"
	"time"
)

type NodeView struct {
	RegionID          string           `json:"regionId"`
	NodeID            string           `json:"nodeId"`
	Name              string           `json:"name,omitempty"`
	Host              string           `json:"host"`
	UDPPort           int              `json:"udpPort,omitempty"`
	AdminPort         int              `json:"adminPort,omitempty"`
	Port              int              `json:"port,omitempty"`
	Enabled           bool             `json:"enabled"`
	Healthy           bool             `json:"healthy"`
	Stale             bool             `json:"stale"`
	Priority          int              `json:"priority"`
	UpdatedAtMS       int64            `json:"updatedAtMs"`
	TicketKeyRotation *TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type TicketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

type PeerAuthz struct {
	PeerID      string   `json:"peerId"`
	NetworkID   string   `json:"networkId"`
	NodeID      string   `json:"nodeId"`
	Enabled     bool     `json:"enabled"`
	VirtualIPs  []string `json:"virtualIps"`
	AllowedIPs  []string `json:"allowedIps"`
	QuotaPolicy string   `json:"quotaPolicy"`
}

type PeerRuntimeConfig struct {
	PeerID                string   `json:"peerId"`
	NetworkID             string   `json:"networkId"`
	NodeID                string   `json:"nodeId"`
	VirtualIPs            []string `json:"virtualIps"`
	AllowedIPs            []string `json:"allowedIps"`
	KeepaliveIntervalSecs int      `json:"keepaliveIntervalSecs"`
	NetworkEnabled        bool     `json:"networkEnabled"`
	PreferredPath         string   `json:"preferredPath"`
	Endpoints             []string `json:"endpoints"`
}

type Topology struct {
	NetworkID string         `json:"networkId"`
	Peers     []TopologyPeer `json:"peers"`
}

type TopologyPeer struct {
	PeerID                  string   `json:"peerId"`
	NetworkID               string   `json:"networkId"`
	NodeID                  string   `json:"nodeId"`
	PublicKey               string   `json:"publicKey"`
	VirtualIPs              []string `json:"virtualIPs"`
	AllowedIPs              []string `json:"allowedIPs"`
	SupportsLanDirect       bool     `json:"supportsLanDirect"`
	SupportsIpv6Direct      bool     `json:"supportsIpv6Direct"`
	SupportsDirectUdp       bool     `json:"supportsDirectUdp"`
	SupportsRelayUdp        bool     `json:"supportsRelayUdp"`
	SupportsDerpTcpTls443   bool     `json:"supportsDerpTcpTls443"`
	PreferIpv6              bool     `json:"preferIpv6"`
	PreferLan               bool     `json:"preferLan"`
	AllowEndpointRoaming    bool     `json:"allowEndpointRoaming"`
	AllowFastReselection    bool     `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool     `json:"allowRelayTicketRenewal"`
	KeepaliveIntervalSecs   int      `json:"keepaliveIntervalSecs"`
	Endpoints               []string `json:"endpoints"`
	ActivePath              string   `json:"activePath"`
	RequireMtuRefresh       bool     `json:"requireMtuRefresh"`
	EndpointChanged         bool     `json:"endpointChanged"`
	UpdatedAt               int64    `json:"updatedAt"`
}

type DerpMap struct {
	PreferredRegionID string       `json:"preferredRegionId"`
	Regions           []DerpRegion `json:"regions"`
}

type DerpRegion struct {
	RegionID string     `json:"regionId"`
	Name     string     `json:"name"`
	Nodes    []DerpNode `json:"nodes"`
}

type DerpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

func PeerID(networkID, deviceID string) string {
	return strings.TrimSpace(networkID) + ":" + strings.TrimSpace(deviceID)
}

func ParsePeerID(peerID string) (string, string) {
	peerID = strings.TrimSpace(peerID)
	if strings.Contains(peerID, ":") {
		parts := strings.SplitN(peerID, ":", 2)
		return strings.TrimSpace(parts[0]), strings.TrimPrefix(strings.TrimSpace(parts[1]), "node-")
	}
	deviceID := strings.TrimPrefix(peerID, "node-")
	return "", strings.TrimSpace(deviceID)
}

func NodeID(deviceID string) string {
	return "node-" + strings.TrimSpace(deviceID)
}

func HostPort(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func SplitHostPort(value string) (string, int) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "udp://"))
	if value == "" {
		return "", 0
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return value, 0
	}
	port, _ := strconv.Atoi(portText)
	return host, port
}

func IsNodeStale(updatedAt, now, freshness int64) bool {
	if updatedAt <= 0 || freshness <= 0 {
		return false
	}
	return now-updatedAt > freshness
}

func StatusEnabled(status string) bool {
	status = strings.TrimSpace(status)
	return status != "" && status != "disabled"
}

func StatusHealthy(status string) bool {
	return strings.TrimSpace(status) == "active"
}

func FreshnessSeconds() int64 {
	return 120
}

func NowUnix() int64 {
	return time.Now().Unix()
}
