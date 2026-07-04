package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type SendClientMessageInput struct {
	NetworkID      string
	FromDeviceID   string
	TargetDeviceID string
	Body           string
	Metadata       map[string]any
}

type SendClientMessageView struct {
	MessageID string `json:"messageId"`
	Transport string `json:"transport"`
	QOS       int    `json:"qos"`
	Topic     string `json:"topic"`
}

type ClientMessageUseCase interface {
	SendClientMessage(ctx context.Context, input SendClientMessageInput) (SendClientMessageView, error)
}

type ClientMessageService struct {
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	MQTT     mqttkit.Config
	Now      func() int64
}
