package service

import "context"

func (s OpsCatalogOrderService) ListOrders(ctx context.Context) ([]OrderView, error) {
	items, err := s.Catalog.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	return orderViews(items), nil
}

func (s OpsCatalogOrderService) CreateOrder(ctx context.Context, input CreateOrderInput) (OrderView, error) {
	input = normalizeCreateOrderInput(input)
	if input.CustomerID == "" || input.ProductID == "" {
		return OrderView{}, ErrInvalidArgument
	}
	customer, err := requireOpsUser(ctx, s.Users, input.CustomerID)
	if err != nil {
		return OrderView{}, err
	}
	product, err := requireOpsProduct(ctx, s.Catalog, input.ProductID)
	if err != nil {
		return OrderView{}, err
	}
	now := opsNow(s.Now).Unix()
	item := newOrder(opsOrderID(s.Catalog), now, customer, product, input)
	if err := s.Catalog.SaveOrder(ctx, item); err != nil {
		return OrderView{}, err
	}
	return orderView(item), nil
}

func (s OpsCatalogOrderService) UpdateOrder(ctx context.Context, input UpdateOrderInput) (OrderView, error) {
	input = normalizeUpdateOrderInput(input)
	if input.OrderID == "" {
		return OrderView{}, ErrInvalidArgument
	}
	item, err := requireOpsOrder(ctx, s.Catalog, input.OrderID)
	if err != nil {
		return OrderView{}, err
	}
	item = applyUpdateOrderInput(item, input, opsNow(s.Now).Unix())
	item = enrichOrder(ctx, s.Users, s.Catalog, item)
	if err := s.Catalog.SaveOrder(ctx, item); err != nil {
		return OrderView{}, err
	}
	return orderView(item), nil
}

func (s OpsCatalogOrderService) ListRenewals(ctx context.Context) ([]RenewalView, error) {
	items, err := s.Catalog.ListRenewals(ctx)
	if err != nil {
		return nil, err
	}
	return renewalViews(items), nil
}

func (s OpsCatalogOrderService) UpdateRenewal(ctx context.Context, input UpdateRenewalInput) (RenewalView, error) {
	input = normalizeUpdateRenewalInput(input)
	if input.RenewalID == "" {
		return RenewalView{}, ErrInvalidArgument
	}
	current, ok, err := s.Catalog.GetRenewal(ctx, input.RenewalID)
	if err != nil {
		return RenewalView{}, err
	}
	item := newOrPendingRenewal(input.RenewalID, opsNow(s.Now).Unix(), current, ok)
	item = applyUpdateRenewalInput(item, input, opsNow(s.Now).Unix())
	item = enrichRenewal(ctx, s.Catalog, item)
	if err := s.Catalog.SaveRenewal(ctx, item); err != nil {
		return RenewalView{}, err
	}
	return renewalView(item), nil
}
