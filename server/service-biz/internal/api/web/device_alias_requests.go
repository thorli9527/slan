package web

import servicepkg "github.com/slan/service-biz/internal/service"

type updateDeviceAliasRequest struct {
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}

func (r updateDeviceAliasRequest) toInput() servicepkg.UpdateDeviceAliasInput {
	return servicepkg.UpdateDeviceAliasInput{
		DeviceID:    r.DeviceID,
		ActorUserID: r.ActorUserID,
		Alias:       r.Alias,
	}
}
