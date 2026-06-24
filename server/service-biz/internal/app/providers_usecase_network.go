package app

func newNetworkUseCasesFromServices(services NetworkServices) NetworkUseCases {
	return NetworkUseCases{
		CoreAccess:       services.CoreAccess,
		InviteManagement: services.InviteManagement,
		DNSManagement:    services.DNSManagement,
		AccessManagement: services.AccessManagement,
		RuntimeControl:   services.RuntimeControl,
	}
}
