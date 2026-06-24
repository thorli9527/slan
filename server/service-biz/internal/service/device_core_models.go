package service

import "github.com/slan/service-biz/internal/pkg/mqttkit"

type DeviceMQTTProfileView struct {
	Enabled         bool                `json:"enabled"`
	BrokerURL       string              `json:"brokerUrl"`
	TopicPrefix     string              `json:"topicPrefix"`
	ClientID        string              `json:"clientId"`
	Username        string              `json:"username"`
	Password        string              `json:"password"`
	ExpiresAt       int64               `json:"expiresAt"`
	PublishTopics   []string            `json:"publishTopics"`
	SubscribeTopics []string            `json:"subscribeTopics"`
	NetworkIDs      []string            `json:"networkIds"`
	Credential      *mqttkit.Credential `json:"credential,omitempty"`
}

type DeviceView struct {
	DeviceID      string `json:"deviceId"`
	OwnerID       string `json:"ownerId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
	RXBytesTotal  int64  `json:"rxBytesTotal"`
	TXBytesTotal  int64  `json:"txBytesTotal"`
	Status        string `json:"status"`
	LastSeenAt    int64  `json:"lastSeenAt"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type DeviceProfileView struct {
	Device           DeviceView `json:"device"`
	ActiveNetworkID  string     `json:"activeNetworkId"`
	OwnerEmail       string     `json:"ownerEmail"`
	MembershipStatus string     `json:"membershipStatus"`
	CurrentVirtualIP string     `json:"currentVirtualIp"`
	VirtualIP        string     `json:"virtualIp"`
	GlobalIP         string     `json:"globalIp"`
	GlobalName       string     `json:"globalName"`
}
