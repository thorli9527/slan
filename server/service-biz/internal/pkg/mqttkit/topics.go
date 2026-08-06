package mqttkit

import (
	"strconv"
	"strings"
)

func AllowTopicAccess(cfg Config, principal, deviceID, topic string, subscribe bool) bool {
	if !cfg.Enabled {
		return false
	}
	topic = trimTopic(topic)
	if topic == "" {
		return false
	}
	if principal == "server" {
		if subscribe {
			return topic == topicRoot(cfg)+"/devices/#" ||
				isControlUpTopic(cfg, topic) ||
				isControlAckTopic(cfg, topic) ||
				topic == topicRoot(cfg)+"/devices/+/heartbeat" ||
				topic == topicRoot(cfg)+"/devices/+/runtime-state" ||
				topic == topicRoot(cfg)+"/server/control/up"
		}
		return isControlDownTopic(cfg, topic) || isNetworkBroadcastTopic(cfg, topic)
	}
	if principal != "device" || strings.TrimSpace(deviceID) == "" {
		return false
	}
	devicePrefix := deviceTopicPrefix(cfg, deviceID)
	if subscribe {
		return topic == devicePrefix+"/control/down" || isNetworkBroadcastTopic(cfg, topic)
	}
	return topic == devicePrefix+"/control/up" ||
		topic == devicePrefix+"/control/ack" ||
		topic == devicePrefix+"/heartbeat" ||
		topic == devicePrefix+"/runtime" ||
		topic == devicePrefix+"/runtime-state" ||
		isControlDownTopic(cfg, topic) ||
		isNetworkBroadcastTopic(cfg, topic) ||
		isDeviceNetworkStateTopic(cfg, deviceID, topic)
}

func IsNetworkTopic(cfg Config, topic string) bool {
	return strings.HasPrefix(trimTopic(topic), topicRoot(cfg)+"/networks/")
}

func NetworkIDFromTopic(cfg Config, topic string) string {
	suffix := strings.TrimPrefix(trimTopic(topic), topicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func ParseDeviceUsername(cfg Config, username string) (string, string, int64, bool) {
	return parseDeviceUsername(cfg, username)
}

func ParseSystemUsername(cfg Config, username, expectedID string) (int64, bool) {
	return parseSystemUsername(cfg, username, expectedID)
}

func SystemUsername(cfg Config, id string, expiresAt int64) string {
	return usernamePrefix(cfg) + ":system:" + strings.TrimSpace(id) + ":" + strconv.FormatInt(expiresAt, 10)
}

func DeviceClientID(cfg Config, deviceID string) string {
	return deviceClientID(cfg, deviceID)
}

func deviceClientID(cfg Config, id string) string {
	return defaultString(cfg.ClientIDPrefix, "slan-device") + "-" + strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func parseDeviceUsername(cfg Config, username string) (string, string, int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 4 || parts[0] != usernamePrefix(cfg) || parts[1] == "system" {
		return "", "", 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	return parts[1], parts[2], expiresAt, err == nil && parts[1] != "" && parts[2] != ""
}

func parseSystemUsername(cfg Config, username, expectedID string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 4 || parts[0] != usernamePrefix(cfg) || parts[1] != "system" || parts[2] != expectedID {
		return 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	return expiresAt, err == nil
}

func usernamePrefix(cfg Config) string {
	return defaultString(cfg.UsernamePrefix, "device")
}

func topicRoot(cfg Config) string {
	root := trimTopic(cfg.TopicPrefix)
	if root == "" {
		return "slan"
	}
	return root
}

func deviceTopicPrefix(cfg Config, deviceID string) string {
	return topicRoot(cfg) + "/devices/" + strings.TrimSpace(deviceID)
}

func isControlUpTopic(cfg Config, topic string) bool {
	suffix := strings.TrimPrefix(topic, topicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "up"
}

func isControlDownTopic(cfg Config, topic string) bool {
	suffix := strings.TrimPrefix(topic, topicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "down"
}

func isControlAckTopic(cfg Config, topic string) bool {
	suffix := strings.TrimPrefix(topic, topicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "ack"
}

func isNetworkBroadcastTopic(cfg Config, topic string) bool {
	suffix := strings.TrimPrefix(topic, topicRoot(cfg)+"/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] == "networks" && parts[1] != "" && parts[2] == "broadcast"
}

func isDeviceNetworkStateTopic(cfg Config, deviceID, topic string) bool {
	suffix := strings.TrimPrefix(topic, topicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 4 && parts[0] != "" && parts[1] == "members" && parts[2] == deviceID && parts[3] == "state"
}

func trimTopic(topic string) string {
	return strings.Trim(strings.TrimSpace(topic), "/")
}
