package service

import "context"

type OpsCatalogDownloadUseCase interface {
	ListClientDownloads(ctx context.Context) ([]ClientDownloadView, error)
	UpsertClientDownload(ctx context.Context, input UpsertClientDownloadInput) (ClientDownloadView, error)
	DeleteClientDownload(ctx context.Context, downloadID string) error
}

type OpsCatalogPlanUseCase interface {
	ListPlans(ctx context.Context) ([]PlanView, error)
	UpsertPlan(ctx context.Context, input UpsertPlanInput) (PlanView, error)
}

type OpsCatalogProductUseCase interface {
	ListProducts(ctx context.Context) ([]ProductView, error)
	CreateProduct(ctx context.Context, input CreateProductInput) (ProductView, error)
	UpdateProduct(ctx context.Context, input UpdateProductInput) (ProductView, error)
}

type OpsCatalogOrderUseCase interface {
	ListOrders(ctx context.Context) ([]OrderView, error)
	CreateOrder(ctx context.Context, input CreateOrderInput) (OrderView, error)
	UpdateOrder(ctx context.Context, input UpdateOrderInput) (OrderView, error)
	ListRenewals(ctx context.Context) ([]RenewalView, error)
	UpdateRenewal(ctx context.Context, input UpdateRenewalInput) (RenewalView, error)
}
