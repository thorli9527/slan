package model

type Device struct {
	DeviceID      string `json:"deviceId"`
	OwnerID       string `json:"ownerId"`
	VirtualIP     string `json:"virtualIp"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias,omitempty"`
	OSName        string `json:"osName,omitempty"`
	OSVersion     string `json:"osVersion,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`
	DeviceVersion string `json:"deviceVersion,omitempty"`
	CountryCode   string `json:"countryCode,omitempty"`
	RXBytesTotal  int64  `json:"rxBytesTotal"`
	TXBytesTotal  int64  `json:"txBytesTotal"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	LastSeenAt    int64  `json:"lastSeenAt,omitempty"`
}

const (
	DeviceRelationRoleOwner  = "owner"
	DeviceRelationRoleShared = "shared"

	DeviceRelationStatusActive  = "active"
	DeviceRelationStatusRevoked = "revoked"
)

// DeviceUserRelation owns user-level authorization for a device. Device
// lifecycle and runtime presence remain on Device and NetworkDevice.
type DeviceUserRelation struct {
	RelationID string `json:"relationId"`
	DeviceID   string `json:"deviceId"`
	UserID     string `json:"userId"`
	Alias      string `json:"alias,omitempty"`
	Role       string `json:"role"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId,omitempty"`
	Status     string `json:"status"`
	CreatedBy  string `json:"createdBy,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	RevokedBy  string `json:"revokedBy,omitempty"`
	RevokedAt  int64  `json:"revokedAt,omitempty"`
}
