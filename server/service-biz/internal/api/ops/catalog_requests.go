package ops

import servicepkg "github.com/slan/service-biz/internal/service"

type upsertPlanRequest struct {
	PlanCode           string `json:"planCode"`
	Name               string `json:"name"`
	DeviceLimit        int    `json:"deviceLimit"`
	OwnDeviceLimit     int    `json:"ownDeviceLimit"`
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

func (r upsertPlanRequest) toInput() servicepkg.UpsertPlanInput {
	deviceLimit := r.DeviceLimit
	if deviceLimit == 0 {
		deviceLimit = r.OwnDeviceLimit
	}
	return servicepkg.UpsertPlanInput{
		PlanCode:           r.PlanCode,
		Name:               r.Name,
		DeviceLimit:        deviceLimit,
		InvitedDeviceLimit: r.InvitedDeviceLimit,
		TotalDeviceLimit:   r.TotalDeviceLimit,
		RelayMonthlyGB:     r.RelayMonthlyGB,
		RelayBandwidthMbps: r.RelayBandwidthMbps,
		RelayThrottleMbps:  r.RelayThrottleMbps,
		P2PUnlimited:       r.P2PUnlimited,
		CustomDomain:       r.CustomDomain,
		ACL:                r.ACL,
		DedicatedRelay:     r.DedicatedRelay,
		AuditLog:           r.AuditLog,
		APIAccess:          r.APIAccess,
		MonthlyPrice:       r.MonthlyPrice,
		YearlyPrice:        r.YearlyPrice,
		Status:             r.Status,
	}
}

type createProductRequest struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	PlanCode           string `json:"planCode"`
	Period             string `json:"period"`
	ValidDays          int    `json:"validDays"`
	RelayTrafficGB     int    `json:"relayTrafficGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	Price              int64  `json:"price"`
	ListPrice          int64  `json:"listPrice"`
	SalePrice          int64  `json:"salePrice"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"autoRenew"`
	Status             string `json:"status"`
	Description        string `json:"description"`
}

func (r createProductRequest) toInput() servicepkg.CreateProductInput {
	price := r.Price
	if price == 0 {
		price = r.ListPrice
	}
	return servicepkg.CreateProductInput{
		Name:               r.Name,
		Type:               r.Type,
		PlanCode:           r.PlanCode,
		Period:             r.Period,
		ValidDays:          r.ValidDays,
		RelayTrafficGB:     r.RelayTrafficGB,
		RelayBandwidthMbps: r.RelayBandwidthMbps,
		Price:              price,
		SalePrice:          r.SalePrice,
		Currency:           r.Currency,
		AutoRenew:          r.AutoRenew,
		Status:             r.Status,
		Description:        r.Description,
	}
}

type updateProductRequest struct {
	ProductID          string `json:"productId"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	PlanCode           string `json:"planCode"`
	Period             string `json:"period"`
	ValidDays          int    `json:"validDays"`
	RelayTrafficGB     int    `json:"relayTrafficGb"`
	RelayBandwidthMbps int    `json:"relayBandwidthMbps"`
	Price              int64  `json:"price"`
	ListPrice          int64  `json:"listPrice"`
	SalePrice          int64  `json:"salePrice"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"autoRenew"`
	Status             string `json:"status"`
	Description        string `json:"description"`
}

func (r updateProductRequest) toInput() servicepkg.UpdateProductInput {
	price := r.Price
	if price == 0 {
		price = r.ListPrice
	}
	return servicepkg.UpdateProductInput{
		ProductID:          r.ProductID,
		Name:               r.Name,
		Type:               r.Type,
		PlanCode:           r.PlanCode,
		Period:             r.Period,
		ValidDays:          r.ValidDays,
		RelayTrafficGB:     r.RelayTrafficGB,
		RelayBandwidthMbps: r.RelayBandwidthMbps,
		Price:              price,
		SalePrice:          r.SalePrice,
		Currency:           r.Currency,
		AutoRenew:          r.AutoRenew,
		Status:             r.Status,
		Description:        r.Description,
	}
}

type createOrderRequest struct {
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

func (r createOrderRequest) toInput() servicepkg.CreateOrderInput {
	return servicepkg.CreateOrderInput{
		CustomerID:      r.CustomerID,
		CustomerEmail:   r.CustomerEmail,
		ProductID:       r.ProductID,
		ProductName:     r.ProductName,
		ProductType:     r.ProductType,
		Amount:          r.Amount,
		Currency:        r.Currency,
		Status:          r.Status,
		PayStatus:       r.PayStatus,
		ProvisionStatus: r.ProvisionStatus,
		Channel:         r.Channel,
		PaidAt:          r.PaidAt,
		ValidUntil:      r.ValidUntil,
	}
}

type updateOrderRequest struct {
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

func (r updateOrderRequest) toInput() servicepkg.UpdateOrderInput {
	return servicepkg.UpdateOrderInput{
		OrderID:         r.OrderID,
		CustomerID:      r.CustomerID,
		CustomerEmail:   r.CustomerEmail,
		ProductID:       r.ProductID,
		ProductName:     r.ProductName,
		ProductType:     r.ProductType,
		Status:          r.Status,
		Amount:          r.Amount,
		Currency:        r.Currency,
		PayStatus:       r.PayStatus,
		ProvisionStatus: r.ProvisionStatus,
		Channel:         r.Channel,
		PaidAt:          r.PaidAt,
		ValidUntil:      r.ValidUntil,
	}
}

type updateRenewalRequest struct {
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

func (r updateRenewalRequest) toInput() servicepkg.UpdateRenewalInput {
	return servicepkg.UpdateRenewalInput{
		RenewalID:     r.RenewalID,
		OrderID:       r.OrderID,
		CustomerID:    r.CustomerID,
		CustomerEmail: r.CustomerEmail,
		PlanCode:      r.PlanCode,
		Period:        r.Period,
		Amount:        r.Amount,
		Status:        r.Status,
		RenewAt:       r.RenewAt,
		PaidAt:        r.PaidAt,
		Source:        r.Source,
		Operator:      r.Operator,
	}
}

type upsertClientDownloadRequest struct {
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

func (r upsertClientDownloadRequest) toInput() servicepkg.UpsertClientDownloadInput {
	return servicepkg.UpsertClientDownloadInput{
		DownloadID:   r.DownloadID,
		Name:         r.Name,
		Platform:     r.Platform,
		Version:      r.Version,
		Arch:         r.Arch,
		Channel:      r.Channel,
		URL:          r.URL,
		SHA256:       r.SHA256,
		ReleaseNotes: r.ReleaseNotes,
		Status:       r.Status,
	}
}
