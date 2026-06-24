package service

type UserAliasView struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Alias     string `json:"alias"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
