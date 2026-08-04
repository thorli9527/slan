package app

import servicepkg "github.com/slan/service-biz/internal/service"

type renewDeviceSessionRequest struct {
	RefreshToken   string `json:"refreshToken"`
	NetworkEnabled *bool  `json:"networkEnabled"`
	RXBytesTotal   int64  `json:"rxBytesTotal"`
	TXBytesTotal   int64  `json:"txBytesTotal"`
	LastSeenAt     int64  `json:"lastSeenAt"`
}

func (r renewDeviceSessionRequest) toInput() servicepkg.RenewDeviceSessionInput {
	return servicepkg.RenewDeviceSessionInput{
		RefreshToken:   r.RefreshToken,
		NetworkEnabled: r.NetworkEnabled,
		RXBytesTotal:   r.RXBytesTotal,
		TXBytesTotal:   r.TXBytesTotal,
		LastSeenAt:     r.LastSeenAt,
	}
}
