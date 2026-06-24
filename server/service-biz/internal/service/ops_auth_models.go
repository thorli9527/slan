package service

type OperatorView struct {
	OperatorID string `json:"operatorId"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type OperatorSessionView struct {
	SessionID   string `json:"sessionId"`
	OperatorID  string `json:"operatorId"`
	AccessToken string `json:"accessToken"`
	Status      string `json:"status"`
	ExpiresAt   int64  `json:"expiresAt"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type OpsSessionView struct {
	Operator OperatorView        `json:"operator"`
	Session  OperatorSessionView `json:"session"`
}

type OpsOperatorView struct {
	Operator    OperatorView `json:"operator"`
	LastLoginAt int64        `json:"lastLoginAt"`
}
