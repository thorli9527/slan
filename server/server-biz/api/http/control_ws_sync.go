package httpapi

import controlws "github.com/slan/server/server-biz/internal/ws"

func startControlSync(deps routerDeps) {
	controlWSDeliveryRetryOnce.Do(func() {
		startControlWSDeliveryRetryLoop(deps)
	})
	controlWSSyncOnce.Do(func() {
		if deps.ControlSync != nil {
			_ = deps.ControlSync.Subscribe(func(event controlws.ControlSyncEvent) {
				if event.InstanceID == controlWSInstanceID {
					return
				}
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
				case "active_network_enabled":
					if event.ActiveNetwork != nil {
						broadcastActiveNetworkEnabled(deps, event.TargetUserID, *event.ActiveNetwork)
					}
				}
			})
		}
	})
}
