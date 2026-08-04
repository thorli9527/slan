package service

import "github.com/slan/service-biz/internal/model"

func buildOpsCustomerViews(customers []model.Customer) []OpsCustomerView {
	items := make([]OpsCustomerView, 0, len(customers))
	for _, customer := range customers {
		items = append(items, buildOpsCustomerView(customer))
	}
	return items
}

func buildOpsCustomerView(customer model.Customer) OpsCustomerView {
	return OpsCustomerView{Customer: customerView(customer)}
}

func customerView(item model.Customer) CustomerView {
	return CustomerView{
		CustomerID: item.CustomerID,
		Email:      item.Email,
		Name:       item.Name,
		Country:    item.Country,
		Province:   item.Province,
		City:       item.City,
		IPRegion:   item.IPRegion,
		Status:     item.Status,
		UpdatedAt:  item.UpdatedAt,
	}
}
