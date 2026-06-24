package app

import "github.com/slan/service-biz/internal/pkg/mqttkit"

func (d UseCaseDependencies) authRepositories() AuthRepositories {
	return d.Repositories.Auth
}

func (d UseCaseDependencies) authIDs() AuthIDGenerators {
	return d.IDs.Auth
}

func (d UseCaseDependencies) deviceRepositories() DeviceRepositories {
	return d.Repositories.Device
}

func (d UseCaseDependencies) deviceIDs() DeviceIDGenerators {
	return d.IDs.Device
}

func (d UseCaseDependencies) networkRepositories() NetworkRepositories {
	return d.Repositories.Network
}

func (d UseCaseDependencies) networkIDs() NetworkIDGenerators {
	return d.IDs.Network
}

func (d UseCaseDependencies) opsRepositories() OpsRepositories {
	return d.Repositories.Ops
}

func (d UseCaseDependencies) opsIDs() OpsIDGenerators {
	return d.IDs.Ops
}

func (d UseCaseDependencies) wireRepositories() WireRepositories {
	return d.Repositories.Wire
}

func (d UseCaseDependencies) mqttRepositories() MQTTRepositories {
	return d.Repositories.MQTT
}

func (d UseCaseDependencies) downloadRepositories() DownloadRepositories {
	return d.Repositories.Download
}

func (d UseCaseDependencies) mqttConfig() mqttkit.Config {
	return d.Runtime.MQTTConfig
}
