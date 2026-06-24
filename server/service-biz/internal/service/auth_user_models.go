package service

type UserView struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type UserSessionView struct {
	SessionID    string `json:"sessionId"`
	UserID       string `json:"userId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Status       string `json:"status"`
	ExpiresAt    int64  `json:"expiresAt"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type AuthSessionView struct {
	User    UserView        `json:"user"`
	Session UserSessionView `json:"session"`
}

type UserSummaryView struct {
	User UserView `json:"user"`
}

type ChangedUserPasswordView struct {
	User      UserView `json:"user"`
	UpdatedAt int64    `json:"updatedAt"`
}

type UserEntitlementView struct {
	UserID      string `json:"userId"`
	PlanCode    string `json:"planCode"`
	DeviceLimit int    `json:"deviceLimit"`
	UsedDevices int    `json:"usedDevices"`
	Status      string `json:"status"`
}
