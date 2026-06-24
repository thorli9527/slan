package service

import "github.com/slan/service-biz/internal/pkg/wirekit"

type OpsRelayNodeView struct {
	NodeID            string                   `json:"nodeId"`
	Name              string                   `json:"name"`
	Region            string                   `json:"region"`
	Endpoint          string                   `json:"endpoint"`
	Status            string                   `json:"status"`
	CreatedAt         int64                    `json:"createdAt"`
	UpdatedAt         int64                    `json:"updatedAt"`
	Transport         string                   `json:"transport"`
	PublicAddr        string                   `json:"publicAddr"`
	Priority          int                      `json:"priority"`
	TicketKeyRotation *wirekit.TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
	MaxBandwidthMbps  int                      `json:"maxBandwidthMbps"`
	MonthlyTrafficGB  int                      `json:"monthlyTrafficGb"`
	UsedTrafficGB     int                      `json:"usedTrafficGb"`
	MaxSessions       int                      `json:"maxSessions"`
	ActiveSessions    int                      `json:"activeSessions"`
	Health            string                   `json:"health"`
}

type OpsPunchNodeView struct {
	NodeID         string `json:"nodeId"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	Endpoint       string `json:"endpoint"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	PublicUDPIP    string `json:"publicUdpIp"`
	PublicUDPPort  int    `json:"publicUdpPort"`
	MaxSessions    int    `json:"maxSessions"`
	ActiveSessions int    `json:"activeSessions"`
	Health         string `json:"health"`
	Priority       int    `json:"priority"`
}
