package service

import "github.com/slan/service-biz/internal/model"

func newOrder(id string, now int64, customer model.User, product model.Product, input CreateOrderInput) model.Order {
	item := model.Order{
		OrderID:         id,
		CustomerID:      input.CustomerID,
		CustomerEmail:   firstNonEmpty(input.CustomerEmail, customer.Email),
		ProductID:       input.ProductID,
		ProductName:     firstNonEmpty(input.ProductName, product.Name),
		ProductType:     firstNonEmpty(input.ProductType, product.Type),
		Status:          firstNonEmpty(input.Status, "pending"),
		Amount:          input.Amount,
		Currency:        firstNonEmpty(input.Currency, product.Currency, "CNY"),
		PayStatus:       firstNonEmpty(input.PayStatus, firstNonEmpty(input.Status, "pending")),
		ProvisionStatus: firstNonEmpty(input.ProvisionStatus, "pending"),
		Channel:         firstNonEmpty(input.Channel, "manual"),
		PaidAt:          input.PaidAt,
		ValidUntil:      input.ValidUntil,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if item.Amount == 0 {
		item.Amount = firstNonZero(product.SalePrice, product.Price)
	}
	return item
}

func applyUpdateOrderInput(item model.Order, input UpdateOrderInput, now int64) model.Order {
	if v := input.Status; v != "" {
		item.Status = v
	}
	if input.Amount > 0 {
		item.Amount = input.Amount
	}
	if v := input.CustomerID; v != "" {
		item.CustomerID = v
	}
	if v := input.CustomerEmail; v != "" {
		item.CustomerEmail = v
	}
	if v := input.ProductID; v != "" {
		item.ProductID = v
	}
	if v := input.ProductName; v != "" {
		item.ProductName = v
	}
	if v := input.ProductType; v != "" {
		item.ProductType = v
	}
	if v := input.Currency; v != "" {
		item.Currency = v
	}
	if v := input.PayStatus; v != "" {
		item.PayStatus = v
	}
	if v := input.ProvisionStatus; v != "" {
		item.ProvisionStatus = v
	}
	if v := input.Channel; v != "" {
		item.Channel = v
	}
	if input.PaidAt > 0 {
		item.PaidAt = input.PaidAt
	}
	if input.ValidUntil > 0 {
		item.ValidUntil = input.ValidUntil
	}
	item.UpdatedAt = now
	return item
}

func newOrPendingRenewal(id string, now int64, current model.Renewal, exists bool) model.Renewal {
	if exists {
		return current
	}
	return model.Renewal{
		RenewalID: id,
		Status:    "pending",
		UpdatedAt: now,
	}
}

func applyUpdateRenewalInput(item model.Renewal, input UpdateRenewalInput, now int64) model.Renewal {
	if v := input.OrderID; v != "" {
		item.OrderID = v
	}
	if v := input.CustomerID; v != "" {
		item.CustomerID = v
	}
	if v := input.CustomerEmail; v != "" {
		item.CustomerEmail = v
	}
	if v := input.PlanCode; v != "" {
		item.PlanCode = v
	}
	if v := input.Period; v != "" {
		item.Period = v
	}
	if input.Amount > 0 {
		item.Amount = input.Amount
	}
	if v := input.Status; v != "" {
		item.Status = v
	}
	if input.RenewAt > 0 {
		item.RenewAt = input.RenewAt
	}
	if input.PaidAt > 0 {
		item.PaidAt = input.PaidAt
	}
	if v := input.Source; v != "" {
		item.Source = v
	}
	if v := input.Operator; v != "" {
		item.Operator = v
	}
	item.UpdatedAt = now
	return item
}
