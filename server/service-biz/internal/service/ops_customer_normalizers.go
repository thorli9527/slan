package service

import "strings"

func normalizeCreateCustomerInput(input CreateCustomerInput) CreateCustomerInput {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	input.Country = strings.TrimSpace(input.Country)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	input.IPRegion = strings.TrimSpace(input.IPRegion)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeUpdateCustomerInput(input UpdateCustomerInput) UpdateCustomerInput {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	input.Country = strings.TrimSpace(input.Country)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	input.IPRegion = strings.TrimSpace(input.IPRegion)
	input.Status = strings.TrimSpace(input.Status)
	return input
}
