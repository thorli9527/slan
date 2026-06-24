package web

import servicepkg "github.com/slan/service-biz/internal/service"

type createDeviceBootstrapKeyRequest struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	NetworkID   string `json:"networkId"`
	TTLSeconds  int64  `json:"ttlSeconds"`
	ExpiresAt   int64  `json:"expiresAt"`
	DeviceAlias string `json:"deviceAlias"`
	namedDeviceRequest
}

func (r createDeviceBootstrapKeyRequest) toInput() servicepkg.CreateDeviceBootstrapKeyInput {
	return servicepkg.CreateDeviceBootstrapKeyInput{
		UserID:      r.UserID,
		ActorUserID: r.ActorUserID,
		NetworkID:   r.NetworkID,
		TTLSeconds:  r.TTLSeconds,
		ExpiresAt:   r.ExpiresAt,
		Name:        r.value(r.DeviceAlias, r.NetworkID, "bootstrap"),
	}
}

type revokeDeviceBootstrapKeyRequest struct {
	KeyID       string `json:"keyId"`
	ActorUserID string `json:"actorUserId"`
}

func (r revokeDeviceBootstrapKeyRequest) toInput() servicepkg.RevokeDeviceBootstrapKeyInput {
	return servicepkg.RevokeDeviceBootstrapKeyInput{
		KeyID:       r.KeyID,
		ActorUserID: r.ActorUserID,
	}
}
