package service

type OpsLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	RemoteIP string `json:"-"`
}

type CreateOperatorInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type UpdateOperatorInput struct {
	OperatorID string `json:"operatorId"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     string `json:"status"`
}

type SetOperatorPasswordInput struct {
	OperatorID string `json:"operatorId"`
	Password   string `json:"password"`
}

type OpsChangePasswordInput struct {
	OperatorID  string `json:"operatorId"`
	OldPassword string `json:"oldPassword"`
	Password    string `json:"password"`
}
