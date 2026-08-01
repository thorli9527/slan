package app

func newAuthUseCasesFromServices(services AuthServices) AuthUseCases {
	return AuthUseCases{
		UserRegistration:    services.UserAuth,
		UserSessions:        services.UserAuth,
		UserTokens:          services.TokenManagement,
		UserAccounts:        services.UserAuth,
		Aliases:             services.Aliases,
		ConsoleKeys:         services.ConsoleAuth,
		ConsoleLogin:        services.ConsoleAuth,
		DeviceLoginPrepare:  services.DeviceLoginAuth,
		DeviceLoginComplete: services.DeviceLoginAuth,
	}
}
