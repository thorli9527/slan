package service

import "context"

func (s OpsDashboardService) Dashboard(ctx context.Context) (OpsDashboardView, error) {
	users, _ := s.Users.ListUsers(ctx)
	operators, _ := s.Operators.ListOperators(ctx)
	networks := 0
	devices := 0
	for _, user := range users {
		userNetworks, _ := s.Networks.ListNetworksByOwner(ctx, user.UserID)
		networks += len(userNetworks)
		userDevices, _ := s.Devices.ListDevicesByOwner(ctx, user.UserID)
		devices += len(userDevices)
	}
	return OpsDashboardView{
		Users:     len(users),
		Devices:   devices,
		Networks:  networks,
		Operators: len(operators),
	}, nil
}
