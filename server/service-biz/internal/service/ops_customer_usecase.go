package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func (s OpsCustomerService) ListCustomers(ctx context.Context) ([]OpsCustomerView, error) {
	customers, err := s.Customers.ListCustomers(ctx)
	if err != nil {
		return nil, err
	}
	return buildOpsCustomerViews(customers), nil
}

func (s OpsCustomerService) CreateCustomer(ctx context.Context, input CreateCustomerInput) (OpsCustomerView, error) {
	input = normalizeCreateCustomerInput(input)
	if input.Email == "" {
		return OpsCustomerView{}, ErrInvalidArgument
	}
	if _, exists, err := s.Customers.GetCustomerByEmail(ctx, input.Email); err != nil {
		return OpsCustomerView{}, err
	} else if exists {
		return OpsCustomerView{}, ErrConflict
	}
	now := opsNow(s.Now).Unix()
	customer := model.Customer{
		CustomerID: generatedID(s.NewCustomerID, "customer"),
		Email:      input.Email, Name: input.Name, Country: input.Country,
		Province: input.Province, City: input.City, IPRegion: input.IPRegion,
		Status: firstNonEmpty(input.Status, "active"), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Customers.SaveCustomer(ctx, customer); err != nil {
		if emailBelongsToAnotherCustomer(ctx, s.Customers, input.Email, customer.CustomerID) {
			return OpsCustomerView{}, ErrConflict
		}
		return OpsCustomerView{}, err
	}
	return buildOpsCustomerView(customer), nil
}

func (s OpsCustomerService) UpdateCustomer(ctx context.Context, input UpdateCustomerInput) (OpsCustomerView, error) {
	input = normalizeUpdateCustomerInput(input)
	if input.CustomerID == "" {
		return OpsCustomerView{}, ErrInvalidArgument
	}
	customer, err := requireOpsCustomer(ctx, s.Customers, input.CustomerID)
	if err != nil {
		return OpsCustomerView{}, err
	}
	if input.Email != "" && input.Email != customer.Email && emailBelongsToAnotherCustomer(ctx, s.Customers, input.Email, customer.CustomerID) {
		return OpsCustomerView{}, ErrConflict
	}
	customer = applyUpdateCustomerInput(customer, input, opsNow(s.Now).Unix())
	if err := s.Customers.SaveCustomer(ctx, customer); err != nil {
		if emailBelongsToAnotherCustomer(ctx, s.Customers, customer.Email, customer.CustomerID) {
			return OpsCustomerView{}, ErrConflict
		}
		return OpsCustomerView{}, err
	}
	return buildOpsCustomerView(customer), nil
}

func emailBelongsToAnotherCustomer(ctx context.Context, customers repository.CustomerRepository, email, customerID string) bool {
	item, found, err := customers.GetCustomerByEmail(ctx, email)
	return err == nil && found && item.CustomerID != customerID
}
