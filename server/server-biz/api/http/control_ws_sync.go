package httpapi

import (
	"github.com/gin-gonic/gin"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

func registerControlWS(router *gin.Engine, path string, deps routerDeps) {
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
						broadcastPeerUpdateToSessions(event.NetworkID, event.SourceNodeID, event.Revision, *event.Peer)
					}
				case "peer_remove":
					broadcastPeerRemoveToSessions(event.NetworkID, event.SourceNodeID, event.Revision)
				case "peer_candidate":
					if event.Candidate != nil {
						sendPeerCandidateToNode(event.TargetNodeID, *event.Candidate)
					}
				case "connect_plan":
					if event.Plan != nil {
						sendConnectPlanToNode(event.NetworkID, event.SourceNodeID, event.TargetNodeID, *event.Plan)
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
