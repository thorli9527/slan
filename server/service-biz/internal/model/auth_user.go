package model

type User struct {
	UserID       string `json:"userId"`
	Email        string `json:"email"`
	Name         string `json:"name,omitempty"`
	Country      string `json:"country,omitempty"`
	Province     string `json:"province,omitempty"`
	City         string `json:"city,omitempty"`
	IPRegion     string `json:"ipRegion,omitempty"`
	PasswordHash string `json:"-"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type UserAlias struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Alias     string `json:"alias"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
