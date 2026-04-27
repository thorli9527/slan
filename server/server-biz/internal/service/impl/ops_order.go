package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

var allowedPurchaseOrderStatuses = map[string]struct{}{
	"pending":   {},
	"paid":      {},
	"active":    {},
	"completed": {},
	"cancelled": {},
	"refunded":  {},
}

func (s dbOpsService) ListPurchaseOrders() ([]dto.OpsPurchaseOrder, error) {
	ctx := context.Background()
	orders, err := s.state.pg.ListPurchaseOrders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.OpsPurchaseOrder, 0, len(orders))
	for _, order := range orders {
		out = append(out, s.opsPurchaseOrderToDTO(ctx, order))
	}
	return out, nil
}

func (s dbOpsService) CreatePaidPurchaseOrder(req dto.OpsCreatePaidOrderRequest) (dto.OpsPurchaseOrder, error) {
	ctx := context.Background()
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return dto.OpsPurchaseOrder{}, fmt.Errorf("%w: userId is required", ErrInvalidArgument)
	}
	user, err := s.state.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsPurchaseOrder{}, ErrNotFound
		}
		return dto.OpsPurchaseOrder{}, err
	}
	code := strings.TrimSpace(strings.ToLower(req.ProductCode))
	if code == "" {
		return dto.OpsPurchaseOrder{}, fmt.Errorf("%w: productCode is required", ErrInvalidArgument)
	}
	product, err := s.state.pg.GetProductByCode(ctx, code)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsPurchaseOrder{}, ErrNotFound
		}
		return dto.OpsPurchaseOrder{}, err
	}
	if product.Status != "active" || product.ProductType == "plan" {
		return dto.OpsPurchaseOrder{}, fmt.Errorf("%w: product is not purchasable", ErrInvalidArgument)
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	months := req.Months
	if months <= 0 {
		months = 1
	}
	if product.ProductType == "addon_dns" {
		quantity = 1
	}
	now := time.Now().Unix()
	expiresBase := now
	if product.ProductType == "addon_dns" {
		activeExpiresAt, err := s.state.activeProductExpiresAt(ctx, user.UserID, product.ProductCode, "")
		if err != nil {
			return dto.OpsPurchaseOrder{}, err
		}
		if activeExpiresAt > expiresBase {
			expiresBase = activeExpiresAt
		}
	}
	expiresAt := int64(0)
	if product.ProductType == "addon_dns" {
		expiresAt = purchaseOrderExpiresAtForMonths(expiresBase, product.BillingCycle, months)
	}
	order := repo.PurchaseOrder{
		OrderID:      util.NewID("order"),
		UserID:       user.UserID,
		UserEmail:    user.Email,
		MerchantID:   valueOrDefault(product.MerchantID, defaultMerchantID),
		MerchantName: valueOrDefault(product.MerchantName, defaultMerchantName),
		ProductID:    product.ProductID,
		ProductCode:  product.ProductCode,
		ProductName:  product.ProductName,
		ProductType:  product.ProductType,
		Quantity:     quantity,
		Months:       months,
		UnitCents:    product.PriceCents,
		AmountCents:  product.PriceCents * int64(quantity) * int64(months),
		Currency:     product.Currency,
		BillingCycle: product.BillingCycle,
		Status:       "paid",
		PaidAt:       now,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.state.pg.CreatePurchaseOrder(ctx, order); err != nil {
		return dto.OpsPurchaseOrder{}, err
	}
	s.state.syncOrderUserEntitlement(ctx, order, "ops paid order created")
	return s.opsPurchaseOrderToDTO(ctx, order), nil
}

func (s dbOpsService) UpdatePurchaseOrderStatus(orderID string, req dto.UpdatePurchaseOrderStatusRequest) (dto.OpsPurchaseOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return dto.OpsPurchaseOrder{}, fmt.Errorf("%w: orderId is required", ErrInvalidArgument)
	}
	status := normalizePurchaseOrderStatus(req.Status)
	if _, ok := allowedPurchaseOrderStatuses[status]; !ok {
		return dto.OpsPurchaseOrder{}, fmt.Errorf("%w: unsupported order status", ErrInvalidArgument)
	}
	ctx := context.Background()
	if err := s.state.pg.UpdatePurchaseOrderStatus(ctx, orderID, status, time.Now().Unix()); err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsPurchaseOrder{}, ErrNotFound
		}
		return dto.OpsPurchaseOrder{}, err
	}
	order, err := s.state.pg.GetPurchaseOrderByID(ctx, orderID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsPurchaseOrder{}, ErrNotFound
		}
		return dto.OpsPurchaseOrder{}, err
	}
	if isPaidPurchaseOrderStatus(status) && order.ExpiresAt == 0 {
		if order.ProductType == "addon_dns" {
			expiresBase := time.Now().Unix()
			activeExpiresAt, err := s.state.activeProductExpiresAt(ctx, order.UserID, order.ProductCode, order.OrderID)
			if err != nil {
				return dto.OpsPurchaseOrder{}, err
			}
			if activeExpiresAt > expiresBase {
				expiresBase = activeExpiresAt
			}
			order.ExpiresAt = purchaseOrderExpiresAtForMonths(expiresBase, order.BillingCycle, maxInt(order.Months, 1))
		}
		if order.ExpiresAt > 0 {
			if err := s.state.pg.UpdatePurchaseOrderExpiresAt(ctx, orderID, order.ExpiresAt); err != nil {
				return dto.OpsPurchaseOrder{}, err
			}
		}
	}
	s.state.syncOrderUserEntitlement(ctx, order, "purchase order status changed")
	return s.opsPurchaseOrderToDTO(ctx, order), nil
}

func normalizePurchaseOrderStatus(status string) string {
	value := strings.TrimSpace(strings.ToLower(status))
	if value == "canceled" {
		return "cancelled"
	}
	return value
}

func (s dbOpsService) opsPurchaseOrderToDTO(ctx context.Context, order repo.PurchaseOrder) dto.OpsPurchaseOrder {
	userEmail := order.UserEmail
	if user, err := s.state.pg.GetUserByID(ctx, order.UserID); err == nil {
		userEmail = user.Email
	}
	expiresAt := purchaseOrderExpiresAt(order)
	if strings.EqualFold(strings.TrimSpace(order.ProductType), "addon_device") {
		if boundExpiresAt, err := s.state.pg.MaxPurchaseOrderDeviceBindingExpiresAt(ctx, order.OrderID); err == nil {
			expiresAt = boundExpiresAt
		}
	}
	return dto.OpsPurchaseOrder{
		OrderID:      order.OrderID,
		UserID:       order.UserID,
		UserEmail:    userEmail,
		MerchantID:   order.MerchantID,
		MerchantName: order.MerchantName,
		ProductID:    order.ProductID,
		ProductCode:  order.ProductCode,
		ProductName:  order.ProductName,
		ProductType:  order.ProductType,
		Quantity:     order.Quantity,
		Months:       maxInt(order.Months, 1),
		UnitCents:    order.UnitCents,
		AmountCents:  order.AmountCents,
		Currency:     order.Currency,
		BillingCycle: order.BillingCycle,
		Status:       order.Status,
		PaidAt:       order.PaidAt,
		CancelledAt:  order.CancelledAt,
		RefundedAt:   order.RefundedAt,
		ExpiresAt:    expiresAt,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
}
