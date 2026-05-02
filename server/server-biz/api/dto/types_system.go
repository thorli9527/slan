package dto

// ErrorResponse 是 HTTP 处理器统一使用的错误响应结构。
type ErrorResponse struct {
	// Code 是稳定的机器可读错误码。
	Code string `json:"code"`
	// Message 是给人看的错误信息。
	Message string `json:"message"`
}

// OpsOverview 描述运营入口首页的核心统计。
type OpsOverview struct {
	// UserCount 是当前系统中的用户总数。
	UserCount int `json:"userCount"`
	// DeviceCount 是当前系统中的设备总数。
	DeviceCount int `json:"deviceCount"`
	// OnlineDeviceCount 是最近仍处于虚拟网络在线状态的设备数。
	OnlineDeviceCount int `json:"onlineDeviceCount"`
	// NodeCount 是当前注册的节点总数。
	NodeCount int `json:"nodeCount"`
	// RelayClusterCount 是当前可见的 relay 集群数量。
	RelayClusterCount int `json:"relayClusterCount"`
	// RelayNodeCount 是当前配置或观测到的 relay 节点数量。
	RelayNodeCount int `json:"relayNodeCount"`
	// RelayOnlineNodeCount 是最近仍有健康 MQTT 心跳的 relay 节点数量。
	RelayOnlineNodeCount int `json:"relayOnlineNodeCount"`
	// DefaultAdminSeeded 表示默认管理员是否已经落库。
	DefaultAdminSeeded bool `json:"defaultAdminSeeded"`
	// DefaultAdminLoginName 是配置中的默认管理员登录名。
	DefaultAdminLoginName string `json:"defaultAdminLoginName,omitempty"`
	// DefaultAdminRoleBound 表示默认管理员是否已绑定 ops-super-admin。
	DefaultAdminRoleBound bool `json:"defaultAdminRoleBound"`
	// SecurityWarnings 是当前环境需要运营关注的安全告警摘要。
	SecurityWarnings []string `json:"securityWarnings,omitempty"`
}

// OpsUser 描述运营入口查看到的用户摘要。
type OpsUser struct {
	// UserID 是用户唯一标识。
	UserID string `json:"userId"`
	// Email 是用户登录邮箱。
	Email string `json:"email"`
	// DeviceCount 是该用户名下设备数量。
	DeviceCount int `json:"deviceCount"`
	// NodeCount 是该用户名下节点数量。
	NodeCount int `json:"nodeCount"`
	// RoleIDs 是当前绑定到用户的角色 ID 列表。
	RoleIDs []string `json:"roleIds,omitempty"`
	// RoleCodes 是当前绑定到用户的角色编码列表。
	RoleCodes []string `json:"roleCodes,omitempty"`
	// RoleNames 是当前绑定到用户的角色名称列表。
	RoleNames []string `json:"roleNames,omitempty"`
	// PlanOverride 是该用户的专属配额覆盖配置；为空时继承全局配置。
	PlanOverride *PlanConfig `json:"planOverride,omitempty"`
}

type PlanConfig struct {
	MaxActiveDevices int `json:"maxActiveDevices"`
	RelayIngressKbps int `json:"relayIngressKbps"`
	RelayEgressKbps  int `json:"relayEgressKbps"`
	UDPIngressKbps   int `json:"udpIngressKbps"`
	UDPEgressKbps    int `json:"udpEgressKbps"`
}

type UpdatePlanConfigRequest = PlanConfig

type UserPlanOverride struct {
	UserID string `json:"userId"`
	PlanConfig
}

// OpsDevice 描述运营入口查看到的设备摘要。
type OpsDevice struct {
	// Device 是设备基础信息。
	Device
	// UserID 是设备所属用户 ID。
	UserID string `json:"userId"`
	// NodeCount 是设备下挂载的节点数量。
	NodeCount int `json:"nodeCount"`
	// NodeIDs 是设备当前关联的全部节点 ID。
	NodeIDs []string `json:"nodeIds,omitempty"`
}

// OpsRelayNode 描述运营入口查看到的 relay 节点摘要。
type OpsRelayNode struct {
	// NodeID 是 relay 节点唯一标识。
	NodeID string `json:"nodeId"`
	// ClusterID 是节点所属集群 ID。
	ClusterID string `json:"clusterId"`
	// ClusterName 是节点所属集群名称。
	ClusterName string `json:"clusterName"`
	// CountryCode 是节点所属国家编码。
	CountryCode string `json:"countryCode,omitempty"`
	// CountryName 是节点所属国家名称。
	CountryName string `json:"countryName,omitempty"`
	// CityCode 是节点所属城市编码。
	CityCode string `json:"cityCode,omitempty"`
	// CityName 是节点所属城市名称。
	CityName string `json:"cityName,omitempty"`
	// Transport 是节点支持的传输类型。
	Transport string `json:"transport"`
	// Address 是节点对外监听地址。
	Address string `json:"address"`
	// Priority 是控制面静态优先级。
	Priority int `json:"priority"`
	// ObservedRttMs 是最近健康样本中的观测 RTT。
	ObservedRttMs uint32 `json:"observedRttMs,omitempty"`
	// PacketLossPpm 是最近健康样本中的丢包率，单位为 ppm。
	PacketLossPpm uint32 `json:"packetLossPpm,omitempty"`
	// PathScore 是控制面聚合后的路径评分。
	PathScore uint32 `json:"pathScore,omitempty"`
	// SampleCount 是参与聚合的样本数量。
	SampleCount int `json:"sampleCount,omitempty"`
	// HeartbeatOnline indicates whether the relay daemon MQTT heartbeat is fresh.
	HeartbeatOnline bool `json:"heartbeatOnline,omitempty"`
	// HeartbeatLastSeenAt is the latest relay heartbeat write time.
	HeartbeatLastSeenAt int64 `json:"heartbeatLastSeenAt,omitempty"`
	// ActiveSessions is the relay daemon reported active session count.
	ActiveSessions int `json:"activeSessions,omitempty"`
}

// OpsRelayTopology 描述运营入口查看到的 relay 拓扑和健康摘要。
type OpsRelayTopology struct {
	// DefaultClusterID 是当前默认 relay 集群 ID。
	DefaultClusterID string `json:"defaultClusterId"`
	// Regions 是按区域组织后的 relay 拓扑视图。
	Regions []RelayRegion `json:"regions,omitempty"`
	// Nodes 是扁平化后的 relay 节点摘要列表。
	Nodes []OpsRelayNode `json:"nodes,omitempty"`
}

// OpsNetworkQuality 描述运营入口看到的用户设备实时网络质量样本。
type OpsNetworkQuality struct {
	// Items 是最近上报的路径质量样本。
	Items []OpsNetworkQualityItem `json:"items,omitempty"`
}

// OpsNetworkQualityItem 展示一条节点路径质量样本及其用户、设备、网络上下文。
type OpsNetworkQualityItem struct {
	HealthID      string `json:"healthId"`
	NetworkID     string `json:"networkId"`
	NetworkName   string `json:"networkName,omitempty"`
	UserID        string `json:"userId,omitempty"`
	UserEmail     string `json:"userEmail,omitempty"`
	DeviceID      string `json:"deviceId,omitempty"`
	DeviceName    string `json:"deviceName,omitempty"`
	NodeID        string `json:"nodeId"`
	PeerNodeID    string `json:"peerNodeId,omitempty"`
	PathType      string `json:"pathType"`
	Endpoint      string `json:"endpoint,omitempty"`
	DerpNodeID    string `json:"derpNodeId,omitempty"`
	ObservedRttMs uint32 `json:"observedRttMs,omitempty"`
	PacketLossPpm uint32 `json:"packetLossPpm,omitempty"`
	PathScore     uint32 `json:"pathScore,omitempty"`
	SampledAtMs   uint64 `json:"sampledAtMs,omitempty"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// OpsAdminInfo 描述管理员信息。
type OpsAdminInfo struct {
	// AdminID 是管理员扩展信息记录 ID。
	AdminID string `json:"adminId"`
	// UserID 是绑定的业务用户 ID。
	UserID string `json:"userId"`
	// LoginName 是管理员登录名。
	LoginName string `json:"loginName"`
	// Email 是管理员关联账号邮箱。
	Email string `json:"email,omitempty"`
	// DisplayName 是管理员显示名称。
	DisplayName string `json:"displayName"`
	// Phone 是管理员联系电话。
	Phone string `json:"phone,omitempty"`
	// Title 是岗位或职务。
	Title string `json:"title,omitempty"`
	// Department 是所属部门。
	Department string `json:"department,omitempty"`
	// Status 是管理员状态，例如 active 或 disabled。
	Status string `json:"status"`
	// PasswordUpdatedAt 是最近一次密码更新时间戳，单位毫秒。
	PasswordUpdatedAt int64 `json:"passwordUpdatedAt,omitempty"`
	// LastLoginAt 是最近一次成功登录时间戳，单位毫秒。
	LastLoginAt int64 `json:"lastLoginAt,omitempty"`
	// LastLoginIP 是最近一次成功登录来源 IP。
	LastLoginIP string `json:"lastLoginIp,omitempty"`
	// FailedLoginCount 是当前连续登录失败次数。
	FailedLoginCount int `json:"failedLoginCount,omitempty"`
	// LockedUntil 是账号锁定截止时间戳，单位毫秒。
	LockedUntil int64 `json:"lockedUntil,omitempty"`
	// UsingSeedPassword 表示该管理员当前仍在使用启动 seed 密码。
	UsingSeedPassword bool `json:"usingSeedPassword,omitempty"`
	// RoleIDs 是当前绑定角色 ID 列表。
	RoleIDs []string `json:"roleIds,omitempty"`
	// RoleCodes 是当前绑定角色编码列表。
	RoleCodes []string `json:"roleCodes,omitempty"`
	// RoleNames 是当前绑定角色名称列表。
	RoleNames []string `json:"roleNames,omitempty"`
}

// UpsertAdminInfoRequest 用于创建或更新管理员信息。
type UpsertAdminInfoRequest struct {
	// UserID 是要补充管理员资料的业务用户 ID。
	UserID string `json:"userId"`
	// LoginName 是管理员登录名。
	LoginName string `json:"loginName"`
	// Password 是管理员登录密码；为空时保留原密码。
	Password string `json:"password,omitempty"`
	// DisplayName 是管理员显示名称。
	DisplayName string `json:"displayName"`
	// Phone 是管理员联系电话。
	Phone string `json:"phone,omitempty"`
	// Title 是岗位或职务。
	Title string `json:"title,omitempty"`
	// Department 是所属部门。
	Department string `json:"department,omitempty"`
	// Status 是管理员状态。
	Status string `json:"status,omitempty"`
}

// OpsLoginRequest 用于管理员登录运营入口。
type OpsLoginRequest struct {
	// LoginName 是管理员登录名。
	LoginName string `json:"loginName"`
	// Password 是管理员登录密码。
	Password string `json:"password"`
}

// OpsLoginResponse 描述管理员登录成功后返回的会话结果。
type OpsLoginResponse struct {
	// AdminID 是当前登录管理员 ID。
	AdminID string `json:"adminId"`
	// UserID 是管理员绑定的业务用户 ID。
	UserID string `json:"userId"`
	// LoginName 是当前登录名。
	LoginName string `json:"loginName"`
	// DisplayName 是管理员显示名称。
	DisplayName string `json:"displayName"`
	// AccessToken 是 ops HTTP 实例使用的 Bearer token。
	AccessToken string `json:"accessToken"`
	// ExpiresIn 是 access token 剩余秒数。
	ExpiresIn int64 `json:"expiresIn"`
}

// ChangeAdminPasswordRequest 用于修改管理员登录密码。
type ChangeAdminPasswordRequest struct {
	// Password 是新的管理员登录密码。
	Password string `json:"password"`
}

// OpsRole 描述角色。
type OpsRole struct {
	// RoleID 是角色唯一标识。
	RoleID string `json:"roleId"`
	// RoleCode 是稳定的角色编码。
	RoleCode string `json:"roleCode"`
	// RoleName 是角色展示名称。
	RoleName string `json:"roleName"`
	// Description 是角色说明。
	Description string `json:"description,omitempty"`
	// Builtin 标记该角色是否为系统内置角色。
	Builtin bool `json:"builtin"`
	// MenuIDs 是该角色当前绑定的菜单 ID 列表。
	MenuIDs []string `json:"menuIds,omitempty"`
	// MenuCodes 是该角色当前绑定的菜单编码列表。
	MenuCodes []string `json:"menuCodes,omitempty"`
	// MenuNames 是该角色当前绑定的菜单名称列表。
	MenuNames []string `json:"menuNames,omitempty"`
}

// CreateRoleRequest 用于创建角色。
type CreateRoleRequest struct {
	// RoleCode 是角色的稳定编码。
	RoleCode string `json:"roleCode"`
	// RoleName 是角色展示名称。
	RoleName string `json:"roleName"`
	// Description 是角色说明。
	Description string `json:"description,omitempty"`
}

// AssignUserRolesRequest 用于绑定用户角色。
type AssignUserRolesRequest struct {
	// RoleIDs 是用户最终应绑定的角色 ID 列表。
	RoleIDs []string `json:"roleIds"`
}

// OpsMenu 描述功能菜单。
type OpsMenu struct {
	// MenuID 是菜单唯一标识。
	MenuID string `json:"menuId"`
	// MenuCode 是稳定菜单编码。
	MenuCode string `json:"menuCode"`
	// MenuName 是菜单展示名称。
	MenuName string `json:"menuName"`
	// Path 是前端路由或功能入口路径。
	Path string `json:"path,omitempty"`
	// ParentID 是父菜单 ID，用于构建菜单树。
	ParentID string `json:"parentId,omitempty"`
	// Sort 是菜单排序权重。
	Sort int `json:"sort"`
	// Status 是菜单状态，例如 active 或 disabled。
	Status string `json:"status"`
}

// CreateMenuRequest 用于创建功能菜单。
type CreateMenuRequest struct {
	// MenuCode 是菜单稳定编码。
	MenuCode string `json:"menuCode"`
	// MenuName 是菜单展示名称。
	MenuName string `json:"menuName"`
	// Path 是菜单对应的功能路径。
	Path string `json:"path,omitempty"`
	// ParentID 是父菜单 ID。
	ParentID string `json:"parentId,omitempty"`
	// Sort 是排序值。
	Sort int `json:"sort,omitempty"`
	// Status 是菜单状态。
	Status string `json:"status,omitempty"`
}

// AssignRoleMenusRequest 用于绑定角色菜单。
type AssignRoleMenusRequest struct {
	// MenuIDs 是角色最终应绑定的菜单 ID 列表。
	MenuIDs []string `json:"menuIds"`
}
