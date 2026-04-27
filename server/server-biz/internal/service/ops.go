package service

import "github.com/slan/server/server-biz/api/dto"

// Ops 抽象了运营入口所需的只读管理视图。
type Ops interface {
	// LoginAdmin 校验管理员登录名和密码，并返回 ops 会话令牌。
	LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error)
	// AuthenticateAdminToken 校验管理员登录后的 ops 访问令牌。
	AuthenticateAdminToken(accessToken string) (string, error)
	// AuthorizeAdminMenu 校验管理员是否具备指定菜单码对应的访问权限。
	AuthorizeAdminMenu(adminID string, menuCode string) error
	// Overview 返回运营入口首页所需的全局统计摘要。
	Overview() (dto.OpsOverview, error)
	// ListUsers 返回当前系统中的用户及其聚合视图。
	ListUsers() ([]dto.OpsUser, error)
	// ListDevices 返回当前系统中的设备及其绑定关系视图。
	ListDevices() ([]dto.OpsDevice, error)
	// RelayTopology 返回当前 relay/DERP 拓扑与节点基础状态摘要。
	RelayTopology() (dto.OpsRelayTopology, error)
	// ListAdmins 返回管理员扩展信息列表。
	ListAdmins() ([]dto.OpsAdminInfo, error)
	// UpsertAdminInfo 创建或更新管理员扩展资料。
	UpsertAdminInfo(req dto.UpsertAdminInfoRequest) (dto.OpsAdminInfo, error)
	// ChangeAdminPassword 修改管理员登录密码并刷新密码审计字段。
	ChangeAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error
	// UnlockAdmin 清除管理员的失败计数与锁定状态。
	UnlockAdmin(adminID string) error
	// ListRoles 返回角色及其已绑定菜单列表。
	ListRoles() ([]dto.OpsRole, error)
	// CreateRole 创建一个新的角色。
	CreateRole(req dto.CreateRoleRequest) (dto.OpsRole, error)
	// AssignUserRoles 为用户重设角色绑定关系。
	AssignUserRoles(userID string, req dto.AssignUserRolesRequest) error
	// ListMenus 返回当前可管理的功能菜单列表。
	ListMenus() ([]dto.OpsMenu, error)
	// CreateMenu 创建一个新的功能菜单。
	CreateMenu(req dto.CreateMenuRequest) (dto.OpsMenu, error)
	// AssignRoleMenus 为角色重设菜单绑定关系。
	AssignRoleMenus(roleID string, req dto.AssignRoleMenusRequest) error
	ListProducts() ([]dto.OpsProduct, error)
	UpsertProduct(req dto.UpsertProductRequest) (dto.OpsProduct, error)
	ListPurchaseOrders() ([]dto.OpsPurchaseOrder, error)
	CreatePaidPurchaseOrder(req dto.OpsCreatePaidOrderRequest) (dto.OpsPurchaseOrder, error)
	UpdatePurchaseOrderStatus(orderID string, req dto.UpdatePurchaseOrderStatusRequest) (dto.OpsPurchaseOrder, error)
}
