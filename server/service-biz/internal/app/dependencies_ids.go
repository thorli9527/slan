package app

type IDGenerators struct {
	Device  DeviceIDGenerators
	Network NetworkIDGenerators
	Ops     OpsIDGenerators
	Wire    WireIDGenerators
}

type DeviceIDGenerators struct {
	NewDeviceID  func() string
	NewSessionID func(string) string
}

type NetworkIDGenerators struct {
	NewNetworkID func() string
	NewSessionID func(string) string
}

type OpsIDGenerators struct {
	NewSessionID    func(string) string
	NewOperatorID   func() string
	NewCustomerID   func() string
	NewServerNodeID func() string
}

type WireIDGenerators struct {
	NewRelayNodeID func() string
	NewPunchNodeID func() string
}
