package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newAuthServices(deps UseCaseDependencies) AuthServices {
	repos := deps.authRepositories()
	ids := deps.authIDs()
	return AuthServices{
		UserAuth: servicepkg.NewAuthUserService(
			repos.Users,
			repos.Sessions,
			repos.Devices,
			deps.networkRepositories().Networks,
			ids.NewUserID,
			ids.NewSessionID,
			nil,
		),
		TokenManagement: servicepkg.TokenManagementService{
			Users:    repos.Users,
			Sessions: repos.Sessions,
			Devices:  repos.Devices,
		},
		Aliases: servicepkg.AuthAliasService{
			Users:   repos.Users,
			Aliases: repos.UserAliases,
		},
		ConsoleAuth: servicepkg.NewAuthConsoleService(repos.Users, repos.Sessions, ids.NewSessionID, nil),
		DeviceLoginAuth: servicepkg.NewAuthDeviceLoginService(
			repos.Users,
			repos.Sessions,
			repos.Devices,
			deps.networkRepositories().Networks,
			deps.mqttConfig(),
			servicepkg.NewDeviceControlPublisher(deps.mqttConfig()),
			servicepkg.NewNetworkEventPublisher(deps.mqttConfig(), deps.networkRepositories().EventDeliveries),
			ids.NewSessionID,
			nil,
		),
	}
}
