package service

type RegisterUserInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginUserInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	SessionMode string `json:"sessionMode"`
}

type ChangeUserPasswordInput struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type LogoutUserInput struct {
	DeviceToken string `json:"deviceToken"`
}

type RenewUserSessionInput struct {
	RefreshToken string `json:"refreshToken"`
}
