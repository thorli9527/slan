package service

type ConsoleLoginKeyView struct {
	KeyID     string `json:"keyId"`
	UserID    string `json:"userId"`
	Key       string `json:"key"`
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expiresAt"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
