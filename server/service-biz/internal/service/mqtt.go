package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type MQTTAuthInput struct {
	ClientID string
	Username string
	Password string
	IsV5     bool
}

type MQTTAuthView struct {
	Allowed   bool
	TenantID  string
	UserID    string
	Principal string
	DeviceID  string
}

type MQTTCheckInput struct {
	Principal string
	DeviceID  string
	UserID    string
	ClientID  string
	Username  string
	Topic     string
	Subscribe bool
	Connect   bool
}

type MQTTEndpointReportInput struct {
	NetworkID string
	DeviceID  string
	NodeID    string
	NATType   string
	Endpoints []DeviceEndpointView
}

type MQTTPathHealthReportInput struct {
	NetworkID         string
	DeviceID          string
	PeerNodeID        string
	PathType          string
	ActivePath        string
	RelayTransport    string
	Endpoint          string
	DerpNodeID        string
	ObservedRttMs     int64
	PacketLossPpm     int64
	PathScore         int64
	RelayMtu          int
	MaxFramePayload   int
	TicketExpiresAt   string
	TicketExpiresInMs int64
	TicketRenewDue    bool
	PathDowngrades    int64
	PathUpgrades      int64
	LastPathChange    string
	SampledAtMs       int64
}

type MQTTUseCase interface {
	Authenticate(ctx context.Context, input MQTTAuthInput) (MQTTAuthView, error)
	CheckACL(ctx context.Context, input MQTTCheckInput) (bool, error)
	ReportEndpoint(ctx context.Context, input MQTTEndpointReportInput) (bool, error)
	ReportPathHealth(ctx context.Context, input MQTTPathHealthReportInput) error
}

type MQTTWebhookService struct {
	Networks repository.NetworkRepository
	Config   mqttkit.Config
	Now      func() time.Time
}

type MQTTControlUpEnvelope struct {
	Type      string          `json:"type"`
	MessageID string          `json:"messageId"`
	RequestID string          `json:"requestId"`
	NetworkID string          `json:"networkId"`
	Payload   json.RawMessage `json:"payload"`
}
