package service

import "github.com/slan/service-biz/internal/pkg/mqttkit"

type DeviceBootstrapKeyView struct {
	KeyID          string `json:"keyId"`
	UserID         string `json:"userId"`
	Name           string `json:"name"`
	DeviceAlias    string `json:"deviceAlias"`
	Token          string `json:"token"`
	Status         string `json:"status"`
	ExpiresAt      int64  `json:"expiresAt"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	NetworkID      string `json:"networkId"`
	UsedAt         int64  `json:"usedAt"`
	UsedByDeviceID string `json:"usedByDeviceId"`
	RevokedAt      int64  `json:"revokedAt"`
}

type DeviceSessionBootstrapView struct {
	Profile    DeviceProfileView     `json:"profile"`
	Device     DeviceView            `json:"device"`
	Session    DeviceSessionView     `json:"session"`
	MQTT       DeviceMQTTProfileView `json:"mqtt"`
	Credential *mqttkit.Credential   `json:"credential,omitempty"`
}
