package app

func newMQTTUseCasesFromServices(services MQTTServices) MQTTUseCases {
	return MQTTUseCases{BrokerWebhook: services.BrokerWebhook}
}
