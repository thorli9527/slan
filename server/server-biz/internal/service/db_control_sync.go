package service

import (
	"context"

	controlws "github.com/slan/server/server-biz/internal/ws"
)

func (s dbControlSyncService) Publish(event controlws.ControlSyncEvent) error {
	return s.state.tokens.PublishControlSyncEvent(context.Background(), event)
}

func (s dbControlSyncService) Subscribe(handler func(controlws.ControlSyncEvent)) error {
	return s.state.tokens.SubscribeControlSyncEvents(context.Background(), handler)
}

func (s dbControlSyncService) NextRevision(networkID string) (uint64, error) {
	return s.state.tokens.NextNetworkRevision(context.Background(), networkID)
}

func (s dbControlSyncService) CurrentRevision(networkID string) (uint64, error) {
	return s.state.tokens.CurrentNetworkRevision(context.Background(), networkID)
}
