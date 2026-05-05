package service

import "github.com/slan/server/server-biz/api/dto"

// Ops describes the operations console service surface.
//
// This interface is used by the ops router (/ops) and typically guarded by ops authentication.
//
// Responsibilities include:
// - Admin login/session management
// - RBAC (roles/menus) management
// - Plan configuration and per-user overrides
// - Global monitoring views (users/devices/relay topology/network quality)
type Ops interface {
	// LoginAdmin authenticates an ops admin and returns an ops access token.
	//
	// Input:
	// - req: username/password or other credential fields per DTO
	// - remoteIP: used for audit/rate-limiting decisions
	LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error)

	// AuthenticateAdminToken validates an ops admin access token and returns the adminID.
	AuthenticateAdminToken(accessToken string) (string, error)

	// AuthorizeAdminMenu checks whether the admin has access to a given menu code.
	AuthorizeAdminMenu(adminID string, menuCode string) error

	// Overview returns aggregated system overview metrics for the ops console.
	Overview() (dto.OpsOverview, error)

	// ListUsers returns all end users visible to ops.
	ListUsers() ([]dto.OpsUser, error)

	// PlanConfig returns the global plan configuration.
	PlanConfig() (dto.PlanConfig, error)

	// UpdatePlanConfig updates the global plan configuration.
	UpdatePlanConfig(req dto.UpdatePlanConfigRequest) (dto.PlanConfig, error)

	// UpdateUserPlanOverride sets a per-user override for plan configuration.
	UpdateUserPlanOverride(userID string, req dto.UpdatePlanConfigRequest) (dto.UserPlanOverride, error)

	// DeleteUserPlanOverride removes a per-user plan override.
	DeleteUserPlanOverride(userID string) error

	// ListDevices returns a global device list for operations.
	ListDevices() ([]dto.OpsDevice, error)

	// RelayTopology returns relay cluster topology and health information.
	RelayTopology() (dto.OpsRelayTopology, error)

	// WireNodes returns the server-wire data-plane node inventory for ops.
	WireNodes() (dto.WireNodesOpsView, error)

	// WireNodeEvents returns paged wire node audit events for ops troubleshooting.
	WireNodeEvents(query dto.WireNodeEventQuery) (dto.WireNodeEventListResponse, error)

	// NetworkQuality returns global network quality aggregates for the ops console.
	NetworkQuality() (dto.OpsNetworkQuality, error)

	// ListAdmins lists ops admins.
	ListAdmins() ([]dto.OpsAdminInfo, error)

	// UpsertAdminInfo creates or updates an ops admin record.
	UpsertAdminInfo(req dto.UpsertAdminInfoRequest) (dto.OpsAdminInfo, error)

	// ChangeAdminPassword changes a specific admin's password (admin-management capability).
	ChangeAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error

	// ChangeOwnAdminPassword changes the current admin's own password.
	ChangeOwnAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error

	// LogoutAdmin invalidates an ops admin access token.
	LogoutAdmin(accessToken string) error

	// UnlockAdmin unlocks an admin account after too many failed attempts or a manual lock.
	UnlockAdmin(adminID string) error

	// ListRoles returns RBAC roles.
	ListRoles() ([]dto.OpsRole, error)

	// CreateRole creates a new RBAC role.
	CreateRole(req dto.CreateRoleRequest) (dto.OpsRole, error)

	// AssignUserRoles assigns roles to an end user.
	AssignUserRoles(userID string, req dto.AssignUserRolesRequest) error

	// ListMenus returns RBAC menus.
	ListMenus() ([]dto.OpsMenu, error)

	// CreateMenu creates a new RBAC menu.
	CreateMenu(req dto.CreateMenuRequest) (dto.OpsMenu, error)

	// AssignRoleMenus assigns menus to a role.
	AssignRoleMenus(roleID string, req dto.AssignRoleMenusRequest) error
}
