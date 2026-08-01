package service

type UpdateUserInput struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	IPRegion string `json:"ipRegion"`
	Status   string `json:"status"`
}

type CreateUserInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type SetUserPasswordInput struct {
	UserID   string `json:"userId"`
	Password string `json:"password"`
}
