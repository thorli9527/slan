package service

type UpsertUserAliasInput struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
}
