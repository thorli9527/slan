package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type OpsCatalogRepository interface {
	GetCustomerPlan(ctx context.Context, customerID string) (string, bool, error)
	SaveCustomerPlan(ctx context.Context, customerID, planCode string) error
	ListClientDownloads(ctx context.Context) ([]model.ClientDownload, error)
	GetClientDownload(ctx context.Context, downloadID string) (model.ClientDownload, bool, error)
	SaveClientDownload(ctx context.Context, item model.ClientDownload) error
	DeleteClientDownload(ctx context.Context, downloadID string) error
	ListPlans(ctx context.Context) ([]model.Plan, error)
	GetPlan(ctx context.Context, planCode string) (model.Plan, bool, error)
	SavePlan(ctx context.Context, item model.Plan) error
	ListProducts(ctx context.Context) ([]model.Product, error)
	GetProduct(ctx context.Context, productID string) (model.Product, bool, error)
	SaveProduct(ctx context.Context, item model.Product) error
	ListOrders(ctx context.Context) ([]model.Order, error)
	GetOrder(ctx context.Context, orderID string) (model.Order, bool, error)
	SaveOrder(ctx context.Context, item model.Order) error
	ListRenewals(ctx context.Context) ([]model.Renewal, error)
	GetRenewal(ctx context.Context, renewalID string) (model.Renewal, bool, error)
	SaveRenewal(ctx context.Context, item model.Renewal) error
	DeleteRenewal(ctx context.Context, renewalID string) error
}
