package app

import servicepkg "github.com/slan/service-biz/internal/service"

type bootstrapDeviceSessionRequest struct {
	SessionKey    string `json:"sessionKey"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	OwnerUserID   string `json:"ownerUserId"`
	OwnerID       string `json:"ownerId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
}

func (r bootstrapDeviceSessionRequest) toInput() servicepkg.BootstrapDeviceSessionInput {
	return servicepkg.BootstrapDeviceSessionInput{
		SessionKey:    r.SessionKey,
		DeviceID:      r.DeviceID,
		OwnerID:       firstNonEmpty(r.OwnerID, r.OwnerUserID, r.UserID),
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
	}
}

type bindDeviceSessionRequest struct {
	DeviceID      string `json:"deviceId"`
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
	LastSeenAt    int64  `json:"lastSeenAt"`
}

func (r bindDeviceSessionRequest) toInput() servicepkg.BindDeviceSessionInput {
	return servicepkg.BindDeviceSessionInput{
		DeviceID:      r.DeviceID,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
		RXBytesTotal:  r.RXBytesTotal,
		TXBytesTotal:  r.TXBytesTotal,
		LastSeenAt:    r.LastSeenAt,
	}
}

type renewDeviceSessionRequest struct {
	NetworkEnabled *bool `json:"networkEnabled"`
	RXBytesTotal   int64 `json:"rxBytesTotal"`
	TXBytesTotal   int64 `json:"txBytesTotal"`
	LastSeenAt     int64 `json:"lastSeenAt"`
}

func (r renewDeviceSessionRequest) toInput() servicepkg.RenewDeviceSessionInput {
	return servicepkg.RenewDeviceSessionInput{
		NetworkEnabled: r.NetworkEnabled,
		RXBytesTotal:   r.RXBytesTotal,
		TXBytesTotal:   r.TXBytesTotal,
		LastSeenAt:     r.LastSeenAt,
	}
}
