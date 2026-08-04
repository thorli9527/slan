package model

type Customer struct {
	CustomerID string `json:"customerId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	IPRegion   string `json:"ipRegion"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}
