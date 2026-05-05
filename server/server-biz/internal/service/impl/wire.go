package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

type dbWireService struct{ state *dbState }

func (s dbWireService) PeerAuthz(peerID string) (dto.WirePeerAuthzView, error) {
	ctx := context.Background()
	node, err := s.nodeForWirePeer(ctx, peerID)
	if err != nil {
		return dto.WirePeerAuthzView{}, err
	}
	networkID, err := s.activeNetworkForWireNode(ctx, node)
	if err != nil {
		return dto.WirePeerAuthzView{}, err
	}
	return s.wirePeerAuthz(ctx, node, networkID)
}

func (s dbWireService) PeerRuntimeConfig(peerID string) (dto.WirePeerRuntimeConfigView, error) {
	ctx := context.Background()
	node, err := s.nodeForWirePeer(ctx, peerID)
	if err != nil {
		return dto.WirePeerRuntimeConfigView{}, err
	}
	networkID, err := s.activeNetworkForWireNode(ctx, node)
	if err != nil {
		return dto.WirePeerRuntimeConfigView{}, err
	}
	authz, err := s.wirePeerAuthz(ctx, node, networkID)
	if err != nil {
		return dto.WirePeerRuntimeConfigView{}, err
	}
	return dto.WirePeerRuntimeConfigView{
		PeerID:               authz.PeerID,
		DeviceID:             authz.DeviceID,
		NodeID:               authz.NodeID,
		NetworkID:            authz.NetworkID,
		VirtualIPs:           authz.VirtualIPs,
		AllowedIPs:           authz.AllowedIPs,
		DNS:                  s.state.buildNetworkMapDNS(ctx, node.UserID, networkID),
		DefaultKeepaliveSecs: defaultControlHeartbeatSeconds,
		NetworkEnabled:       authz.Enabled,
	}, nil
}

func (s dbWireService) NetworkTopology(networkID string) (dto.WireNetworkTopologyView, error) {
	ctx := context.Background()
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		return dto.WireNetworkTopologyView{}, fmt.Errorf("%w: networkId is required", ErrInvalidArgument)
	}
	if _, err := s.state.pg.GetNetworkByID(ctx, networkID); err != nil {
		return dto.WireNetworkTopologyView{}, ErrNotFound
	}
	nodes, err := s.state.pg.ListNodesByNetwork(ctx, networkID)
	if err != nil {
		return dto.WireNetworkTopologyView{}, err
	}
	peers := make([]dto.WireTopologyPeer, 0, len(nodes))
	for _, node := range nodes {
		authz, err := s.wirePeerAuthz(ctx, node, networkID)
		if err != nil {
			continue
		}
		peers = append(peers, dto.WireTopologyPeer{
			PeerID:         node.NodeID,
			DeviceID:       node.DeviceID,
			NodeID:         node.NodeID,
			PublicKey:      node.NodePublicKey,
			VirtualIPs:     authz.VirtualIPs,
			AllowedIPs:     authz.AllowedIPs,
			NetworkEnabled: authz.Enabled,
		})
	}
	return dto.WireNetworkTopologyView{
		NetworkID: networkID,
		Peers:     peers,
		DNS:       s.state.buildNetworkMapDNS(ctx, "", networkID),
	}, nil
}

func (s dbWireService) nodeForWirePeer(ctx context.Context, peerID string) (repo.Node, error) {
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return repo.Node{}, fmt.Errorf("%w: peerId is required", ErrInvalidArgument)
	}
	node, err := s.state.pg.GetNodeByID(ctx, peerID)
	if err != nil {
		return repo.Node{}, ErrNotFound
	}
	return node, nil
}

func (s dbWireService) activeNetworkForWireNode(ctx context.Context, node repo.Node) (string, error) {
	members, err := s.state.pg.ListMembersByDevice(ctx, node.DeviceID)
	if err != nil {
		return "", err
	}
	for _, member := range members {
		if member.Status != "active" {
			continue
		}
		if _, err := s.state.requireActiveNetworkAttachment(ctx, member.NetworkID, node.DeviceID, ErrForbidden, "node device"); err == nil {
			return member.NetworkID, nil
		}
	}
	return "", ErrForbidden
}

func (s dbWireService) wirePeerAuthz(ctx context.Context, node repo.Node, networkID string) (dto.WirePeerAuthzView, error) {
	if _, err := s.state.requireActiveNetworkMember(ctx, networkID, node.DeviceID, ErrForbidden, "node device"); err != nil {
		return dto.WirePeerAuthzView{}, err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, networkID, node.DeviceID, ErrForbidden, "node device"); err != nil {
		return dto.WirePeerAuthzView{}, err
	}
	return dto.WirePeerAuthzView{
		PeerID:      node.NodeID,
		DeviceID:    node.DeviceID,
		NodeID:      node.NodeID,
		NetworkID:   networkID,
		Enabled:     true,
		VirtualIPs:  s.state.virtualIPsForDeviceInNetwork(ctx, node.DeviceID, networkID),
		AllowedIPs:  s.state.allowedRoutesForDeviceInNetwork(ctx, node.DeviceID, networkID),
		QuotaPolicy: "default",
	}, nil
}
