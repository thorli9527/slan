package httpapi

import (
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/util"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

func fanoutPeerUpdate(deps routerDeps, session wsSession) {
	peer, err := deps.ControlChannel.PeerSnapshot(session.userID, session.nodeID, session.networkID, session.nodeID)
	if err != nil {
		return
	}
	revision := uint64(1)
	if deps.ControlSync != nil {
		if next, err := deps.ControlSync.NextRevision(session.networkID); err == nil && next > 0 {
			revision = next
		}
	}
	wsPeer := dtoPeerToWSPeer(peer)
	broadcastPeerUpdateToSessions(deps, session.networkID, session.nodeID, revision, wsPeer)
	metricAdd("peer_update_broadcast_total", 1)
	if deps.ControlSync != nil {
		_ = deps.ControlSync.Publish(controlws.ControlSyncEvent{
			InstanceID:   controlWSInstanceID,
			Type:         "peer_update",
			NetworkID:    session.networkID,
			SourceNodeID: session.nodeID,
			Revision:     revision,
			Peer:         util.Ptr(wsPeer),
		})
		metricAdd("sync_event_published_total", 1)
	}
}

func fanoutPeerRemove(deps routerDeps, networkID, sourceNodeID string) {
	revision := uint64(1)
	if deps.ControlSync != nil {
		if next, err := deps.ControlSync.NextRevision(networkID); err == nil && next > 0 {
			revision = next
		}
	}
	broadcastPeerRemoveToSessions(deps, networkID, sourceNodeID, revision)
	metricAdd("peer_remove_broadcast_total", 1)
	if deps.ControlSync != nil {
		_ = deps.ControlSync.Publish(controlws.ControlSyncEvent{
			InstanceID:   controlWSInstanceID,
			Type:         "peer_remove",
			NetworkID:    networkID,
			SourceNodeID: sourceNodeID,
			Revision:     revision,
			PeerNodeID:   sourceNodeID,
		})
		metricAdd("sync_event_published_total", 1)
	}
}

func fanoutConnectPlans(deps routerDeps, session wsSession) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(session.networkID, session.nodeID) {
		planForSource, err := deps.ControlChannel.ConnectPlan(session.userID, session.nodeID, session.networkID, peerSession.nodeID)
		if err == nil {
			sendConnectPlanToNode(deps, session.networkID, session.nodeID, session.nodeID, planForSource)
			publishConnectPlan(deps, session.networkID, session.nodeID, session.nodeID, planForSource)
		}
		planForPeer, err := deps.ControlChannel.ConnectPlan(peerSession.userID, peerSession.nodeID, peerSession.networkID, session.nodeID)
		if err == nil {
			sendConnectPlanToNode(deps, peerSession.networkID, peerSession.nodeID, peerSession.nodeID, planForPeer)
			publishConnectPlan(deps, peerSession.networkID, peerSession.nodeID, peerSession.nodeID, planForPeer)
		}
	}
}

func fanoutConnectPlanPair(deps routerDeps, session wsSession, peerNodeID string) {
	if peerNodeID == "" {
		return
	}

	planForSource, err := deps.ControlChannel.ConnectPlan(session.userID, session.nodeID, session.networkID, peerNodeID)
	if err == nil {
		sendConnectPlanToNode(deps, session.networkID, session.nodeID, session.nodeID, planForSource)
	}

	planForPeer, err := deps.ControlChannel.ConnectPlanByNode(peerNodeID, session.networkID, session.nodeID)
	if err == nil {
		sendConnectPlanToNode(deps, session.networkID, peerNodeID, peerNodeID, planForPeer)
		publishConnectPlan(deps, session.networkID, peerNodeID, peerNodeID, planForPeer)
	}
}

func publishConnectPlan(deps routerDeps, networkID, sourceNodeID, targetNodeID string, plan controlws.ConnectPlan) {
	if deps.ControlSync == nil {
		return
	}
	rev, _ := deps.ControlSync.CurrentRevision(networkID)
	_ = deps.ControlSync.Publish(controlws.ControlSyncEvent{
		InstanceID:   controlWSInstanceID,
		Type:         "connect_plan",
		NetworkID:    networkID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Revision:     rev,
		Plan:         util.Ptr(plan),
	})
	metricAdd("sync_event_published_total", 1)
}

func sendPeerCandidateToNode(deps routerDeps, targetNodeID string, candidate controlws.PeerCandidate) {
	if targetNodeID == "" {
		return
	}
	session := defaultControlWSHub.session(targetNodeID)
	if session == nil {
		return
	}
	_ = session.sendTracked("peer_candidate", "", candidate, &deps)
}

func sendConnectPlanToNode(deps routerDeps, networkID, sourceNodeID, targetNodeID string, plan controlws.ConnectPlan) {
	if targetNodeID == "" {
		return
	}
	session := defaultControlWSHub.session(targetNodeID)
	if session == nil {
		return
	}
	metricRecordConnectPlan(networkID, sourceNodeID, targetNodeID, plan)
	_ = session.sendTracked("connect_plan", "", plan, &deps)
}

func broadcastPeerUpdateToSessions(deps routerDeps, networkID, sourceNodeID string, revision uint64, peer controlws.Peer) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, sourceNodeID) {
		_ = peerSession.sendTracked("peer_update", "", controlws.PeerUpdate{
			NetworkID: networkID,
			Revision:  revision,
			Peer:      peer,
		}, &deps)
	}
}

func broadcastPeerRemoveToSessions(deps routerDeps, networkID, sourceNodeID string, revision uint64) {
	if sourceNodeID == "" || networkID == "" {
		return
	}
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, sourceNodeID) {
		_ = peerSession.sendTracked("peer_remove", "", controlws.PeerRemove{
			NetworkID:  networkID,
			Revision:   revision,
			PeerNodeID: sourceNodeID,
		}, &deps)
	}
}

func broadcastNetworkRestartRequired(deps routerDeps, networkID string, restart controlws.NetworkRestartRequired) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, "") {
		_ = peerSession.sendTracked("network_restart_required", "", restart, &deps)
	}
}

func broadcastDeviceIPReassigned(deps routerDeps, networkID string, deviceIP controlws.DeviceIPReassigned) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, "") {
		_ = peerSession.sendTracked("device_ip_reassigned", "", deviceIP, &deps)
	}
}

func broadcastActiveNetworkEnabled(deps routerDeps, userID string, enabled controlws.ActiveNetworkEnabled) {
	if userID == "" {
		return
	}
	for _, session := range defaultControlWSHub.sessionsByUser(userID) {
		_ = session.sendTracked("active_network_enabled", "", enabled, &deps)
	}
}

func dtoPeerToWSPeer(peer dto.Peer) controlws.Peer {
	endpoints := make([]controlws.Endpoint, 0, len(peer.Endpoints))
	for _, endpoint := range peer.Endpoints {
		endpoints = append(endpoints, controlws.Endpoint{
			Type:      endpoint.Type,
			Address:   endpoint.Address,
			UpdatedAt: endpoint.UpdatedAt,
		})
	}
	return controlws.Peer{
		NodeID:        peer.NodeID,
		DeviceID:      peer.DeviceID,
		PublicKey:     peer.PublicKey,
		Status:        peer.Status,
		RelayAllowed:  peer.RelayAllowed,
		VirtualIPs:    append([]string(nil), peer.VirtualIPs...),
		Endpoints:     endpoints,
		AllowedRoutes: append([]string(nil), peer.AllowedRoutes...),
	}
}
