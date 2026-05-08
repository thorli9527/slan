package dto

type MQTTCredentialAuthResult struct {
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
