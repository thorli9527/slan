package service

type ClientDownloadView struct {
	DownloadID   string `json:"downloadId"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	ReleaseNotes string `json:"releaseNotes"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	PlatformName string `json:"platformName"`
	Channel      string `json:"channel"`
	FileSize     int64  `json:"fileSize"`
}

type PlanView struct {
	PlanCode           string `json:"planCode"`
	Name               string `json:"name"`
	DeviceLimit        int    `json:"deviceLimit"`
	Status             string `json:"status"`
	UpdatedAt          int64  `json:"updatedAt"`
	InvitedDeviceLimit int    `json:"invitedDeviceLimit"`
	TotalDeviceLimit   int    `json:"totalDeviceLimit"`
	RelayMonthlyGB     int    `json:"relayMonthlyGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	RelayThrottleMbps  int    `json:"relayThrottleMbps"`
	P2PUnlimited       bool   `json:"p2pUnlimited"`
	CustomDomain       bool   `json:"customDomain"`
	ACL                bool   `json:"acl"`
	DedicatedRelay     bool   `json:"dedicatedRelay"`
	AuditLog           bool   `json:"auditLog"`
	APIAccess          bool   `json:"apiAccess"`
	MonthlyPrice       int64  `json:"monthlyPrice"`
	YearlyPrice        int64  `json:"yearlyPrice"`
}

type ProductView struct {
	ProductID          string `json:"productId"`
	Name               string `json:"name"`
	PlanCode           string `json:"planCode"`
	Price              int64  `json:"price"`
	Status             string `json:"status"`
	CreatedAt          int64  `json:"createdAt"`
	UpdatedAt          int64  `json:"updatedAt"`
	Type               string `json:"type"`
	Period             string `json:"period"`
	ValidDays          int    `json:"validDays"`
	RelayTrafficGB     int    `json:"relayTrafficGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	SalePrice          int64  `json:"salePrice"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"autoRenew"`
	Description        string `json:"description"`
}

type OrderView struct {
	OrderID         string `json:"orderId"`
	CustomerID      string `json:"customerId"`
	ProductID       string `json:"productId"`
	Status          string `json:"status"`
	Amount          int64  `json:"amount"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
	CustomerEmail   string `json:"customerEmail"`
	ProductName     string `json:"productName"`
	ProductType     string `json:"productType"`
	Currency        string `json:"currency"`
	PayStatus       string `json:"payStatus"`
	ProvisionStatus string `json:"provisionStatus"`
	Channel         string `json:"channel"`
	PaidAt          int64  `json:"paidAt"`
	ValidUntil      int64  `json:"validUntil"`
}

type RenewalView struct {
	RenewalID     string `json:"renewalId"`
	OrderID       string `json:"orderId"`
	Status        string `json:"status"`
	RenewAt       int64  `json:"renewAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	CustomerEmail string `json:"customerEmail"`
	PlanCode      string `json:"planCode"`
	Period        string `json:"period"`
	Amount        int64  `json:"amount"`
	PaidAt        int64  `json:"paidAt"`
	Source        string `json:"source"`
	Operator      string `json:"operator"`
}
