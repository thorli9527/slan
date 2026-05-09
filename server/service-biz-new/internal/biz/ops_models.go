package biz

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

type OperatorSession struct {
	SessionID  string `json:"sessionId"`
	OperatorID string `json:"operatorId"`
	Token      string `json:"token"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type OperatorAuthResponse struct {
	Operator OperatorUser    `json:"operator"`
	Session  OperatorSession `json:"session"`
}

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

type OpsRelayNode struct {
	NodeID           string `json:"nodeId"`
	Name             string `json:"name"`
	Region           string `json:"region"`
	Transport        string `json:"transport"`
	PublicAddr       string `json:"publicAddr"`
	InternalAddr     string `json:"internalAddr,omitempty"`
	MaxBandwidthMbps int    `json:"maxBandwidthMbps"`
	MonthlyTrafficGB int    `json:"monthlyTrafficGb"`
	UsedTrafficGB    int    `json:"usedTrafficGb"`
	MaxSessions      int    `json:"maxSessions"`
	ActiveSessions   int    `json:"activeSessions"`
	Status           string `json:"status"`
	Health           string `json:"health"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

type CustomerPlanAssignment struct {
	UserID    string `json:"userId"`
	PlanCode  string `json:"planCode"`
	ExpiresAt int64  `json:"expiresAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

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
