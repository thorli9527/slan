package model

type PublicMapping struct {
	MappingID    string `json:"mappingId"`
	NetworkID    string `json:"networkId"`
	Name         string `json:"name"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	DeviceID     string `json:"deviceId"`
	Protocol     string `json:"protocol"`
	InternalIP   string `json:"internalIp"`
	InternalPort int    `json:"internalPort"`
	ExternalPort int    `json:"externalPort"`
	AccessMode   string `json:"accessMode"`
	TLSMode      string `json:"tlsMode"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}
