package store

import (
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

type Store interface {
	RegisterPeer(model.PeerRegistration) (model.PeerRecord, error)
	UpdateEndpoints(peerID string, endpoints []model.Endpoint) (model.PeerRecord, error)
	UpdatePathHealth(peerID string, probes []model.PathProbe) (model.PeerRecord, error)
	UpdateDerpHealth(peerID string, samples []model.DerpHealthSample) (model.PeerRecord, error)
	UpdateActivePath(peerID string, path model.PathKind) (model.PeerRecord, error)
	IssueRelayTicket(peerID string, relay model.RelayNode, ttl time.Duration, renewAfter time.Duration) (model.RelayTicket, error)
	IssueDerpTicket(peerID, regionID, nodeID string, ttl time.Duration, renewAfter time.Duration) (model.DerpTicket, error)
	DerpMap() model.DerpMap
	GetPeer(peerID string) (model.PeerRecord, bool)
	ListPeersByNetwork(networkID string) []model.PeerRecord
}
