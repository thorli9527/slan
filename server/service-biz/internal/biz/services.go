package biz

// ApplicationServices 聚合 service-biz 的业务服务实现。
// HTTP/MQTT 入口负责协议细节，业务服务负责参数语义、跨 store 调用编排和响应对象组装。
type ApplicationServices struct {
	Auth     AuthService
	Devices  DeviceService
	Network  NetworkService
	Ops      OpsService
	Download DownloadService
	MQTT     MQTTControlService
	Audit    AuditService
	Runtime  RuntimeService
	Punch    PunchService
	Wire     WireService
}

func newApplicationServices(store BusinessStore, mqtt MQTTConfig) ApplicationServices {
	return ApplicationServices{
		Auth:     AuthService{store: store},
		Devices:  DeviceService{store: store},
		Network:  NetworkService{store: store},
		Ops:      OpsService{store: store},
		Download: DownloadService{store: store},
		MQTT:     MQTTControlService{store: store},
		Audit:    AuditService{store: store},
		Runtime:  RuntimeService{store: store, mqtt: mqtt},
		Punch:    PunchService{store: store, mqtt: mqtt},
		Wire:     WireService{store: store},
	}
}
