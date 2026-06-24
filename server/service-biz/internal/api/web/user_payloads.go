package web

import servicepkg "github.com/slan/service-biz/internal/service"

func userPayload(item servicepkg.UserSummaryView) map[string]any {
	user := item.User
	return map[string]any{
		"userId":    user.UserID,
		"email":     user.Email,
		"name":      user.Name,
		"status":    user.Status,
		"createdAt": user.CreatedAt,
		"updatedAt": user.UpdatedAt,
	}
}

func userEntitlementPayload(view servicepkg.UserEntitlementView) map[string]any {
	return map[string]any{
		"userId":             view.UserID,
		"planCode":           view.PlanCode,
		"planName":           entitlementPlanName(view.PlanCode),
		"ownDeviceLimit":     view.DeviceLimit,
		"invitedDeviceLimit": 0,
		"totalDeviceLimit":   view.DeviceLimit,
		"ownDevices":         view.UsedDevices,
		"invitedDevices":     0,
		"totalDevices":       view.UsedDevices,
		"remainingDevices":   maxInt(view.DeviceLimit-view.UsedDevices, 0),
		"status":             view.Status,
	}
}

func entitlementPlanName(planCode string) string {
	switch planCode {
	case "free":
		return "免费版"
	case "pro":
		return "专业版"
	case "team":
		return "团队版"
	case "enterprise":
		return "企业版"
	default:
		if planCode == "" {
			return "免费版"
		}
		return planCode
	}
}

func changedUserPasswordPayload(item servicepkg.ChangedUserPasswordView) map[string]any {
	return map[string]any{
		"user":      userPayload(servicepkg.UserSummaryView{User: item.User}),
		"updatedAt": item.UpdatedAt,
	}
}

func userAliasPayload(view servicepkg.UserAliasView) map[string]any {
	return map[string]any{
		"ownerUserId": view.UserID,
		"userId":      view.UserID,
		"email":       view.Email,
		"alias":       view.Alias,
		"updatedAt":   view.UpdatedAt,
	}
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
