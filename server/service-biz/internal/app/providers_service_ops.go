package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newOpsServices(deps UseCaseDependencies) OpsServices {
	repos := deps.opsRepositories()
	ids := deps.opsIDs()
	return OpsServices{
		SessionAuth: servicepkg.OpsAuthSessionService{
			Operators:         repos.Operators,
			OperatorSessions:  repos.OperatorSessions,
			NewOperatorSessID: ids.NewSessionID,
		},
		OperatorDirectory: servicepkg.OpsOperatorService{
			Operators: repos.Operators,
		},
		OperatorSecurity: servicepkg.OpsOperatorPasswordService{
			Operators: repos.Operators,
		},
		DashboardOverview: servicepkg.OpsDashboardService{
			Users:     repos.Users,
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			Operators: repos.Operators,
		},
		AuditOverview: servicepkg.OpsAuditService{
			Audit: repos.Audit,
			Now:   deps.now(),
		},
		NodeRegistry: servicepkg.OpsNodeService{
			Nodes: repos.Nodes,
		},
		UserDirectory: servicepkg.OpsUserService{
			Users:     repos.Users,
			Devices:   repos.Devices,
			NewUserID: deps.authIDs().NewUserID,
		},
		DeviceDirectory: servicepkg.OpsManagedDeviceService{
			Users:    repos.Users,
			Devices:  repos.Devices,
			Networks: repos.Networks,
		},
	}
}
