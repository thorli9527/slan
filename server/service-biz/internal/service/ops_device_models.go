package service

type OpsManagedDeviceView struct {
	Device          DeviceView `json:"device"`
	GlobalIP        string     `json:"globalIp"`
	GlobalName      string     `json:"globalName"`
	HeartbeatOnline bool       `json:"heartbeatOnline"`
	NetworkEnabled  bool       `json:"networkEnabled"`
	DeviceEnabled   bool       `json:"deviceEnabled"`
	NetworkCount    int        `json:"networkCount"`
}
