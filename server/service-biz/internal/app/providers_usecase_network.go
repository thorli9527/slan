package app

func newNetworkUseCasesFromServices(services NetworkServices) NetworkUseCases {
	return NetworkUseCases{
		CoreAccess:       services.CoreAccess,
		DNSManagement:    services.DNSManagement,
		AccessManagement: services.AccessManagement,
		RuntimeControl:   services.RuntimeControl,
	}
}
