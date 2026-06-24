package service

import "github.com/slan/service-biz/internal/model"

func userSummaryViews(items []model.User) []UserSummaryView {
	views := make([]UserSummaryView, 0, len(items))
	for _, item := range items {
		views = append(views, userSummaryView(item))
	}
	return views
}

func userEntitlementView(userID string, usedDevices int) UserEntitlementView {
	return UserEntitlementView{
		UserID:      userID,
		PlanCode:    "free",
		DeviceLimit: 10,
		UsedDevices: usedDevices,
		Status:      "active",
	}
}
