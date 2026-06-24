package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildOpsCustomerViews(
	ctx context.Context,
	users []model.User,
	catalog repository.OpsRepository,
	devices repository.DeviceRepository,
	planExpires map[string]int64,
) []OpsCustomerView {
	items := make([]OpsCustomerView, 0, len(users))
	for _, user := range users {
		items = append(items, buildOpsCustomerView(ctx, catalog, devices, opsCustomerFromUser(ctx, catalog, user), planExpires[user.UserID]))
	}
	return items
}

func buildOpsCustomerView(ctx context.Context, _ repository.OpsRepository, devices repository.DeviceRepository, customer model.Customer, planExpiresAt int64) OpsCustomerView {
	view := OpsCustomerView{Customer: customerView(customer)}
	ownedDevices, err := devices.ListDevicesByOwner(ctx, customer.CustomerID)
	if err == nil {
		view.OwnDevices = len(ownedDevices)
		var totalBytes int64
		for _, device := range ownedDevices {
			totalBytes += device.RXBytesTotal + device.TXBytesTotal
		}
		view.RelayUsedGB = int(totalBytes / (1024 * 1024 * 1024))
	}
	view.PlanExpiresAt = planExpiresAt
	return view
}

func customerView(item model.Customer) CustomerView {
	return CustomerView{
		CustomerID: item.CustomerID,
		Email:      item.Email,
		Name:       item.Name,
		Country:    item.Country,
		Province:   item.Province,
		City:       item.City,
		IPRegion:   item.IPRegion,
		PlanCode:   item.PlanCode,
		Status:     item.Status,
		UpdatedAt:  item.UpdatedAt,
	}
}

func latestRenewalExpiryByCustomer(items []model.Renewal) map[string]int64 {
	out := make(map[string]int64, len(items))
	for _, item := range items {
		if item.CustomerID == "" || item.RenewAt == 0 {
			continue
		}
		if item.RenewAt > out[item.CustomerID] {
			out[item.CustomerID] = item.RenewAt
		}
	}
	return out
}
