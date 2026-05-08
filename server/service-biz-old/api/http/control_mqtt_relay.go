package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/mqttauth"
)

var relayHeartbeatMQTTOnce sync.Once

func startRelayHeartbeatMQTT(deps routerDeps) {
	if !deps.Config.MQTT.Enabled || deps.ControlChannel == nil {
		return
	}
	relayHeartbeatMQTTOnce.Do(func() {
		go func() {
			for {
				credential := mqttauth.ServerSubscriberCredential(deps.Config.MQTT, time.Now())
				if credential == nil {
					return
				}
				err := mqttauth.Subscribe(
					context.Background(),
					deps.Config.MQTT,
					credential.ClientID+"-relay-heartbeat",
					credential.Username,
					credential.Password,
					mqttauth.RelayHeartbeatTopicFilter(deps.Config.MQTT),
					func(topic string, payload []byte) {
						if err := handleRelayHeartbeatMQTTMessage(deps, topic, payload); err != nil {
							log.Printf("mqtt relay heartbeat ignored topic=%s err=%v", topic, err)
						}
					},
				)
				if err != nil {
					log.Printf("mqtt relay heartbeat subscriber disconnected: %v", err)
				}
				time.Sleep(3 * time.Second)
			}
		}()
	})
}

func handleRelayHeartbeatMQTTMessage(deps routerDeps, topic string, payload []byte) error {
	heartbeat, ok, err := decodeRelayHeartbeatMQTTMessage(deps.Config.MQTT.TopicPrefix, topic, payload)
	if !ok {
		return nil
	}
	if err != nil {
		return err
	}
	return deps.ControlChannel.ReportRelayHeartbeat(heartbeat)
}

func decodeRelayHeartbeatMQTTMessage(topicPrefix, topic string, payload []byte) (controlmsg.RelayNodeHeartbeat, bool, error) {
	nodeID, ok := parseRelayHeartbeatTopic(topicPrefix, topic)
	if !ok {
		return controlmsg.RelayNodeHeartbeat{}, false, nil
	}
	var heartbeat controlmsg.RelayNodeHeartbeat
	if err := json.Unmarshal(payload, &heartbeat); err != nil {
		return controlmsg.RelayNodeHeartbeat{}, true, err
	}
	heartbeat.NodeID = nodeID
	return heartbeat, true, nil
}

func parseRelayHeartbeatTopic(topicPrefix, topic string) (string, bool) {
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
	if parts[len(prefixParts)] != "relays" || parts[len(prefixParts)+2] != "heartbeat" {
		return "", false
	}
	nodeID := parts[len(prefixParts)+1]
	return nodeID, nodeID != ""
}
