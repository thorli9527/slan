package service

type UpdateCustomerInput struct {
	CustomerID string `json:"customerId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	Status     string `json:"status"`
}

type AssignCustomerPlanInput struct {
	CustomerID string `json:"customerId"`
	PlanCode   string `json:"planCode"`
	ExpiresAt  int64  `json:"expiresAt"`
	Amount     int64  `json:"amount"`
	Period     string `json:"period"`
}
