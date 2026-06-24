package service

import "github.com/slan/service-biz/internal/model"

func deviceLoginDeviceView(item model.DeviceLoginDevice) DeviceLoginDeviceView {
	return DeviceLoginDeviceView{
		DeviceID:      item.DeviceID,
		UserID:        item.UserID,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		CountryCode:   item.CountryCode,
		VerifyCode:    item.VerifyCode,
		Status:        item.Status,
		ExpiresAt:     item.ExpiresAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func completedDeviceLoginView(item model.DeviceLoginDevice) CompleteDeviceLoginDeviceView {
	return CompleteDeviceLoginDeviceView{Device: deviceLoginDeviceView(item)}
}
