package service

import "github.com/slan/service-biz/internal/pkg/mqttkit"

type DeviceLoginDeviceView struct {
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
	VerifyCode    string `json:"verifyCode"`
	Status        string `json:"status"`
	ExpiresAt     int64  `json:"expiresAt"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type PrepareDeviceLoginDeviceView struct {
	Device     DeviceLoginDeviceView `json:"device"`
	Credential *mqttkit.Credential   `json:"credential,omitempty"`
}

type CompleteDeviceLoginDeviceView struct {
	Device DeviceLoginDeviceView `json:"device"`
}
