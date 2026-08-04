package service

import "context"

type OpsCustomerUseCase interface {
	ListCustomers(ctx context.Context) ([]OpsCustomerView, error)
	CreateCustomer(ctx context.Context, input CreateCustomerInput) (OpsCustomerView, error)
	UpdateCustomer(ctx context.Context, input UpdateCustomerInput) (OpsCustomerView, error)
}
