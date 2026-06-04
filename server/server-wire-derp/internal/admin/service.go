package admin

import "github.com/slan/server/server-wire-derp/internal/state"

// Service 承载 DERP 管理 HTTP 的只读查询业务。
type Service struct {
	store state.AdminViewStore
}

func NewService(store state.AdminViewStore) *Service {
	return &Service{store: store}
}

func (s *Service) Connections() []state.ConnectionView {
	return s.store.Connections()
}

func (s *Service) Connection(peerID string) (state.ConnectionView, bool) {
	return s.store.Connection(peerID)
}

func (s *Service) Sessions() []state.SessionView {
	return s.store.Sessions()
}

func (s *Service) Session(sessionID string) (state.SessionView, bool) {
	return s.store.Session(sessionID)
}

func (s *Service) Regions() []state.RegionView {
	return s.store.Regions()
}

func (s *Service) Metrics() state.Metrics {
	return s.store.Metrics()
}
