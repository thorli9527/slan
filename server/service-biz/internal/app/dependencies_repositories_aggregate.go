package app

type Repositories struct {
	Auth    AuthRepositories
	Device  DeviceRepositories
	Network NetworkRepositories
	Ops     OpsRepositories
	Wire    WireRepositories
	MQTT    MQTTRepositories
}
