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
			deps.networkIDs().NewNetworkID,
			ids.NewSessionID,
			nil,
			nil,
		),
		Aliases: servicepkg.AuthAliasService{
			Users:   repos.Users,
			Aliases: repos.UserAliases,
		},
		ConsoleAuth: servicepkg.NewAuthConsoleService(repos.Users, repos.Sessions, ids.NewSessionID, nil),
		DeviceLoginAuth: servicepkg.NewAuthDeviceLoginService(
			repos.Users,
			repos.Devices,
			deps.networkRepositories().Networks,
			deps.mqttConfig(),
			nil,
		),
	}
}
