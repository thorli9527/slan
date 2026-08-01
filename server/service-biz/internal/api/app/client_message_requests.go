package app

import servicepkg "github.com/slan/service-biz/internal/service"

type sendClientMessageRequest struct {
	NetworkID      string         `json:"networkId"`
	FromDeviceID   string         `json:"fromDeviceId"`
	TargetDeviceID string         `json:"targetDeviceId"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata"`
}

func (r sendClientMessageRequest) toInput() servicepkg.SendClientMessageInput {
	return servicepkg.SendClientMessageInput{
		NetworkID:      r.NetworkID,
		FromDeviceID:   r.FromDeviceID,
		TargetDeviceID: r.TargetDeviceID,
		Body:           r.Body,
		Metadata:       r.Metadata,
	}
}
