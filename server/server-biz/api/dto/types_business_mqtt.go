package dto

// MQTTAuthCheckRequest is used by an MQTT auth manager to validate a generated SLAN credential.
type MQTTAuthCheckRequest struct {
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// MQTTAuthCheckResponse reports whether the MQTT auth manager should allow the connection.
type MQTTAuthCheckResponse struct {
	Allow     bool   `json:"allow"`
	DeviceID  string `json:"deviceId,omitempty"`
	Principal string `json:"principal,omitempty"`
}
