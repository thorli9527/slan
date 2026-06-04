package biz

import (
	"encoding/json"
)

// DeviceUserLoginPayload 是客户端设备登录完成后，通过控制通道下发给设备的用户授权载荷。
type DeviceUserLoginPayload struct {
	AccessToken  string  `json:"accessToken"`
	UserToken    string  `json:"userToken,omitempty"`
	RefreshToken *string `json:"refreshToken,omitempty"`
	UserID       string  `json:"userId"`
	UserLabel    string  `json:"userLabel"`
	DeviceID     *string `json:"deviceId,omitempty"`
	VirtualIP    *string `json:"virtualIp,omitempty"`
	ExpiresIn    uint64  `json:"expiresIn,omitempty"`
	Action       string  `json:"action,omitempty"`
}

// MQTTControlDelivery 记录一条通过 MQTT 下发到设备的控制任务及其投递状态。
type MQTTControlDelivery struct {
	DeliveryID    string          `json:"deliveryId"`
	DeviceID      string          `json:"deviceId"`
	MessageType   string          `json:"messageType,omitempty"`
	TaskID        string          `json:"taskId,omitempty"`
	Action        string          `json:"action,omitempty"`
	Status        string          `json:"status"`
	AttemptCount  int             `json:"attemptCount,omitempty"`
	Error         string          `json:"error,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	ProcessedAtMs int64           `json:"processedAtMs,omitempty"`
	CreatedAt     int64           `json:"createdAt,omitempty"`
	ExpiresAt     int64           `json:"expiresAt,omitempty"`
	PublishedAt   int64           `json:"publishedAt,omitempty"`
	AckedAt       int64           `json:"ackedAt"`
	UpdatedAt     int64           `json:"updatedAt"`
}
