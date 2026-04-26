package dto

// MQTTAuthCheckRequest is used by an MQTT auth provider to validate a generated SLAN credential.
type MQTTAuthCheckRequest struct {
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// MQTTAuthCheckResponse reports whether the MQTT auth provider should allow the connection.
type MQTTAuthCheckResponse struct {
	Allow     bool   `json:"allow"`
	DeviceID  string `json:"deviceId,omitempty"`
	Principal string `json:"principal,omitempty"`
}

type BifroMQAuthResponse struct {
	OK     *BifroMQAuthOK `json:"ok,omitempty"`
	Reject string         `json:"reject,omitempty"`
}

type BifroMQAuthOK struct {
	TenantID string            `json:"tenantId"`
	UserID   string            `json:"userId"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}
