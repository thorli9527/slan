package app

type IDGenerators struct {
	Auth     AuthIDGenerators
	Device   DeviceIDGenerators
	Network  NetworkIDGenerators
	Ops      OpsIDGenerators
	Wire     WireIDGenerators
	Download DownloadIDGenerators
}

type AuthIDGenerators struct {
	NewUserID    func() string
	NewSessionID func(string) string
}

type DeviceIDGenerators struct {
	NewDeviceID  func() string
	NewSessionID func(string) string
}

type NetworkIDGenerators struct {
	NewNetworkID func() string
	NewInviteID  func() string
	NewSessionID func(string) string
}

type OpsIDGenerators struct {
	NewSessionID  func(string) string
	NewOperatorID func() string
	NewProductID  func() string
	NewOrderID    func() string
}

type WireIDGenerators struct {
	NewRelayNodeID func() string
	NewPunchNodeID func() string
}

type DownloadIDGenerators struct {
	NewDownloadID func() string
}
