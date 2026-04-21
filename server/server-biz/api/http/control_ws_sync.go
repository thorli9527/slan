package httpapi

import (
	"github.com/gin-gonic/gin"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

func registerControlWS(router *gin.Engine, path string, deps routerDeps) {
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
						sendPeerCandidateToNode(deps, event.TargetNodeID, *event.Candidate)
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

	router.GET(path, func(c *gin.Context) {
		websocket.Handler(func(conn *websocket.Conn) {
			serveControlWS(conn, deps)
		}).ServeHTTP(c.Writer, c.Request)
	})
}
