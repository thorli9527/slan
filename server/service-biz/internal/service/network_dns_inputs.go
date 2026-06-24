package service

type CreateDNSZoneInput struct {
	NetworkID    string `json:"networkId"`
	ActorUserID  string `json:"actorUserId"`
	Name         string `json:"name"`
	ExposeGlobal bool   `json:"exposeGlobal"`
}

type UpdateDNSZoneInput struct {
	ZoneID       string `json:"zoneId"`
	ActorUserID  string `json:"actorUserId"`
	Name         string `json:"name"`
	ExposeGlobal *bool  `json:"exposeGlobal,omitempty"`
	Status       string `json:"status"`
}

type CreateDNSRecordInput struct {
	NetworkID   string `json:"networkId"`
	ActorUserID string `json:"actorUserId"`
	ZoneID      string `json:"zoneId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	Port        string `json:"port"`
	TTL         int    `json:"ttl"`
}

type UpdateDNSRecordInput struct {
	RecordID    string `json:"recordId"`
	ActorUserID string `json:"actorUserId"`
	ZoneID      string `json:"zoneId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	Port        string `json:"port"`
	TTL         int    `json:"ttl"`
}

type DeleteDNSZoneInput struct {
	ZoneID      string `json:"zoneId"`
	ActorUserID string `json:"actorUserId"`
}

type DeleteDNSRecordInput struct {
	RecordID    string `json:"recordId"`
	ActorUserID string `json:"actorUserId"`
}
