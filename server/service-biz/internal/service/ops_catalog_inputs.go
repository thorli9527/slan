package service

type UpsertClientDownloadInput struct {
	DownloadID   string `json:"downloadId"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	Channel      string `json:"channel"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	ReleaseNotes string `json:"releaseNotes"`
	Status       string `json:"status"`
}

type UpsertPlanInput struct {
	PlanCode           string `json:"planCode"`
	Name               string `json:"name"`
	DeviceLimit        int    `json:"deviceLimit"`
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
	Status             string `json:"status"`
}

type CreateProductInput struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	PlanCode           string `json:"planCode"`
	Period             string `json:"period"`
	ValidDays          int    `json:"validDays"`
	RelayTrafficGB     int    `json:"relayTrafficGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	Price              int64  `json:"price"`
	SalePrice          int64  `json:"salePrice"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"autoRenew"`
	Status             string `json:"status"`
	Description        string `json:"description"`
}

type UpdateProductInput struct {
	ProductID          string `json:"productId"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	PlanCode           string `json:"planCode"`
	Period             string `json:"period"`
	ValidDays          int    `json:"validDays"`
	RelayTrafficGB     int    `json:"relayTrafficGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	Price              int64  `json:"price"`
	SalePrice          int64  `json:"salePrice"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"autoRenew"`
	Status             string `json:"status"`
	Description        string `json:"description"`
}

type CreateOrderInput struct {
	CustomerID      string `json:"customerId"`
	CustomerEmail   string `json:"customerEmail"`
	ProductID       string `json:"productId"`
	ProductName     string `json:"productName"`
	ProductType     string `json:"productType"`
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
	PayStatus       string `json:"payStatus"`
	ProvisionStatus string `json:"provisionStatus"`
	Channel         string `json:"channel"`
	PaidAt          int64  `json:"paidAt"`
	ValidUntil      int64  `json:"validUntil"`
}

type UpdateOrderInput struct {
	OrderID          string `json:"orderId"`
	CustomerID       string `json:"customerId"`
	CustomerEmail    string `json:"customerEmail"`
	ProductID        string `json:"productId"`
	ProductName      string `json:"productName"`
	ProductType      string `json:"productType"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	PayStatus        string `json:"payStatus"`
	ProvisionStatus  string `json:"provisionStatus"`
	Channel          string `json:"channel"`
	PaidAt           int64  `json:"paidAt"`
	ValidUntil       int64  `json:"validUntil"`
}

type UpdateRenewalInput struct {
	RenewalID     string `json:"renewalId"`
	OrderID       string `json:"orderId"`
	CustomerID    string `json:"customerId"`
	CustomerEmail string `json:"customerEmail"`
	PlanCode      string `json:"planCode"`
	Period        string `json:"period"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
	RenewAt       int64  `json:"renewAt"`
	PaidAt        int64  `json:"paidAt"`
	Source        string `json:"source"`
	Operator      string `json:"operator"`
}
