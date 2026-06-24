package service

import "strings"

func normalizeUpsertClientDownloadInput(input UpsertClientDownloadInput) UpsertClientDownloadInput {
	input.DownloadID = strings.TrimSpace(input.DownloadID)
	input.Name = strings.TrimSpace(input.Name)
	input.Platform = strings.TrimSpace(input.Platform)
	input.Version = strings.TrimSpace(input.Version)
	input.Arch = strings.TrimSpace(input.Arch)
	input.Channel = strings.TrimSpace(input.Channel)
	input.URL = strings.TrimSpace(input.URL)
	input.SHA256 = strings.TrimSpace(input.SHA256)
	input.ReleaseNotes = strings.TrimSpace(input.ReleaseNotes)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeUpsertPlanInput(input UpsertPlanInput) UpsertPlanInput {
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeCreateProductInput(input CreateProductInput) CreateProductInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.TrimSpace(input.Type)
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.Period = strings.TrimSpace(input.Period)
	input.Currency = strings.TrimSpace(input.Currency)
	input.Status = strings.TrimSpace(input.Status)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateProductInput(input UpdateProductInput) UpdateProductInput {
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.TrimSpace(input.Type)
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.Period = strings.TrimSpace(input.Period)
	input.Currency = strings.TrimSpace(input.Currency)
	input.Status = strings.TrimSpace(input.Status)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeCreateOrderInput(input CreateOrderInput) CreateOrderInput {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerEmail = strings.TrimSpace(input.CustomerEmail)
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.ProductName = strings.TrimSpace(input.ProductName)
	input.ProductType = strings.TrimSpace(input.ProductType)
	input.Currency = strings.TrimSpace(input.Currency)
	input.Status = strings.TrimSpace(input.Status)
	input.PayStatus = strings.TrimSpace(input.PayStatus)
	input.ProvisionStatus = strings.TrimSpace(input.ProvisionStatus)
	input.Channel = strings.TrimSpace(input.Channel)
	return input
}

func normalizeUpdateOrderInput(input UpdateOrderInput) UpdateOrderInput {
	input.OrderID = strings.TrimSpace(input.OrderID)
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerEmail = strings.TrimSpace(input.CustomerEmail)
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.ProductName = strings.TrimSpace(input.ProductName)
	input.ProductType = strings.TrimSpace(input.ProductType)
	input.Status = strings.TrimSpace(input.Status)
	input.Currency = strings.TrimSpace(input.Currency)
	input.PayStatus = strings.TrimSpace(input.PayStatus)
	input.ProvisionStatus = strings.TrimSpace(input.ProvisionStatus)
	input.Channel = strings.TrimSpace(input.Channel)
	return input
}

func normalizeUpdateRenewalInput(input UpdateRenewalInput) UpdateRenewalInput {
	input.RenewalID = strings.TrimSpace(input.RenewalID)
	input.OrderID = strings.TrimSpace(input.OrderID)
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerEmail = strings.TrimSpace(input.CustomerEmail)
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.Period = strings.TrimSpace(input.Period)
	input.Status = strings.TrimSpace(input.Status)
	input.Source = strings.TrimSpace(input.Source)
	input.Operator = strings.TrimSpace(input.Operator)
	return input
}

func normalizeDownloadID(downloadID string) string {
	return strings.TrimSpace(downloadID)
}
