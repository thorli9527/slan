package service

import "github.com/slan/server/server-biz/api/dto"

// Ops describes the operations console service surface.
type Ops interface {
	LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error)
	AuthenticateAdminToken(accessToken string) (string, error)
	AuthorizeAdminMenu(adminID string, menuCode string) error
	Overview() (dto.OpsOverview, error)
	ListUsers() ([]dto.OpsUser, error)
	ListDevices() ([]dto.OpsDevice, error)
	RelayTopology() (dto.OpsRelayTopology, error)
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
	ListProducts() ([]dto.OpsProduct, error)
	UpsertProduct(req dto.UpsertProductRequest) (dto.OpsProduct, error)
	ListPurchaseOrders() ([]dto.OpsPurchaseOrder, error)
	CreatePaidPurchaseOrder(req dto.OpsCreatePaidOrderRequest) (dto.OpsPurchaseOrder, error)
	UpdatePurchaseOrderStatus(orderID string, req dto.UpdatePurchaseOrderStatusRequest) (dto.OpsPurchaseOrder, error)
}
