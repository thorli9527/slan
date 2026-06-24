package service

import "github.com/slan/service-biz/internal/model"

func newPlan(now int64, input UpsertPlanInput) model.Plan {
	return model.Plan{
		PlanCode:           input.PlanCode,
		Name:               input.Name,
		DeviceLimit:        input.DeviceLimit,
		InvitedDeviceLimit: input.InvitedDeviceLimit,
		TotalDeviceLimit:   input.TotalDeviceLimit,
		RelayMonthlyGB:     input.RelayMonthlyGB,
		RelayBandwidthMbps: input.RelayBandwidthMbps,
		RelayThrottleMbps:  input.RelayThrottleMbps,
		P2PUnlimited:       input.P2PUnlimited,
		CustomDomain:       input.CustomDomain,
		ACL:                input.ACL,
		DedicatedRelay:     input.DedicatedRelay,
		AuditLog:           input.AuditLog,
		APIAccess:          input.APIAccess,
		MonthlyPrice:       input.MonthlyPrice,
		YearlyPrice:        input.YearlyPrice,
		Status:             firstNonEmpty(input.Status, "active"),
		UpdatedAt:          now,
	}
}

func mergePlan(current model.Plan, input UpsertPlanInput, now int64) model.Plan {
	item := newPlan(now, input)
	if item.Name == "" {
		item.Name = current.Name
	}
	if item.DeviceLimit == 0 {
		item.DeviceLimit = current.DeviceLimit
	}
	if item.InvitedDeviceLimit == 0 {
		item.InvitedDeviceLimit = current.InvitedDeviceLimit
	}
	if item.TotalDeviceLimit == 0 {
		item.TotalDeviceLimit = current.TotalDeviceLimit
	}
	if item.RelayMonthlyGB == 0 {
		item.RelayMonthlyGB = current.RelayMonthlyGB
	}
	if item.RelayBandwidthMbps == 0 {
		item.RelayBandwidthMbps = current.RelayBandwidthMbps
	}
	if item.RelayThrottleMbps == 0 {
		item.RelayThrottleMbps = current.RelayThrottleMbps
	}
	if item.MonthlyPrice == 0 {
		item.MonthlyPrice = current.MonthlyPrice
	}
	if item.YearlyPrice == 0 {
		item.YearlyPrice = current.YearlyPrice
	}
	if input.Status == "" {
		item.Status = current.Status
	}
	if item.TotalDeviceLimit == 0 {
		item.TotalDeviceLimit = item.DeviceLimit + item.InvitedDeviceLimit
	}
	return item
}

func newProduct(id string, now int64, input CreateProductInput) model.Product {
	item := model.Product{
		ProductID:          id,
		Name:               input.Name,
		Type:               firstNonEmpty(input.Type, "plan"),
		PlanCode:           input.PlanCode,
		Period:             input.Period,
		ValidDays:          input.ValidDays,
		RelayTrafficGB:     input.RelayTrafficGB,
		RelayBandwidthMbps: input.RelayBandwidthMbps,
		Price:              input.Price,
		SalePrice:          input.SalePrice,
		Currency:           firstNonEmpty(input.Currency, "CNY"),
		AutoRenew:          input.AutoRenew,
		Status:             firstNonEmpty(input.Status, "active"),
		Description:        input.Description,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if item.SalePrice == 0 {
		item.SalePrice = item.Price
	}
	return item
}

func applyUpdateProductInput(item model.Product, input UpdateProductInput, now int64) model.Product {
	if v := input.Name; v != "" {
		item.Name = v
	}
	if v := input.Type; v != "" {
		item.Type = v
	}
	if v := input.PlanCode; v != "" {
		item.PlanCode = v
	}
	if v := input.Period; v != "" {
		item.Period = v
	}
	if input.ValidDays > 0 {
		item.ValidDays = input.ValidDays
	}
	if input.RelayTrafficGB > 0 {
		item.RelayTrafficGB = input.RelayTrafficGB
	}
	if input.RelayBandwidthMbps > 0 {
		item.RelayBandwidthMbps = input.RelayBandwidthMbps
	}
	if input.Price > 0 {
		item.Price = input.Price
	}
	if input.SalePrice > 0 {
		item.SalePrice = input.SalePrice
	}
	if v := input.Currency; v != "" {
		item.Currency = v
	}
	item.AutoRenew = input.AutoRenew
	if v := input.Status; v != "" {
		item.Status = v
	}
	if input.Description != "" || item.Description != "" {
		item.Description = input.Description
	}
	item.UpdatedAt = now
	return item
}
