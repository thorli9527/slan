package service

import "github.com/slan/service-biz/internal/model"

func applyUpdateCustomerInput(customer model.Customer, input UpdateCustomerInput, now int64) model.Customer {
	if input.Email != "" {
		customer.Email = input.Email
	}
	if input.Name != "" {
		customer.Name = input.Name
	}
	customer.Country = input.Country
	customer.Province = input.Province
	customer.City = input.City
	customer.IPRegion = input.IPRegion
	if input.Status != "" {
		customer.Status = input.Status
	}
	customer.UpdatedAt = now
	return customer
}
