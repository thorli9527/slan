package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func enrichOrder(ctx context.Context, users repository.UserRepository, catalog repository.OpsRepository, item model.Order) model.Order {
	if item.CustomerEmail == "" && item.CustomerID != "" {
		if customer, err := requireOpsUser(ctx, users, item.CustomerID); err == nil {
			item.CustomerEmail = customer.Email
		}
	}
	if item.ProductID != "" {
		if product, err := requireOpsProduct(ctx, catalog, item.ProductID); err == nil {
			if item.ProductName == "" {
				item.ProductName = product.Name
			}
			if item.ProductType == "" {
				item.ProductType = product.Type
			}
			if item.Currency == "" {
				item.Currency = product.Currency
			}
			if item.Amount == 0 {
				item.Amount = firstNonZero(product.SalePrice, product.Price)
			}
		}
	}
	return item
}

func enrichRenewal(ctx context.Context, catalog repository.OpsRepository, item model.Renewal) model.Renewal {
	if item.OrderID != "" {
		if order, err := requireOpsOrder(ctx, catalog, item.OrderID); err == nil {
			if item.CustomerID == "" {
				item.CustomerID = order.CustomerID
			}
			if item.CustomerEmail == "" {
				item.CustomerEmail = order.CustomerEmail
			}
			if item.Amount == 0 {
				item.Amount = order.Amount
			}
			if item.RenewAt == 0 {
				item.RenewAt = order.ValidUntil
			}
			if item.PlanCode == "" && order.ProductID != "" {
				if product, err := requireOpsProduct(ctx, catalog, order.ProductID); err == nil {
					item.PlanCode = product.PlanCode
					if item.Period == "" {
						item.Period = product.Period
					}
				}
			}
		}
	}
	if item.Source == "" {
		item.Source = "manual"
	}
	if item.Operator == "" {
		item.Operator = "ops"
	}
	return item
}
