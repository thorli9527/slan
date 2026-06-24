package service

import "github.com/slan/service-biz/internal/pkg/wirekit"

type WireUpsertNodeInput struct {
	RegionID          string                   `json:"regionId"`
	NodeID            string                   `json:"nodeId"`
	Name              string                   `json:"name"`
	Host              string                   `json:"host"`
	UDPPort           int                      `json:"udpPort,omitempty"`
	AdminPort         int                      `json:"adminPort,omitempty"`
	Port              int                      `json:"port,omitempty"`
	Priority          int                      `json:"priority,omitempty"`
	TicketKeyRotation *wirekit.TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
	Enabled           *bool                    `json:"enabled,omitempty"`
	Healthy           *bool                    `json:"healthy,omitempty"`
}

type WireNodeStatusInput struct {
	RegionID          string                   `json:"regionId"`
	NodeID            string                   `json:"nodeId"`
	Enabled           *bool                    `json:"enabled,omitempty"`
	Healthy           *bool                    `json:"healthy,omitempty"`
	TicketKeyRotation *wirekit.TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type WireNodeDeleteInput struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
}
