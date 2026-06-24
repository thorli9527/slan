package model

type AuditEvent struct {
	EventID      string `json:"eventId"`
	ActorType    string `json:"actorType"`
	ActorID      string `json:"actorId"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}
