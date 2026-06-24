package model

type DNSZone struct {
	ZoneID       string `json:"zoneId"`
	NetworkID    string `json:"networkId"`
	Name         string `json:"name"`
	ExposeGlobal bool   `json:"exposeGlobal"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type DNSRecord struct {
	RecordID  string `json:"recordId"`
	NetworkID string `json:"networkId"`
	ZoneID    string `json:"zoneId,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Port      string `json:"port,omitempty"`
	TTL       int    `json:"ttl"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
