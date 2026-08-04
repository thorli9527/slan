package service

type CreateDNSZoneInput struct {
	NetworkID string `json:"networkId"`
	Name      string `json:"name"`
}

type UpdateDNSZoneInput struct {
	ZoneID string `json:"zoneId"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateDNSRecordInput struct {
	NetworkID string `json:"networkId"`
	ZoneID    string `json:"zoneId"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Port      string `json:"port"`
	TTL       int    `json:"ttl"`
}

type UpdateDNSRecordInput struct {
	RecordID string `json:"recordId"`
	ZoneID   string `json:"zoneId"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Port     string `json:"port"`
	TTL      int    `json:"ttl"`
}

type DeleteDNSZoneInput struct {
	ZoneID string `json:"zoneId"`
}

type DeleteDNSRecordInput struct {
	RecordID string `json:"recordId"`
}
