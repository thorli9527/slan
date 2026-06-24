package ops

import servicepkg "github.com/slan/service-biz/internal/service"

type opsLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r opsLoginRequest) toInput() servicepkg.OpsLoginInput {
	return servicepkg.OpsLoginInput{
		Email:    r.Email,
		Password: r.Password,
	}
}

type opsChangePasswordRequest struct {
	OperatorID  string `json:"operatorId"`
	Password    string `json:"password"`
	NewPassword string `json:"newPassword"`
}

func (r opsChangePasswordRequest) toInput() servicepkg.OpsChangePasswordInput {
	return servicepkg.OpsChangePasswordInput{
		OperatorID: r.OperatorID,
		Password:   firstNonEmpty(r.Password, r.NewPassword),
	}
}

type createOperatorRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (r createOperatorRequest) toInput() servicepkg.CreateOperatorInput {
	return servicepkg.CreateOperatorInput{
		Email:    r.Email,
		Name:     r.Name,
		Role:     r.Role,
		Password: r.Password,
	}
}

type updateOperatorRequest struct {
	OperatorID string `json:"operatorId"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     string `json:"status"`
}

func (r updateOperatorRequest) toInput() servicepkg.UpdateOperatorInput {
	return servicepkg.UpdateOperatorInput{
		OperatorID: r.OperatorID,
		Name:       r.Name,
		Role:       r.Role,
		Status:     r.Status,
	}
}

type setOperatorPasswordRequest struct {
	OperatorID string `json:"operatorId"`
	Password   string `json:"password"`
}

func (r setOperatorPasswordRequest) toInput() servicepkg.SetOperatorPasswordInput {
	return servicepkg.SetOperatorPasswordInput{
		OperatorID: r.OperatorID,
		Password:   r.Password,
	}
}
