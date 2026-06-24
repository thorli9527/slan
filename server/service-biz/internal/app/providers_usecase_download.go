package app

func newDownloadUseCasesFromServices(services DownloadServices) DownloadUseCases {
	return DownloadUseCases{ClientDelivery: services.ClientDelivery}
}
