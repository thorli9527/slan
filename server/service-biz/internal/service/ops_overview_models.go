package service

type OpsDashboardView struct {
	Customers int `json:"customers"`
	Devices   int `json:"devices"`
	Networks  int `json:"networks"`
	Operators int `json:"operators"`
}

type OpsAuditEventView struct {
	EventID      string `json:"eventId"`
	ActorType    string `json:"actorType"`
	ActorID      string `json:"actorId"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Status       string `json:"status"`
	RemoteIP     string `json:"remoteIp,omitempty"`
	Detail       string `json:"detail,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
}
