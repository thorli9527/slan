package service

type DNSZoneView struct {
	ZoneID    string `json:"zoneId"`
	NetworkID string `json:"networkId"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type DNSRecordView struct {
	RecordID       string `json:"recordId"`
	NetworkID      string `json:"networkId"`
	ZoneID         string `json:"zoneId"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Value          string `json:"value"`
	TTL            int    `json:"ttl"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	TargetDeviceID string `json:"targetDeviceId"`
	TargetIP       string `json:"targetIp"`
	CNAME          string `json:"cname"`
	Port           string `json:"port"`
}
