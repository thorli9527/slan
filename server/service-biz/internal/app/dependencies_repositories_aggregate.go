package app

type Repositories struct {
	Device  DeviceRepositories
	Network NetworkRepositories
	Ops     OpsRepositories
	Wire    WireRepositories
	MQTT    MQTTRepositories
}
