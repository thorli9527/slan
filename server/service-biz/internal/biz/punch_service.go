package biz

import "context"

// PunchService 承载 biz 到 punch-service 的授权和转发编排。
type PunchService struct {
	store BusinessStore
	mqtt  MQTTConfig
}

func (s PunchService) CreateConnectSession(ctx context.Context, networkID string, req CreatePunchConnectSessionRequest, auth punchDeviceAuth) (map[string]any, int, error) {
	if err := s.store.AuthorizePunchConnect(networkID, req.RequesterNodeID, req.PeerNodeID, auth, s.mqtt); err != nil {
		return nil, 0, err
	}
	return requestPunchConnectSession(ctx, s.store.ActivePunchNodes(), networkID, req.RequesterNodeID, req.PeerNodeID, req.TTLSeconds, auth)
}
