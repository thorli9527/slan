package biz

// OperatorUser 是运营后台管理员账号。
type OperatorUser struct {
	OperatorID   string `json:"operatorId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	PasswordHash string `json:"-"`
	LastLoginAt  int64  `json:"lastLoginAt,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// OperatorSession 是运营后台登录会话。
type OperatorSession struct {
	SessionID  string `json:"sessionId"`
	OperatorID string `json:"operatorId"`
	Token      string `json:"token"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
}

// OperatorAuthResponse 是运营后台登录成功返回的认证信息。
type OperatorAuthResponse struct {
	Operator OperatorUser    `json:"operator"`
	Session  OperatorSession `json:"session"`
}

// DeviceQuota 是按用户套餐计算后的设备额度视图。
type DeviceQuota struct {
	UserID             string `json:"userId"`
	PlanCode           string `json:"planCode"`
	PlanName           string `json:"planName"`
	OwnDeviceLimit     int    `json:"ownDeviceLimit"`
	InvitedDeviceLimit int    `json:"invitedDeviceLimit"`
	TotalDeviceLimit   int    `json:"totalDeviceLimit"`
	OwnDevices         int    `json:"ownDevices"`
	InvitedDevices     int    `json:"invitedDevices"`
	TotalDevices       int    `json:"totalDevices"`
	RemainingDevices   int    `json:"remainingDevices"`
	PlanExpiresAt      int64  `json:"planExpiresAt,omitempty"`
	Status             string `json:"status"`
}

// OpsPlan 是运营后台维护的套餐能力定义。
type OpsPlan struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	OwnDeviceLimit     int     `json:"ownDeviceLimit"`
	InvitedDeviceLimit int     `json:"invitedDeviceLimit"`
	TotalDeviceLimit   int     `json:"totalDeviceLimit"`
	RelayMonthlyGB     int     `json:"relayMonthlyGb"`
	RelayBandwidthMbps int     `json:"relayBandwidthMbps"`
	RelayThrottleMbps  int     `json:"relayThrottleMbps,omitempty"`
	P2PUnlimited       bool    `json:"p2pUnlimited"`
	CustomDomain       bool    `json:"customDomain"`
	ACL                bool    `json:"acl"`
	DedicatedRelay     bool    `json:"dedicatedRelay"`
	AuditLog           bool    `json:"auditLog"`
	APIAccess          bool    `json:"apiAccess"`
	MonthlyPrice       float64 `json:"monthlyPrice"`
	YearlyPrice        float64 `json:"yearlyPrice"`
	Status             string  `json:"status"`
}

// Product 是可售卖的套餐商品或增值服务。
type Product struct {
	ProductID          string  `json:"productId"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	PlanCode           string  `json:"planCode"`
	Period             string  `json:"period"`
	ValidDays          int     `json:"validDays"`
	RelayTrafficGB     int     `json:"relayTrafficGb"`
	RelayBandwidthMbps int     `json:"relayBandwidthMbps"`
	ListPrice          float64 `json:"listPrice"`
	SalePrice          float64 `json:"salePrice"`
	Currency           string  `json:"currency"`
	AutoRenew          bool    `json:"autoRenew"`
	Status             string  `json:"status"`
	Description        string  `json:"description,omitempty"`
	CreatedAt          int64   `json:"createdAt"`
	UpdatedAt          int64   `json:"updatedAt"`
}

// OpsRelayNode 是运营后台管理的 UDP relay/DERP 中继节点。
type OpsRelayNode struct {
	// NodeID 是节点唯一 ID。
	NodeID string `json:"nodeId"`
	// Name 是后台显示名称。
	Name string `json:"name"`
	// Region 是节点所在区域。
	Region string `json:"region"`
	// Transport 表示节点类型，例如 relay_udp 或 derp_tcp_tls_443。
	Transport string `json:"transport"`
	// PublicAddr 是客户端可访问的公网地址。
	PublicAddr        string              `json:"publicAddr"`
	InternalAddr      string              `json:"internalAddr,omitempty"`
	MaxBandwidthMbps  int                 `json:"maxBandwidthMbps"`
	MonthlyTrafficGB  int                 `json:"monthlyTrafficGb"`
	UsedTrafficGB     int                 `json:"usedTrafficGb"`
	MaxSessions       int                 `json:"maxSessions"`
	ActiveSessions    int                 `json:"activeSessions"`
	Status            string              `json:"status"`
	Health            string              `json:"health"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
	CreatedAt         int64               `json:"createdAt"`
	UpdatedAt         int64               `json:"updatedAt"`
}

// OpsPunchNode 是运营后台管理的 P2P 打洞节点。
type OpsPunchNode struct {
	// NodeID 是打洞节点唯一 ID。
	NodeID string `json:"nodeId"`
	// Name 是后台显示名称。
	Name string `json:"name"`
	// Region 是节点所在区域。
	Region string `json:"region"`
	// PublicUDPIP 是客户端和 biz 访问 punch UDP 服务的公网 IP。
	PublicUDPIP string `json:"publicUdpIp"`
	// PublicUDPPort 是 punch UDP 服务端口，HTTP 管理端口约定为该端口 + 1。
	PublicUDPPort  int    `json:"publicUdpPort"`
	MaxSessions    int    `json:"maxSessions"`
	ActiveSessions int    `json:"activeSessions"`
	Status         string `json:"status"`
	Health         string `json:"health"`
	Priority       int    `json:"priority,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// CustomerPlanAssignment 是客户当前套餐分配记录。
type CustomerPlanAssignment struct {
	UserID    string `json:"userId"`
	PlanCode  string `json:"planCode"`
	ExpiresAt int64  `json:"expiresAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// CustomerProfile 是运营后台客户列表使用的聚合视图。
type CustomerProfile struct {
	CustomerID     string  `json:"customerId"`
	Email          string  `json:"email"`
	Name           string  `json:"name,omitempty"`
	Country        string  `json:"country,omitempty"`
	Province       string  `json:"province,omitempty"`
	City           string  `json:"city,omitempty"`
	IPRegion       string  `json:"ipRegion,omitempty"`
	PlanCode       string  `json:"planCode"`
	PlanExpiresAt  int64   `json:"planExpiresAt,omitempty"`
	OwnDevices     int     `json:"ownDevices"`
	InvitedDevices int     `json:"invitedDevices"`
	RelayUsedGB    float64 `json:"relayUsedGb"`
	Status         string  `json:"status"`
}

// OpsDeviceView 是运营后台设备列表使用的聚合视图。
type OpsDeviceView struct {
	DeviceID        string `json:"deviceId"`
	OwnerID         string `json:"ownerId"`
	OwnerEmail      string `json:"ownerEmail,omitempty"`
	Name            string `json:"name"`
	Alias           string `json:"alias,omitempty"`
	Platform        string `json:"platform"`
	OSName          string `json:"osName,omitempty"`
	OSVersion       string `json:"osVersion,omitempty"`
	GlobalIP        string `json:"globalIp"`
	GlobalName      string `json:"globalName"`
	Status          string `json:"status"`
	HeartbeatOnline bool   `json:"heartbeatOnline"`
	NetworkEnabled  bool   `json:"networkEnabled"`
	DeviceEnabled   bool   `json:"deviceEnabled"`
	RxBytesTotal    uint64 `json:"rxBytesTotal"`
	TxBytesTotal    uint64 `json:"txBytesTotal"`
	NetworkCount    int    `json:"networkCount"`
	LastSeenAt      int64  `json:"lastSeenAt,omitempty"`
	LastReportAt    int64  `json:"lastReportAt,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

// Order 是订单记录，描述客户购买商品及开通状态。
type Order struct {
	OrderID         string  `json:"orderId"`
	CustomerID      string  `json:"customerId"`
	CustomerEmail   string  `json:"customerEmail"`
	ProductID       string  `json:"productId"`
	ProductName     string  `json:"productName"`
	ProductType     string  `json:"productType"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	PayStatus       string  `json:"payStatus"`
	ProvisionStatus string  `json:"provisionStatus"`
	Channel         string  `json:"channel"`
	CreatedAt       int64   `json:"createdAt"`
	PaidAt          int64   `json:"paidAt,omitempty"`
	ValidUntil      int64   `json:"validUntil,omitempty"`
}

// Renewal 是套餐续费记录。
type Renewal struct {
	RenewalID     string  `json:"renewalId"`
	CustomerID    string  `json:"customerId"`
	CustomerEmail string  `json:"customerEmail"`
	PlanCode      string  `json:"planCode"`
	Period        string  `json:"period"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	PaidAt        int64   `json:"paidAt"`
	ValidUntil    int64   `json:"validUntil"`
	Source        string  `json:"source"`
	Operator      string  `json:"operator,omitempty"`
}
