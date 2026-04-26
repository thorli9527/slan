package httpapi

import (
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
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
	peerSessions, err := deps.ControlChannel.ActiveSessions(session.networkID, session.nodeID)
	if err != nil {
		return
	}
	for _, peerSession := range peerSessions {
		planForSource, err := deps.ControlChannel.ConnectPlan(session.userID, session.nodeID, session.networkID, peerSession.NodeID)
		if err == nil {
			sendConnectPlanToNode(deps, session.networkID, session.nodeID, session.nodeID, planForSource)
			publishConnectPlan(deps, session.networkID, session.nodeID, session.nodeID, planForSource)
		}
		planForPeer, err := deps.ControlChannel.ConnectPlan(peerSession.UserID, peerSession.NodeID, peerSession.NetworkID, session.nodeID)
		if err == nil {
			sendConnectPlanToNode(deps, peerSession.NetworkID, peerSession.NodeID, peerSession.NodeID, planForPeer)
			publishConnectPlan(deps, peerSession.NetworkID, peerSession.NodeID, peerSession.NodeID, planForPeer)
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

func sendPeerCandidateToNode(deps routerDeps, networkID, targetNodeID string, candidate controlws.PeerCandidate) {
	if targetNodeID == "" {
		return
	}
	session, ok := mqttSessionByNode(deps, networkID, targetNodeID)
	if !ok {
		return
	}
	_ = publishControlMQTTEnvelope(deps, session.DeviceID, "peer_candidate", "", candidate)
}

func sendConnectPlanToNode(deps routerDeps, networkID, sourceNodeID, targetNodeID string, plan controlws.ConnectPlan) {
	if targetNodeID == "" {
		return
	}
	session, ok := mqttSessionByNode(deps, networkID, targetNodeID)
	if !ok {
		return
	}
	metricRecordConnectPlan(networkID, sourceNodeID, targetNodeID, plan)
	_ = publishControlMQTTEnvelope(deps, session.DeviceID, "connect_plan", "", plan)
}

func broadcastPeerUpdateToSessions(deps routerDeps, networkID, sourceNodeID string, revision uint64, peer controlws.Peer) {
	for _, peerSession := range mqttSessionsInNetwork(deps, networkID, sourceNodeID) {
		_ = publishControlMQTTEnvelope(deps, peerSession.DeviceID, "peer_update", "", controlws.PeerUpdate{
			NetworkID: networkID,
			Revision:  revision,
			Peer:      peer,
		})
	}
}

func broadcastPeerRemoveToSessions(deps routerDeps, networkID, sourceNodeID string, revision uint64) {
	if sourceNodeID == "" || networkID == "" {
		return
	}
	for _, peerSession := range mqttSessionsInNetwork(deps, networkID, sourceNodeID) {
		_ = publishControlMQTTEnvelope(deps, peerSession.DeviceID, "peer_remove", "", controlws.PeerRemove{
			NetworkID:  networkID,
			Revision:   revision,
			PeerNodeID: sourceNodeID,
		})
	}
}

func broadcastNetworkRestartRequired(deps routerDeps, networkID string, restart controlws.NetworkRestartRequired) {
	for _, peerSession := range mqttSessionsInNetwork(deps, networkID, "") {
		_ = publishControlMQTTEnvelope(deps, peerSession.DeviceID, "network_restart_required", "", restart)
	}
}

func broadcastDeviceIPReassigned(deps routerDeps, networkID string, deviceIP controlws.DeviceIPReassigned) {
	for _, peerSession := range mqttSessionsInNetwork(deps, networkID, "") {
		_ = publishControlMQTTEnvelope(deps, peerSession.DeviceID, "device_ip_reassigned", "", deviceIP)
	}
}

func broadcastActiveNetworkEnabled(deps routerDeps, userID string, enabled controlws.ActiveNetworkEnabled) {
	if userID == "" {
		return
	}
	for _, session := range mqttSessionsInNetwork(deps, enabled.NetworkID, "") {
		if session.UserID == userID {
			_ = publishControlMQTTEnvelope(deps, session.DeviceID, "active_network_enabled", "", enabled)
		}
	}
}

func mqttSessionsInNetwork(deps routerDeps, networkID, excludeNodeID string) []service.ControlSession {
	if deps.ControlChannel == nil {
		return nil
	}
	sessions, err := deps.ControlChannel.ActiveSessions(networkID, excludeNodeID)
	if err != nil {
		return nil
	}
	return sessions
}

func mqttSessionByNode(deps routerDeps, networkID, nodeID string) (service.ControlSession, bool) {
	for _, session := range mqttSessionsInNetwork(deps, networkID, "") {
		if session.NodeID == nodeID {
			return session, true
		}
	}
	return service.ControlSession{}, false
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
