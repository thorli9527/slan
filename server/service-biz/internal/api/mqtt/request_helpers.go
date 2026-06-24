package mqtt

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type authRequest struct {
	Values map[string]any
}

type checkRequest struct {
	Values map[string]any
}

type endpointReportRequest struct {
	Values map[string]any
}

type pathHealthReportRequest struct {
	Values map[string]any
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := bytesString(value); ok {
				return strings.TrimSpace(text)
			}
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func mapValue(values map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if nested, ok := value.(map[string]any); ok {
				return nested
			}
		}
	}
	return nil
}

func nestedStringValue(values map[string]any, directKeys []string, nestedKeys ...string) string {
	if value := stringValue(values, directKeys...); value != "" {
		return value
	}
	for _, parent := range []map[string]any{
		mapValue(values, "attrs", "attributes", "metadata"),
		mapValue(values, "client", "clientInfo", "client_info", "conn"),
		mapValue(values, "info", "packet"),
	} {
		if parent == nil {
			continue
		}
		if value := stringValue(parent, nestedKeys...); value != "" {
			return value
		}
		if nested := mapValue(parent, "attrs", "attributes", "metadata"); nested != nil {
			if value := stringValue(nested, nestedKeys...); value != "" {
				return value
			}
		}
	}
	return ""
}

func bytesString(value any) (string, bool) {
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

func headerValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func authRequestIsV5(values map[string]any) bool {
	if _, ok := values["responseInfo"]; ok {
		return true
	}
	if _, ok := values["userProps"]; ok {
		return true
	}
	version := stringValue(values, "version", "ver", "protocolVersion")
	return version == "5" || strings.EqualFold(version, "MQTT5")
}

func checkTopic(req map[string]any) (string, bool, bool) {
	if _, ok := req["conn"]; ok {
		return "", false, true
	}
	if sub, ok := req["sub"].(map[string]any); ok {
		return topicFromValues(sub), true, false
	}
	if pub, ok := req["pub"].(map[string]any); ok {
		return topicFromValues(pub), false, false
	}
	action := strings.ToLower(stringValue(req, "action", "operation", "type"))
	if strings.Contains(action, "connect") {
		return "", false, true
	}
	subscribe := strings.Contains(action, "sub")
	if topic := topicFromValues(req); topic != "" {
		return topic, subscribe, false
	}
	for _, key := range []string{"topics", "topicFilters", "topic_filters"} {
		if values, ok := req[key].([]any); ok && len(values) > 0 {
			return strings.TrimSpace(fmt.Sprint(values[0])), true, false
		}
	}
	return "", subscribe, false
}

func topicFromValues(values map[string]any) string {
	return nestedStringValue(values, []string{"topic", "topicFilter", "topic_filter"}, "topic", "topicFilter", "topic_filter")
}

func decodeAuthRequest(r *http.Request) (authRequest, error) {
	var values map[string]any
	if err := decodeJSONBody(r, &values); err != nil {
		return authRequest{}, err
	}
	return authRequest{Values: values}, nil
}

func decodeCheckRequest(r *http.Request) (checkRequest, error) {
	var values map[string]any
	if err := decodeJSONBody(r, &values); err != nil {
		return checkRequest{}, err
	}
	return checkRequest{Values: values}, nil
}

func decodeEndpointReportRequest(r *http.Request) (endpointReportRequest, error) {
	var values map[string]any
	if err := decodeJSONBody(r, &values); err != nil {
		return endpointReportRequest{}, err
	}
	return endpointReportRequest{Values: values}, nil
}

func decodePathHealthReportRequest(r *http.Request) (pathHealthReportRequest, error) {
	var values map[string]any
	if err := decodeJSONBody(r, &values); err != nil {
		return pathHealthReportRequest{}, err
	}
	return pathHealthReportRequest{Values: values}, nil
}

func (r authRequest) input() servicepkg.MQTTAuthInput {
	return servicepkg.MQTTAuthInput{
		ClientID: nestedStringValue(r.Values, []string{"clientId", "client_id"}, "clientId", "client_id"),
		Username: nestedStringValue(r.Values, []string{"username", "userName"}, "username", "userName"),
		Password: nestedStringValue(r.Values, []string{"password"}, "password"),
		IsV5:     authRequestIsV5(r.Values),
	}
}

func (r checkRequest) input(httpReq *http.Request) servicepkg.MQTTCheckInput {
	principal := nestedStringValue(r.Values, []string{"principal"}, "principal")
	deviceID := nestedStringValue(r.Values, []string{"deviceId", "device_id"}, "deviceId", "device_id")
	userID := nestedStringValue(r.Values, []string{"userId", "user_id"}, "userId", "user_id")
	if userID == "" {
		userID = headerValue(httpReq, "user_id", "user-id", "userid", "userId")
	}
	if deviceID == "" {
		deviceID = headerValue(httpReq, "device_id", "device-id", "deviceId")
	}
	topic, subscribe, connect := checkTopic(r.Values)
	return servicepkg.MQTTCheckInput{
		Principal: principal,
		DeviceID:  deviceID,
		UserID:    userID,
		Topic:     topic,
		Subscribe: subscribe,
		Connect:   connect,
	}
}

func (r endpointReportRequest) input() servicepkg.MQTTEndpointReportInput {
	payload := mapValue(r.Values, "payload")
	if payload == nil {
		payload = r.Values
	}
	deviceID := nestedStringValue(payload, []string{"deviceId", "device_id"}, "deviceId", "device_id")
	nodeID := nestedStringValue(payload, []string{"nodeId", "node_id"}, "nodeId", "node_id")
	if deviceID == "" && strings.HasPrefix(nodeID, "node-") {
		deviceID = strings.TrimPrefix(nodeID, "node-")
	}
	endpoints := make([]servicepkg.DeviceEndpointView, 0)
	if values, ok := payload["endpoints"].([]any); ok {
		for _, item := range values {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			endpoints = append(endpoints, servicepkg.DeviceEndpointView{
				Type:      stringValue(entry, "type", "kind"),
				Address:   stringValue(entry, "address", "endpoint"),
				UpdatedAt: int64Value(entry, "updatedAt", "updated_at"),
			})
		}
	}
	return servicepkg.MQTTEndpointReportInput{
		NetworkID: stringValue(payload, "networkId", "network_id"),
		DeviceID:  deviceID,
		NodeID:    nodeID,
		NATType:   stringValue(payload, "natType", "nat_type"),
		Endpoints: endpoints,
	}
}

func (r pathHealthReportRequest) input() servicepkg.MQTTPathHealthReportInput {
	payload := mapValue(r.Values, "payload")
	if payload == nil {
		payload = r.Values
	}
	deviceID := nestedStringValue(payload, []string{"deviceId", "device_id"}, "deviceId", "device_id")
	return servicepkg.MQTTPathHealthReportInput{
		NetworkID:         stringValue(payload, "networkId", "network_id"),
		DeviceID:          deviceID,
		PeerNodeID:        stringValue(payload, "peerNodeId", "peer_node_id"),
		PathType:          stringValue(payload, "pathType", "path_type"),
		ActivePath:        stringValue(payload, "activePath", "active_path"),
		RelayTransport:    stringValue(payload, "relayTransport", "relay_transport"),
		Endpoint:          stringValue(payload, "endpoint"),
		DerpNodeID:        stringValue(payload, "derpNodeId", "derp_node_id"),
		ObservedRttMs:     int64Value(payload, "observedRttMs", "observed_rtt_ms"),
		PacketLossPpm:     int64Value(payload, "packetLossPpm", "packet_loss_ppm"),
		PathScore:         int64Value(payload, "pathScore", "path_score"),
		RelayMtu:          int(int64Value(payload, "relayMtu", "relay_mtu")),
		MaxFramePayload:   int(int64Value(payload, "maxFramePayload", "max_frame_payload")),
		TicketExpiresAt:   stringValue(payload, "ticketExpiresAt", "ticket_expires_at"),
		TicketExpiresInMs: int64Value(payload, "ticketExpiresInMs", "ticket_expires_in_ms"),
		TicketRenewDue:    boolValue(payload, "ticketRenewDue", "ticket_renew_due"),
		PathDowngrades:    int64Value(payload, "pathDowngrades", "path_downgrades"),
		PathUpgrades:      int64Value(payload, "pathUpgrades", "path_upgrades"),
		LastPathChange:    stringValue(payload, "lastPathChange", "last_path_change"),
		SampledAtMs:       int64Value(payload, "sampledAtMs", "sampled_at_ms"),
	}
}

func decodeJSONBody(r *http.Request, out any) error {
	decoder := jsonDecoder(r)
	return decoder(out)
}

func int64Value(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int64:
			return typed
		case int:
			return int64(typed)
		case float64:
			return int64(typed)
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			text := strings.TrimSpace(strings.ToLower(typed))
			return text == "true" || text == "1" || text == "yes"
		case float64:
			return typed != 0
		case int:
			return typed != 0
		}
	}
	return false
}
