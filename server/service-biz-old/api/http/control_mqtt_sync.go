package httpapi

import controlmsg "github.com/slan/server/server-biz/internal/controlmsg"

func startControlSync(deps routerDeps) {
	controlmsgDeliveryRetryOnce.Do(func() {
		startcontrolmsgDeliveryRetryLoop(deps)
	})
	controlmsgSyncOnce.Do(func() {
		if deps.ControlSync != nil {
			_ = deps.ControlSync.Subscribe(func(event controlmsg.ControlSyncEvent) {
				metricAdd("sync_event_received_total", 1)
				switch event.Type {
				case "peer_update":
					if event.Peer != nil {
						broadcastPeerUpdateToSessions(deps, event.NetworkID, event.SourceNodeID, event.Revision, *event.Peer)
					}
				case "peer_remove":
					broadcastPeerRemoveToSessions(deps, event.NetworkID, event.SourceNodeID, event.Revision)
				case "peer_candidate":
					if event.Candidate != nil {
						sendPeerCandidateToNode(deps, event.NetworkID, event.TargetNodeID, *event.Candidate)
					}
				case "connect_plan":
					if event.Plan != nil {
						sendConnectPlanToNode(deps, event.NetworkID, event.SourceNodeID, event.TargetNodeID, *event.Plan)
					}
				case "network_restart_required":
					if event.Restart != nil {
						broadcastNetworkRestartRequired(deps, event.NetworkID, *event.Restart)
					}
				case "device_ip_reassigned":
					if event.DeviceIP != nil {
						broadcastDeviceIPReassigned(deps, event.NetworkID, *event.DeviceIP)
					}
				case controlmsg.DeviceNetworkDisabledEvent:
					if event.DeviceDisabled != nil {
						broadcastDeviceNetworkDisabled(deps, event.NetworkID, *event.DeviceDisabled)
					}
					if event.DevicePresence != nil {
						publishNetworkBroadcastMQTTEnvelope(deps, event.Type, *event.DevicePresence)
					}
				case "active_network_enabled":
					if event.ActiveNetwork != nil {
						broadcastActiveNetworkEnabled(deps, event.TargetUserID, *event.ActiveNetwork)
					}
				case controlmsg.DeviceNetworkEnabledEvent, controlmsg.DeviceNetworkExpiredEvent:
					if event.DevicePresence != nil {
						publishNetworkBroadcastMQTTEnvelope(deps, event.Type, *event.DevicePresence)
					}
				}
			})
		}
	})
}
