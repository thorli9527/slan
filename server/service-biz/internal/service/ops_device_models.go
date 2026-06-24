package service

type OpsManagedDeviceView struct {
	Device          DeviceView `json:"device"`
	OwnerEmail      string     `json:"ownerEmail"`
	GlobalIP        string     `json:"globalIp"`
	GlobalName      string     `json:"globalName"`
	HeartbeatOnline bool       `json:"heartbeatOnline"`
	NetworkEnabled  bool       `json:"networkEnabled"`
	DeviceEnabled   bool       `json:"deviceEnabled"`
	NetworkCount    int        `json:"networkCount"`
}
