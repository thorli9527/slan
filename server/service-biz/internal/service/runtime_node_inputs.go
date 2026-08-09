package service

type UpsertRuntimeNodeInput struct {
	NodeID           string `json:"nodeId"`
	Name             string `json:"name"`
	DERPRegionID     string `json:"-"`
	Endpoint         string `json:"endpoint"`
	Transport        string `json:"transport"`
	MaxBandwidthMbps int    `json:"maxBandwidthMbps"`
	MonthlyTrafficGB int    `json:"monthlyTrafficGb"`
	UsedTrafficGB    int    `json:"usedTrafficGb"`
	MaxSessions      int    `json:"maxSessions"`
	ActiveSessions   int    `json:"activeSessions"`
	Status           string `json:"status"`
	Health           string `json:"health"`
	Priority         int    `json:"priority"`
}
