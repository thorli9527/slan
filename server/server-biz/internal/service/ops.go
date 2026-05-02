package service

import "github.com/slan/server/server-biz/api/dto"

// Ops describes the operations console service surface.
type Ops interface {
	LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error)
	AuthenticateAdminToken(accessToken string) (string, error)
	AuthorizeAdminMenu(adminID string, menuCode string) error
	Overview() (dto.OpsOverview, error)
	ListUsers() ([]dto.OpsUser, error)
	PlanConfig() (dto.PlanConfig, error)
	UpdatePlanConfig(req dto.UpdatePlanConfigRequest) (dto.PlanConfig, error)
	UpdateUserPlanOverride(userID string, req dto.UpdatePlanConfigRequest) (dto.UserPlanOverride, error)
	DeleteUserPlanOverride(userID string) error
	ListDevices() ([]dto.OpsDevice, error)
	RelayTopology() (dto.OpsRelayTopology, error)
	NetworkQuality() (dto.OpsNetworkQuality, error)
	ListAdmins() ([]dto.OpsAdminInfo, error)
	UpsertAdminInfo(req dto.UpsertAdminInfoRequest) (dto.OpsAdminInfo, error)
	ChangeAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error
	ChangeOwnAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error
	LogoutAdmin(accessToken string) error
	UnlockAdmin(adminID string) error
	ListRoles() ([]dto.OpsRole, error)
	CreateRole(req dto.CreateRoleRequest) (dto.OpsRole, error)
	AssignUserRoles(userID string, req dto.AssignUserRolesRequest) error
	ListMenus() ([]dto.OpsMenu, error)
	CreateMenu(req dto.CreateMenuRequest) (dto.OpsMenu, error)
	AssignRoleMenus(roleID string, req dto.AssignRoleMenusRequest) error
}
