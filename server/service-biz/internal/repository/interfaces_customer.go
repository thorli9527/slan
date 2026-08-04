package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type CustomerRepository interface {
	ListCustomers(ctx context.Context) ([]model.Customer, error)
	GetCustomer(ctx context.Context, customerID string) (model.Customer, bool, error)
	GetCustomerByEmail(ctx context.Context, email string) (model.Customer, bool, error)
	SaveCustomer(ctx context.Context, customer model.Customer) error
}
