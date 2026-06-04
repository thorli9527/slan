package biz

import "strings"

func mqttAllowTopicAccess(cfg MQTTConfig, principal, deviceID, topic string, subscribe bool) bool {
	topic = trimTopic(topic)
	if topic == "" {
		return false
	}
	if principal == "server" {
		if subscribe {
			return topic == mqttTopicRoot(cfg)+"/devices/#" ||
				isControlUpTopic(cfg, topic) ||
				topic == mqttServerControlUpTopic(cfg)
		}
		return isControlDownTopic(cfg, topic) || isNetworkBroadcastTopic(cfg, topic)
	}
	if principal != "device" || strings.TrimSpace(deviceID) == "" {
		return false
	}
	devicePrefix := mqttDeviceTopicPrefix(cfg, deviceID)
	if subscribe {
		return topic == devicePrefix+"/control/down" ||
			isNetworkBroadcastTopic(cfg, topic)
	}
	return topic == devicePrefix+"/control/up" ||
		topic == devicePrefix+"/control/ack" ||
		topic == devicePrefix+"/heartbeat" ||
		topic == devicePrefix+"/runtime" ||
		topic == devicePrefix+"/runtime-state" ||
		isDeviceNetworkStateTopic(cfg, deviceID, topic)
}

func mqttDeviceTopicPrefix(cfg MQTTConfig, deviceID string) string {
	return mqttTopicRoot(cfg) + "/devices/" + strings.TrimSpace(deviceID)
}

func mqttControlDownTopic(cfg MQTTConfig, deviceID string) string {
	return mqttDeviceTopicPrefix(cfg, deviceID) + "/control/down"
}

func mqttNetworkBroadcastTopic(cfg MQTTConfig, networkID string) string {
	return mqttTopicRoot(cfg) + "/networks/" + strings.TrimSpace(networkID) + "/broadcast"
}

func mqttNetworkMemberStateTopic(cfg MQTTConfig, networkID, deviceID string) string {
	return mqttTopicRoot(cfg) + "/networks/" + strings.TrimSpace(networkID) + "/members/" + strings.TrimSpace(deviceID) + "/state"
}

func mqttServerControlUpTopic(cfg MQTTConfig) string {
	return mqttTopicRoot(cfg) + "/server/control/up"
}

func isControlUpTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "up"
}

func isControlDownTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "down"
}

func isNetworkBroadcastTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] == "networks" && parts[1] != "" && parts[2] == "broadcast"
}

func isDeviceNetworkStateTopic(cfg MQTTConfig, deviceID, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 4 && parts[0] != "" && parts[1] == "members" && parts[2] == deviceID && parts[3] == "state"
}
func trimTopic(topic string) string {
	return strings.Trim(strings.TrimSpace(topic), "/")
}

func mqttTopicRoot(cfg MQTTConfig) string {
	root := trimTopic(cfg.TopicPrefix)
	if root == "" {
		return "slan"
	}
	return root
}
