package biz

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func mqttStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := mqttBytesString(value); ok {
				return strings.TrimSpace(text)
			}
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func mqttBytesString(value any) (string, bool) {
	items, ok := value.([]any)
	if !ok {
		return "", false
	}
	out := make([]byte, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case float64:
			if typed < 0 || typed > 255 {
				return "", false
			}
			out = append(out, byte(typed))
		case int:
			if typed < 0 || typed > 255 {
				return "", false
			}
			out = append(out, byte(typed))
		default:
			return "", false
		}
	}
	return string(out), true
}

func mqttRequestKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		key = redactedFieldName(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mqttHeaderValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func mqttAuthRequestIsV5(values map[string]any) bool {
	if _, ok := values["responseInfo"]; ok {
		return true
	}
	if _, ok := values["userProps"]; ok {
		return true
	}
	if version := mqttStringValue(values, "version", "ver", "protocolVersion"); version == "5" || strings.EqualFold(version, "MQTT5") {
		return true
	}
	return false
}

func mqttCheckTopic(req map[string]any) (string, bool, bool) {
	if _, ok := req["conn"]; ok {
		return "", false, true
	}
	if sub, ok := req["sub"].(map[string]any); ok {
		return mqttTopicFromValues(sub), true, false
	}
	if pub, ok := req["pub"].(map[string]any); ok {
		return mqttTopicFromValues(pub), false, false
	}
	action := strings.ToLower(mqttStringValue(req, "action", "operation", "type"))
	if strings.Contains(action, "connect") {
		return "", false, true
	}
	subscribe := strings.Contains(action, "sub")
	if topic := mqttTopicFromValues(req); topic != "" {
		return topic, subscribe, false
	}
	for _, key := range []string{"topics", "topicFilters", "topic_filters"} {
		if values, ok := req[key].([]any); ok && len(values) > 0 {
			return strings.TrimSpace(fmt.Sprint(values[0])), true, false
		}
	}
	return "", subscribe, false
}

func mqttTopicFromValues(values map[string]any) string {
	return mqttStringValue(values, "topic", "topicFilter", "topic_filter")
}

func isNetworkTopic(cfg MQTTConfig, topic string) bool {
	return strings.HasPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/networks/")
}

func networkIDFromMQTTTopic(cfg MQTTConfig, topic string) string {
	suffix := strings.TrimPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
