package service

type UpdateDeviceInput struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Alias    string `json:"alias"`
	Status   string `json:"status"`
}
