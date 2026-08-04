package service

type CustomerView struct {
	CustomerID string `json:"customerId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	Status     string `json:"status"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type OpsCustomerView struct {
	Customer    CustomerView `json:"customer"`
	RelayUsedGB int          `json:"relayUsedGb"`
}
