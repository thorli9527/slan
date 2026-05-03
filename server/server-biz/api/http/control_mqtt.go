package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

var controlmsgOnce sync.Once

type controlmsgEnvelope struct {
	Type         string          `json:"type"`
	RequestID    string          `json:"requestId,omitempty"`
	MessageID    string          `json:"messageId,omitempty"`
	SourceNodeID string          `json:"sourceNodeId,omitempty"`
	NetworkID    string          `json:"networkId,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

func startControlMQTT(deps routerDeps) {
	if !deps.Config.MQTT.Enabled || deps.ControlChannel == nil {
		return
	}
	controlmsgOnce.Do(func() {
		go func() {
			for {
				credential := mqttauth.ServerSubscriberCredential(deps.Config.MQTT, time.Now())
				if credential == nil {
					return
				}
				err := mqttauth.Subscribe(
					context.Background(),
					deps.Config.MQTT,
					credential.ClientID,
					credential.Username,
					credential.Password,
					mqttauth.ControlUpTopicFilter(deps.Config.MQTT),
					func(topic string, payload []byte) {
						if err := handleControlMQTTMessage(deps, topic, payload); err != nil {
							log.Printf("mqtt control message ignored topic=%s err=%v", topic, err)
						}
					},
				)
				if err != nil {
					log.Printf("mqtt control subscriber disconnected: %v", err)
				}
				time.Sleep(3 * time.Second)
			}
		}()
	})
}

func handleControlMQTTMessage(deps routerDeps, topic string, payload []byte) error {
	deviceID, ok := parseControlUpTopic(deps.Config.MQTT.TopicPrefix, topic)
	if !ok {
		return service.ErrInvalidArgument
	}
	var env controlmsgEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	session := controlSession{
		deviceID:  deviceID,
		nodeID:    strings.TrimSpace(env.SourceNodeID),
		networkID: strings.TrimSpace(env.NetworkID),
	}
	return handleControlMQTTEnvelope(deps, deviceID, &session, env)
}

func handleControlMQTTEnvelope(deps routerDeps, deviceID string, session *controlSession, env controlmsgEnvelope) error {
	switch env.Type {
	case "node_hello":
		var hello controlmsg.NodeHello
		if err := json.Unmarshal(env.Payload, &hello); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		if hello.DeviceID != "" && hello.DeviceID != deviceID {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrForbidden)
		}
		ack, networkMap, err := deps.ControlChannel.Handshake(hello)
		if err != nil {
			metricAdd("handshake_error_total", 1)
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		metricAdd("handshake_success_total", 1)
		metricAddByType(controlmsgMessageTypeMetrics, "node_hello_ack", 1)
		*session = controlSession{
			userID:    util.FirstNonEmpty(hello.UserID, networkMap.SelfUserID),
			deviceID:  deviceID,
			nodeID:    hello.NodeID,
			networkID: hello.NetworkID,
		}
		if err := publishControlMQTTEnvelope(deps, deviceID, "node_hello_ack", env.RequestID, ack); err != nil {
			return err
		}
		if err := publishControlMQTTEnvelope(deps, deviceID, "network_map_response", env.RequestID, controlmsg.NetworkMapResponse{
			Map: dtoNetworkMapToControl(networkMap),
		}); err != nil {
			return err
		}
		fanoutPeerUpdate(deps, *session)
		fanoutConnectPlans(deps, *session)
	case "ping":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var ping controlmsg.Ping
		if err := json.Unmarshal(env.Payload, &ping); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		return publishControlMQTTEnvelope(deps, deviceID, "pong", env.RequestID, controlmsg.Pong{Timestamp: ping.Timestamp})
	case "network_map_request":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var req controlmsg.NetworkMapRequest
		if err := json.Unmarshal(env.Payload, &req); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		networkID := util.FirstNonEmpty(req.NetworkID, session.networkID)
		networkMap, err := deps.ControlChannel.NetworkMap(session.userID, session.nodeID, networkID)
		if err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		return publishControlMQTTEnvelope(deps, deviceID, "network_map_response", env.RequestID, controlmsg.NetworkMapResponse{
			Map: dtoNetworkMapToControl(networkMap),
		})
	case "endpoint_report":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var report controlmsg.EndpointReport
		if err := json.Unmarshal(env.Payload, &report); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		report.NodeID = util.FirstNonEmpty(report.NodeID, session.nodeID)
		report.NetworkID = util.FirstNonEmpty(report.NetworkID, session.networkID)
		networkMap, err := deps.ControlChannel.ReportEndpoints(session.userID, report)
		if err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		if err := publishControlMQTTEnvelope(deps, deviceID, "network_map_response", env.RequestID, controlmsg.NetworkMapResponse{
			Map: dtoNetworkMapToControl(networkMap),
		}); err != nil {
			return err
		}
		fanoutPeerUpdate(deps, *session)
		fanoutConnectPlans(deps, *session)
	case "connection_state":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var state controlmsg.ConnectionState
		if err := json.Unmarshal(env.Payload, &state); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		state.NetworkID = util.FirstNonEmpty(state.NetworkID, session.networkID)
		if err := deps.ControlChannel.ReportConnectionState(session.userID, session.nodeID, state); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		metricRecordConnectionState(session, state)
		if state.State == "connected" {
			resetConnectPlanRetry(deps, session.networkID, session.nodeID, state.PeerNodeID)
		}
		if state.State == "failed" || state.State == "closed" {
			if allowConnectPlanRetry(deps, session.networkID, session.nodeID, state.PeerNodeID) {
				fanoutConnectPlanPair(deps, *session, state.PeerNodeID)
			}
		}
	case "path_health_report":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var report controlmsg.PathHealthReport
		if err := json.Unmarshal(env.Payload, &report); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		report.NetworkID = util.FirstNonEmpty(report.NetworkID, session.networkID)
		if err := deps.ControlChannel.ReportPathHealth(session.userID, session.nodeID, report); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		metricRecordPathHealth(session, report)
	case "relay_policy_report":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var report controlmsg.RelayPolicyReport
		if err := json.Unmarshal(env.Payload, &report); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		report.NetworkID = util.FirstNonEmpty(report.NetworkID, session.networkID)
		report.DeviceID = util.FirstNonEmpty(report.DeviceID, deviceID)
		if err := deps.ControlChannel.ReportRelayPolicy(session.userID, session.nodeID, report); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
	case "disconnect_notice":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var notice controlmsg.DisconnectNotice
		if err := json.Unmarshal(env.Payload, &notice); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		notice.NetworkID = util.FirstNonEmpty(notice.NetworkID, session.networkID)
		if err := deps.ControlChannel.Disconnect(session.userID, session.nodeID, notice); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		fanoutPeerRemove(deps, session.networkID, session.nodeID)
	case "peer_candidate":
		if err := hydrateMQTTSession(deps, session); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, err)
		}
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
		var candidate controlmsg.PeerCandidate
		if err := json.Unmarshal(env.Payload, &candidate); err != nil {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		if candidate.PeerNodeID == "" || candidate.Endpoint == "" || candidate.CandidateType == "" {
			return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
		}
		forwardPeerCandidate(deps, *session, candidate)
	default:
		return publishControlMQTTError(deps, deviceID, env.RequestID, service.ErrInvalidArgument)
	}
	return nil
}

func hydrateMQTTSession(deps routerDeps, session *controlSession) error {
	if session.nodeID == "" {
		if strings.TrimSpace(session.deviceID) == "" {
			return service.ErrUnauthorized
		}
		current, err := deps.ControlChannel.LatestSessionByDevice(session.deviceID)
		if err != nil {
			return err
		}
		session.userID = current.UserID
		session.deviceID = current.DeviceID
		session.nodeID = current.NodeID
		if session.networkID == "" {
			session.networkID = current.NetworkID
		}
		return nil
	}
	if session.networkID == "" {
		return service.ErrUnauthorized
	}
	sessions, err := deps.ControlChannel.ActiveSessions(session.networkID, "")
	if err != nil {
		return err
	}
	for _, current := range sessions {
		if current.NodeID == session.nodeID {
			session.userID = current.UserID
			session.deviceID = current.DeviceID
			return nil
		}
	}
	return service.ErrUnauthorized
}

func publishControlMQTTError(deps routerDeps, deviceID, requestID string, err error) error {
	code := controlErrorCode(err)
	metricAddByType(controlmsgErrorCodeMetrics, code, 1)
	return publishControlMQTTEnvelope(deps, deviceID, "error", requestID, controlmsg.ErrorMessage{
		Code:    code,
		Message: err.Error(),
	})
}

func publishControlMQTTEnvelope(deps routerDeps, deviceID, msgType, requestID string, payload any) error {
	credential := mqttauth.ServerSubscriberCredential(deps.Config.MQTT, time.Now())
	if credential == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deps.Config.MQTT.PublishTimeoutMilliseconds)*time.Millisecond)
	defer cancel()
	return mqttauth.PublishJSONWithOptions(ctx, deps.Config.MQTT, credential.ClientID, credential.Username, credential.Password, mqttauth.ControlDownTopic(deps.Config.MQTT, deviceID), controlmsg.Envelope{
		Type:      msgType,
		RequestID: requestID,
		MessageID: controlMQTTMessageID(msgType),
		Payload:   payload,
	}, mqttauth.PublishOptions{
		QoS: controlMQTTMessageQoS(msgType),
	})
}

func controlMQTTMessageID(msgType string) string {
	if controlMQTTMessageQoS(msgType) == mqttauth.PublishQoSAtMostOnce {
		return ""
	}
	return util.NewID("msg")
}

func controlMQTTMessageQoS(msgType string) byte {
	switch msgType {
	case "pong":
		// Heartbeat responses are intentionally lossy and should not pay the
		// QoS2 round-trip cost.
		return mqttauth.PublishQoSAtMostOnce
	default:
		// Control/config/task responses must survive transient client or broker
		// interruptions.
		return mqttauth.PublishQoSExactlyOnce
	}
}

func parseControlUpTopic(topicPrefix, topic string) (string, bool) {
	prefixParts := splitTopic(topicPrefix)
	parts := splitTopic(topic)
	if len(parts) != len(prefixParts)+3 {
		return "", false
	}
	for index, prefixPart := range prefixParts {
		if parts[index] != prefixPart {
			return "", false
		}
	}
	deviceID := parts[len(prefixParts)]
	if deviceID == "" || parts[len(prefixParts)+1] != "control" || parts[len(prefixParts)+2] != "up" {
		return "", false
	}
	return deviceID, true
}

func splitTopic(topic string) []string {
	trimmed := strings.Trim(strings.TrimSpace(topic), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
