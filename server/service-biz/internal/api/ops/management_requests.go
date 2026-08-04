package ops

import "strconv"

import servicepkg "github.com/slan/service-biz/internal/service"

type upsertNodeRequest struct {
	NodeID           string `json:"nodeId"`
	Name             string `json:"name"`
	Region           string `json:"region"`
	Endpoint         string `json:"endpoint"`
	Transport        string `json:"transport"`
	PublicAddr       string `json:"publicAddr"`
	PublicUDPIP      string `json:"publicUdpIp"`
	PublicUDPPort    int    `json:"publicUdpPort"`
	MaxBandwidthMbps int    `json:"maxBandwidthMbps"`
	MonthlyTrafficGB int    `json:"monthlyTrafficGb"`
	UsedTrafficGB    int    `json:"usedTrafficGb"`
	MaxSessions      int    `json:"maxSessions"`
	ActiveSessions   int    `json:"activeSessions"`
	Status           string `json:"status"`
	Health           string `json:"health"`
	Priority         int    `json:"priority"`
}

func (r upsertNodeRequest) toInput() servicepkg.UpsertNodeInput {
	endpoint := r.Endpoint
	if endpoint == "" {
		if r.PublicAddr != "" {
			endpoint = r.PublicAddr
		} else if r.PublicUDPIP != "" && r.PublicUDPPort > 0 {
			endpoint = r.PublicUDPIP + ":" + itoa(r.PublicUDPPort)
		}
	}
	return servicepkg.UpsertNodeInput{
		NodeID:           r.NodeID,
		Name:             r.Name,
		Region:           r.Region,
		Endpoint:         endpoint,
		Transport:        r.Transport,
		MaxBandwidthMbps: r.MaxBandwidthMbps,
		MonthlyTrafficGB: r.MonthlyTrafficGB,
		UsedTrafficGB:    r.UsedTrafficGB,
		MaxSessions:      r.MaxSessions,
		ActiveSessions:   r.ActiveSessions,
		Status:           r.Status,
		Health:           r.Health,
		Priority:         r.Priority,
	}
}

type updateNodeStatusRequest struct {
	Status  string `json:"status"`
	Enabled *bool  `json:"enabled"`
}

func (r updateNodeStatusRequest) normalizedStatus() string {
	if r.Status != "" {
		return r.Status
	}
	if r.Enabled == nil {
		return ""
	}
	if *r.Enabled {
		return "active"
	}
	return "disabled"
}

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

func itoa(v int) string {
	return strconv.Itoa(v)
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
