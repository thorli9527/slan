package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestOpsNodeServiceUpsertRelayNodeValidatesServerSide(t *testing.T) {
	now := time.Unix(1710000000, 0)
	tests := []struct {
		name    string
		repo    *opsNodeTestRepo
		input   UpsertNodeInput
		wantErr error
	}{
		{
			name: "rejects non ipv4 relay address",
			repo: &opsNodeTestRepo{},
			input: UpsertNodeInput{
				Name:      "relay-a",
				Endpoint:  "relay.example.com:29110",
				Transport: relayTransportUDP,
				Status:    nodeStatusActive,
				Health:    nodeHealthHealthy,
			},
			wantErr: ErrInvalidArgument,
		},
		{
			name: "rejects duplicate relay endpoint",
			repo: &opsNodeTestRepo{
				relayNodes: []model.RelayNode{{
					NodeID:    "relay000000000000000000000000000001",
					Name:      "relay-existing",
					Endpoint:  "47.245.40.231:29110",
					Transport: relayTransportUDP,
					Status:    nodeStatusActive,
					Health:    nodeHealthHealthy,
				}},
			},
			input: UpsertNodeInput{
				Name:      "relay-b",
				Endpoint:  "udp://47.245.40.231:29110",
				Transport: relayTransportUDP,
				Status:    nodeStatusActive,
				Health:    nodeHealthHealthy,
			},
			wantErr: ErrConflict,
		},
		{
			name: "accepts valid derp relay",
			repo: &opsNodeTestRepo{},
			input: UpsertNodeInput{
				Name:      "derp-a",
				Endpoint:  "47.245.40.231:29120",
				Transport: relayTransportDerpTLS,
				Status:    nodeStatusActive,
				Health:    nodeHealthHealthy,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := OpsNodeService{
				Nodes: tt.repo,
				Now:   func() time.Time { return now },
			}
			_, err := svc.UpsertRelayNode(context.Background(), tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && len(tt.repo.savedRelayNodes) != 1 {
				t.Fatalf("expected relay node to be saved")
			}
			if tt.wantErr == nil && tt.repo.savedRelayNodes[0].Region != tt.repo.savedRelayNodes[0].NodeID {
				t.Fatalf("relay protocol region must be generated from node ID: %+v", tt.repo.savedRelayNodes[0])
			}
		})
	}
}

func TestOpsNodeServiceUpsertPunchNodeValidatesServerSide(t *testing.T) {
	now := time.Unix(1710000000, 0)
	tests := []struct {
		name    string
		repo    *opsNodeTestRepo
		input   UpsertNodeInput
		wantErr error
	}{
		{
			name: "rejects invalid udp port",
			repo: &opsNodeTestRepo{},
			input: UpsertNodeInput{
				Name:     "punch-a",
				Endpoint: "47.245.40.231:65535",
				Status:   nodeStatusActive,
				Health:   nodeHealthHealthy,
			},
			wantErr: ErrInvalidArgument,
		},
		{
			name: "rejects duplicate punch endpoint",
			repo: &opsNodeTestRepo{
				punchNodes: []model.PunchNode{{
					NodeID:   "punch000000000000000000000000000001",
					Name:     "punch-existing",
					Endpoint: "47.245.40.231:29130",
					Status:   nodeStatusActive,
					Health:   nodeHealthHealthy,
				}},
			},
			input: UpsertNodeInput{
				Name:     "punch-b",
				Endpoint: "47.245.40.231:29130",
				Status:   nodeStatusActive,
				Health:   nodeHealthHealthy,
			},
			wantErr: ErrConflict,
		},
		{
			name: "accepts valid punch endpoint",
			repo: &opsNodeTestRepo{},
			input: UpsertNodeInput{
				Name:     "punch-c",
				Endpoint: "47.245.40.231:29130",
				Status:   nodeStatusActive,
				Health:   nodeHealthHealthy,
				Priority: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := OpsNodeService{
				Nodes: tt.repo,
				Now:   func() time.Time { return now },
			}
			_, err := svc.UpsertPunchNode(context.Background(), tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && len(tt.repo.savedPunchNodes) != 1 {
				t.Fatalf("expected punch node to be saved")
			}
		})
	}
}

type opsNodeTestRepo struct {
	relayNodes      []model.RelayNode
	punchNodes      []model.PunchNode
	savedRelayNodes []model.RelayNode
	savedPunchNodes []model.PunchNode
}

func (r *opsNodeTestRepo) ListRelayNodes(context.Context) ([]model.RelayNode, error) {
	items := make([]model.RelayNode, len(r.relayNodes))
	copy(items, r.relayNodes)
	return items, nil
}

func (r *opsNodeTestRepo) GetRelayNode(_ context.Context, nodeID string) (model.RelayNode, bool, error) {
	for _, item := range r.relayNodes {
		if item.NodeID == nodeID {
			return item, true, nil
		}
	}
	return model.RelayNode{}, false, nil
}

func (r *opsNodeTestRepo) SaveRelayNode(_ context.Context, item model.RelayNode) error {
	r.savedRelayNodes = append(r.savedRelayNodes, item)
	return nil
}

func (r *opsNodeTestRepo) DeleteRelayNode(context.Context, string) error { return nil }

func (r *opsNodeTestRepo) ListPunchNodes(context.Context) ([]model.PunchNode, error) {
	items := make([]model.PunchNode, len(r.punchNodes))
	copy(items, r.punchNodes)
	return items, nil
}

func (r *opsNodeTestRepo) GetPunchNode(_ context.Context, nodeID string) (model.PunchNode, bool, error) {
	for _, item := range r.punchNodes {
		if item.NodeID == nodeID {
			return item, true, nil
		}
	}
	return model.PunchNode{}, false, nil
}

func (r *opsNodeTestRepo) SavePunchNode(_ context.Context, item model.PunchNode) error {
	r.savedPunchNodes = append(r.savedPunchNodes, item)
	return nil
}

func (r *opsNodeTestRepo) DeletePunchNode(context.Context, string) error { return nil }

func (r *opsNodeTestRepo) NewRelayNodeID() string { return "relay000000000000000000000000000999" }
func (r *opsNodeTestRepo) NewPunchNodeID() string { return "punch000000000000000000000000000999" }
