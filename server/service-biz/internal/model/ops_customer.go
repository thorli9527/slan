package model

type Customer struct {
	CustomerID string `json:"customerId"`
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	Status     string `json:"status"`
	PlanCode   string `json:"planCode,omitempty"`
	UpdatedAt  int64  `json:"updatedAt"`
}
