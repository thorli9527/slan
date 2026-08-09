package ops

import servicepkg "github.com/slan/service-biz/internal/service"

type updateCustomerRequest struct {
	CustomerID string `json:"customerId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	Status     string `json:"status"`
}

func (r updateCustomerRequest) toCreateInput() servicepkg.CreateCustomerInput {
	return servicepkg.CreateCustomerInput{
		Email: r.Email, Name: r.Name, Country: r.Country, Province: r.Province,
		City: r.City, IPRegion: r.IPRegion, Status: r.Status,
	}
}

func (r updateCustomerRequest) toInput() servicepkg.UpdateCustomerInput {
	return servicepkg.UpdateCustomerInput{
		CustomerID: r.CustomerID,
		Email:      r.Email,
		Name:       r.Name,
		Country:    r.Country,
		Province:   r.Province,
		City:       r.City,
		IPRegion:   r.IPRegion,
		Status:     r.Status,
	}
}

type updateManagedDeviceRequest struct {
	DeviceID  string `json:"deviceId"`
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	VirtualIP string `json:"virtualIp"`
	Status    string `json:"status"`
}

type createManagedDeviceRequest struct {
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	Platform  string `json:"platform"`
	OSName    string `json:"osName"`
	OSVersion string `json:"osVersion"`
	PublicKey string `json:"publicKey"`
}

func (r createManagedDeviceRequest) toInput() servicepkg.CreateOpsDeviceInput {
	return servicepkg.CreateOpsDeviceInput{Name: r.Name, Alias: r.Alias, Platform: r.Platform, OSName: r.OSName, OSVersion: r.OSVersion, PublicKey: r.PublicKey}
}

func (r updateManagedDeviceRequest) toInput() servicepkg.UpdateDeviceInput {
	return servicepkg.UpdateDeviceInput{
		DeviceID:  r.DeviceID,
		Name:      r.Name,
		Alias:     r.Alias,
		VirtualIP: r.VirtualIP,
		Status:    r.Status,
	}
}
