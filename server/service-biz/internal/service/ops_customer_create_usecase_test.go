package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type customerCreateTestRepository struct {
	items map[string]model.Customer
}

func (r *customerCreateTestRepository) ListCustomers(context.Context) ([]model.Customer, error) {
	items := make([]model.Customer, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, nil
}

func (r *customerCreateTestRepository) GetCustomer(_ context.Context, customerID string) (model.Customer, bool, error) {
	item, ok := r.items[customerID]
	return item, ok, nil
}

func (r *customerCreateTestRepository) GetCustomerByEmail(_ context.Context, email string) (model.Customer, bool, error) {
	for _, item := range r.items {
		if item.Email == email {
			return item, true, nil
		}
	}
	return model.Customer{}, false, nil
}

func (r *customerCreateTestRepository) SaveCustomer(_ context.Context, customer model.Customer) error {
	if r.items == nil {
		r.items = make(map[string]model.Customer)
	}
	r.items[customer.CustomerID] = customer
	return nil
}

func TestCreateCustomerPersistsIndependentCustomer(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	repository := &customerCreateTestRepository{}
	service := OpsCustomerService{
		Customers:     repository,
		NewCustomerID: func() string { return "customer-000001" },
		Now:           func() time.Time { return now },
	}

	view, err := service.CreateCustomer(context.Background(), CreateCustomerInput{
		Email: " Customer@Example.COM ", Name: "Example Customer",
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	if view.Customer.CustomerID != "customer-000001" || view.Customer.Email != "customer@example.com" || view.Customer.Status != "active" {
		t.Fatalf("unexpected customer: %#v", view)
	}
	stored := repository.items[view.Customer.CustomerID]
	if stored.CreatedAt != now.Unix() || stored.UpdatedAt != now.Unix() {
		t.Fatalf("unexpected timestamps: %#v", stored)
	}

	if _, err := service.CreateCustomer(context.Background(), CreateCustomerInput{Email: "customer@example.com"}); err != ErrConflict {
		t.Fatalf("duplicate email error = %v, want %v", err, ErrConflict)
	}
}

func TestUpdateCustomerNormalizesAndRejectsDuplicateEmail(t *testing.T) {
	repository := &customerCreateTestRepository{items: map[string]model.Customer{
		"customer-1": {CustomerID: "customer-1", Email: "first@example.com", Status: "active"},
		"customer-2": {CustomerID: "customer-2", Email: "second@example.com", Status: "active"},
	}}
	service := OpsCustomerService{Customers: repository, Now: func() time.Time { return time.Unix(1_700_000_000, 0) }}

	if _, err := service.UpdateCustomer(context.Background(), UpdateCustomerInput{
		CustomerID: "customer-1", Email: " SECOND@EXAMPLE.COM ",
	}); err != ErrConflict {
		t.Fatalf("duplicate update error = %v, want %v", err, ErrConflict)
	}
	if repository.items["customer-1"].Email != "first@example.com" {
		t.Fatalf("duplicate update changed customer: %#v", repository.items["customer-1"])
	}
}
