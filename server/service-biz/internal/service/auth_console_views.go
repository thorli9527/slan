package service

import "github.com/slan/service-biz/internal/model"

func consoleLoginKeyView(item model.ConsoleLoginKey) ConsoleLoginKeyView {
	return ConsoleLoginKeyView{
		KeyID:     item.KeyID,
		UserID:    item.UserID,
		Key:       item.Key,
		Status:    item.Status,
		ExpiresAt: item.ExpiresAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
