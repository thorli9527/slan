package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newMQTTServices(deps UseCaseDependencies) MQTTServices {
	repos := deps.mqttRepositories()
	return MQTTServices{
		BrokerWebhook: servicepkg.MQTTWebhookService{
			Networks:        repos.Networks,
			EventPublisher:  servicepkg.NewNetworkEventPublisher(deps.mqttConfig(), repos.EventDeliveries),
			EventDeliveries: repos.EventDeliveries,
			Config:          deps.mqttConfig(),
		},
	}
}
