package service

import (
	"time"

	"github.com/slan/service-biz/internal/model"
)

func newConsoleLoginKey(newSessID func(string) string, now time.Time, userID string) (model.ConsoleLoginKey, error) {
	token, err := randomHex(12)
	if err != nil {
		return model.ConsoleLoginKey{}, err
	}
	return model.ConsoleLoginKey{
		KeyID:     newAuthSessionID(newSessID),
		UserID:    userID,
		Key:       token,
		Status:    "active",
		ExpiresAt: now.Add(10 * time.Minute).Unix(),
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}, nil
}

func markConsoleLoginKeyUsed(item model.ConsoleLoginKey, now int64) model.ConsoleLoginKey {
	item.Status = "used"
	item.UpdatedAt = now
	return item
}
