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

type valueReader struct {
	values map[string]any
}

func newValueReader(values map[string]any) valueReader {
	return valueReader{values: values}
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

func (r valueReader) string(keys ...string) string {
	return stringValue(r.values, keys...)
}

func (r valueReader) nestedString(directKey string, nestedKeys ...string) string {
	return nestedStringValue(r.values, []string{directKey}, nestedKeys...)
}

func (r valueReader) int64(keys ...string) int64 {
	return int64Value(r.values, keys...)
}

func (r valueReader) bool(keys ...string) bool {
	return boolValue(r.values, keys...)
}

func nestedStringValue(values map[string]any, directKeys []string, nestedKeys ...string) string {
	if value := stringValue(values, directKeys...); value != "" {
		return value
	}
	for _, parent := range []map[string]any{
		mapValue(values, "attrs", "attributes", "metadata"),
		mapValue(values, "client", "clientInfo", "conn"),
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

func payloadValues(values map[string]any) map[string]any {
	if payload := mapValue(values, "payload"); payload != nil {
		return payload
	}
	return values
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
	version := stringValue(values, "version")
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
	for _, key := range []string{"topics", "topicFilters"} {
		if values, ok := req[key].([]any); ok && len(values) > 0 {
			return strings.TrimSpace(fmt.Sprint(values[0])), true, false
		}
	}
	return "", subscribe, false
}

func topicFromValues(values map[string]any) string {
	reader := newValueReader(values)
	if topic := reader.string("topic", "topicFilter"); topic != "" {
		return topic
	}
	if topic := reader.nestedString("topic", "topic", "topicFilter"); topic != "" {
		return topic
	}
	return reader.nestedString("topicFilter", "topic", "topicFilter")
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
	reader := newValueReader(r.Values)
	return servicepkg.MQTTAuthInput{
		ClientID: reader.nestedString("clientId", "clientId"),
		Username: reader.nestedString("username", "username"),
		Password: reader.nestedString("password", "password"),
		IsV5:     authRequestIsV5(r.Values),
	}
}

func (r checkRequest) input(httpReq *http.Request) servicepkg.MQTTCheckInput {
	reader := newValueReader(r.Values)
	principal := reader.nestedString("principal", "principal")
	deviceID := reader.nestedString("deviceId", "deviceId", "deviceID")
	userID := reader.nestedString("userId", "userId", "userID")
	clientID := reader.nestedString("clientId", "clientId", "clientID")
	username := reader.nestedString("username", "username", "userName")
	if userID == "" {
		userID = headerValue(httpReq, "userId", "userID", "user_id", "User_id")
	}
	if deviceID == "" {
		deviceID = headerValue(httpReq, "deviceId", "deviceID", "device_id", "Device_id")
	}
	if clientID == "" {
		clientID = headerValue(httpReq, "clientId", "clientID", "client_id", "Client_id")
	}
	if username == "" {
		username = headerValue(httpReq, "username", "userName", "user_name", "User_name")
	}
	topic, subscribe, connect := checkTopic(r.Values)
	return servicepkg.MQTTCheckInput{
		Principal: principal,
		DeviceID:  deviceID,
		UserID:    userID,
		ClientID:  clientID,
		Username:  username,
		Topic:     topic,
		Subscribe: subscribe,
		Connect:   connect,
	}
}

func (r endpointReportRequest) input() servicepkg.MQTTEndpointReportInput {
	reader := newValueReader(payloadValues(r.Values))
	deviceID := reader.nestedString("deviceId", "deviceId")
	nodeID := reader.nestedString("nodeId", "nodeId")
	if deviceID == "" && strings.HasPrefix(nodeID, "node-") {
		deviceID = strings.TrimPrefix(nodeID, "node-")
	}
	endpoints := make([]servicepkg.DeviceEndpointView, 0)
	if values, ok := reader.values["endpoints"].([]any); ok {
		for _, item := range values {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			entryReader := newValueReader(entry)
			endpoints = append(endpoints, servicepkg.DeviceEndpointView{
				Type:      entryReader.string("type", "kind"),
				Address:   entryReader.string("address", "endpoint"),
				UpdatedAt: entryReader.int64("updatedAt"),
			})
		}
	}
	return servicepkg.MQTTEndpointReportInput{
		NetworkID: reader.string("networkId"),
		DeviceID:  deviceID,
		NodeID:    nodeID,
		NATType:   reader.string("natType"),
		Endpoints: endpoints,
	}
}

func (r pathHealthReportRequest) input() servicepkg.MQTTPathHealthReportInput {
	reader := newValueReader(payloadValues(r.Values))
	deviceID := reader.nestedString("deviceId", "deviceId")
	return servicepkg.MQTTPathHealthReportInput{
		NetworkID:         reader.string("networkId"),
		DeviceID:          deviceID,
		PeerNodeID:        reader.string("peerNodeId"),
		PathType:          reader.string("pathType"),
		ActivePath:        reader.string("activePath"),
		RelayTransport:    reader.string("relayTransport"),
		Endpoint:          reader.string("endpoint"),
		DerpNodeID:        reader.string("derpNodeId"),
		ObservedRttMs:     reader.int64("observedRttMs"),
		PacketLossPpm:     reader.int64("packetLossPpm"),
		PathScore:         reader.int64("pathScore"),
		SignalScore:       int(reader.int64("signalScore")),
		SignalQuality:     reader.string("signalQuality"),
		RelayMtu:          int(reader.int64("relayMtu")),
		MaxFramePayload:   int(reader.int64("maxFramePayload")),
		TicketExpiresAt:   reader.string("ticketExpiresAt"),
		TicketExpiresInMs: reader.int64("ticketExpiresInMs"),
		TicketRenewDue:    reader.bool("ticketRenewDue"),
		PathDowngrades:    reader.int64("pathDowngrades"),
		PathUpgrades:      reader.int64("pathUpgrades"),
		LastPathChange:    reader.string("lastPathChange"),
		SampledAtMs:       reader.int64("sampledAtMs"),
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
