package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s OpsDashboardService) Dashboard(ctx context.Context) (OpsDashboardView, error) {
	customers, _ := s.Customers.ListCustomers(ctx)
	operators, _ := s.Operators.ListOperators(ctx)
	networkCount := 0
	devices := 0
	if items, err := s.Inventory.ListAllDevices(ctx); err == nil {
		devices = len(items)
	}
	if lister, ok := s.Networks.(interface {
		ListNetworks(context.Context) ([]model.Network, error)
	}); ok {
		if networks, err := lister.ListNetworks(ctx); err == nil {
			networkCount = len(networks)
		}
	}
	return OpsDashboardView{
		Customers: len(customers),
		Devices:   devices,
		Networks:  networkCount,
		Operators: len(operators),
	}, nil
}
