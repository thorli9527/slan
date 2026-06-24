package service

type CustomerView struct {
	CustomerID string `json:"customerId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	PlanCode   string `json:"planCode"`
	Status     string `json:"status"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type OpsCustomerView struct {
	Customer       CustomerView `json:"customer"`
	OwnDevices     int          `json:"ownDevices"`
	InvitedDevices int          `json:"invitedDevices"`
	PlanExpiresAt  int64        `json:"planExpiresAt"`
	RelayUsedGB    int          `json:"relayUsedGb"`
}

type OpsCustomerPlanAssignmentView struct {
	Customer   OpsCustomerView `json:"customer"`
	PlanCode   string          `json:"planCode"`
	Period     string          `json:"period"`
	Amount     int64           `json:"amount"`
	PaidAt     int64           `json:"paidAt"`
	ValidUntil int64           `json:"validUntil"`
}
