package service

type UpdateDeviceInput struct {
	DeviceID  string `json:"deviceId"`
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	VirtualIP string `json:"virtualIp"`
	Status    string `json:"status"`
}

type CreateOpsDeviceInput struct {
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	Platform  string `json:"platform"`
	OSName    string `json:"osName"`
	OSVersion string `json:"osVersion"`
	PublicKey string `json:"publicKey"`
}
