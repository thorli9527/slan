package service

type CreateConsoleLoginKeyInput struct {
	UserID string `json:"userId"`
}

type ConsoleLoginInput struct {
	Key      string `json:"key"`
	LoginKey string `json:"loginKey"`
}
