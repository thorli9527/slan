package service

type CreateCustomerInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	IPRegion string `json:"ipRegion"`
	Status   string `json:"status"`
}

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
