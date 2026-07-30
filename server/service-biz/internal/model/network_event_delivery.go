package model

type NetworkEventDelivery struct {
	EventID        string
	TargetDeviceID string
	NetworkID      string
	EventType      string
	ConfigVersion  int64
	Payload        string
	Status         string
	Attempts       int
	NextRetryAt    int64
	ExpiresAt      int64
	AcknowledgedAt int64
	CreatedAt      int64
	UpdatedAt      int64
}
