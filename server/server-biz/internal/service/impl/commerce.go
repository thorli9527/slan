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

func (s dbNetworkService) ListPurchaseProducts(userID string) ([]dto.PurchaseProduct, error) {
	ctx := context.Background()
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUnauthorized
	}
	products, err := s.state.pg.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PurchaseProduct, 0, len(products))
	for _, product := range products {
		if product.Status != "active" || product.ProductType == "plan" {
			continue
		}
		out = append(out, purchaseProductToDTO(product))
	}
	return out, nil
}

func (s dbNetworkService) ListPurchaseOrders(userID string) ([]dto.PurchaseOrder, error) {
	ctx := context.Background()
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrUnauthorized
	}
	orders, err := s.state.pg.ListPurchaseOrdersByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PurchaseOrder, 0, len(orders))
	for _, order := range orders {
		out = append(out, s.state.purchaseOrderToDTO(ctx, order))
	}
	return out, nil
}

func (s dbNetworkService) CreatePurchaseOrder(userID string, req dto.CreatePurchaseOrderRequest) (dto.PurchaseOrder, error) {
	ctx := context.Background()
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return dto.PurchaseOrder{}, ErrUnauthorized
	}
	user, err := s.state.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.PurchaseOrder{}, ErrNotFound
		}
		return dto.PurchaseOrder{}, err
	}
	code := strings.TrimSpace(strings.ToLower(req.ProductCode))
	if code == "" {
		code = "extra-device"
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	months := req.Months
	if months <= 0 {
		months = 1
	}
	product, err := s.state.pg.GetProductByCode(ctx, code)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.PurchaseOrder{}, ErrNotFound
		}
		return dto.PurchaseOrder{}, err
	}
	if product.Status != "active" || product.ProductType == "plan" {
		return dto.PurchaseOrder{}, fmt.Errorf("%w: product is not purchasable", ErrInvalidArgument)
	}
	if product.ProductType == "addon_dns" {
		quantity = 1
	}
	now := time.Now().Unix()
	record := repo.PurchaseOrder{
		OrderID:      util.NewID("order"),
		UserID:       userID,
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
		Status:       "pending",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.state.pg.CreatePurchaseOrder(ctx, record); err != nil {
		return dto.PurchaseOrder{}, err
	}
	return s.state.purchaseOrderToDTO(ctx, record), nil
}

func (s dbNetworkService) GetProductEntitlement(userID, productCode string) (dto.ProductEntitlement, error) {
	ctx := context.Background()
	userID = strings.TrimSpace(userID)
	code := strings.TrimSpace(strings.ToLower(productCode))
	if userID == "" {
		return dto.ProductEntitlement{}, ErrUnauthorized
	}
	if code == "" {
		return dto.ProductEntitlement{}, fmt.Errorf("%w: productCode is required", ErrInvalidArgument)
	}
	product, err := s.state.pg.GetProductByCode(ctx, code)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.ProductEntitlement{}, ErrNotFound
		}
		return dto.ProductEntitlement{}, err
	}
	if product.Status != "active" {
		return dto.ProductEntitlement{ProductCode: code}, nil
	}
	entitlement, err := s.state.productEntitlement(ctx, userID, product)
	if err != nil {
		return dto.ProductEntitlement{}, err
	}
	return entitlement, nil
}

func (s *dbState) requireActiveProductEntitlement(ctx context.Context, userID, productCode string) error {
	code := strings.TrimSpace(strings.ToLower(productCode))
	product, err := s.pg.GetProductByCode(ctx, code)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	entitlement, err := s.productEntitlement(ctx, userID, product)
	if err != nil {
		return err
	}
	if !entitlement.Active {
		return fmt.Errorf("%w: product %s is required", ErrPaymentRequired, product.ProductCode)
	}
	return nil
}

func (s *dbState) productEntitlement(ctx context.Context, userID string, product repo.Product) (dto.ProductEntitlement, error) {
	now := time.Now().Unix()
	out := dto.ProductEntitlement{ProductCode: product.ProductCode}
	orders, err := s.pg.ListPurchaseOrdersByUserProduct(ctx, userID, product.ProductCode)
	if err != nil {
		return dto.ProductEntitlement{}, err
	}
	for _, order := range orders {
		if !isPaidPurchaseOrderStatus(order.Status) {
			continue
		}
		expiresAt := purchaseOrderExpiresAt(order)
		if expiresAt <= now {
			continue
		}
		out.Active = true
		if expiresAt > out.ExpiresAt {
			out.ExpiresAt = expiresAt
		}
	}
	return out, nil
}

func (s *dbState) activeProductExpiresAt(ctx context.Context, userID, productCode, excludeOrderID string) (int64, error) {
	now := time.Now().Unix()
	orders, err := s.pg.ListPurchaseOrdersByUserProduct(ctx, userID, productCode)
	if err != nil {
		return 0, err
	}
	var maxExpiresAt int64
	for _, order := range orders {
		if order.OrderID == excludeOrderID || !isPaidPurchaseOrderStatus(order.Status) {
			continue
		}
		expiresAt := purchaseOrderExpiresAt(order)
		if expiresAt <= now {
			continue
		}
		if expiresAt > maxExpiresAt {
			maxExpiresAt = expiresAt
		}
	}
	return maxExpiresAt, nil
}

func isPaidPurchaseOrderStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paid", "active", "completed":
		return true
	default:
		return false
	}
}

func purchaseOrderExpiresAt(order repo.PurchaseOrder) int64 {
	if order.ExpiresAt > 0 {
		return order.ExpiresAt
	}
	if strings.EqualFold(strings.TrimSpace(order.ProductType), "addon_device") {
		return 0
	}
	base := order.CreatedAt
	if base <= 0 {
		base = order.UpdatedAt
	}
	if base <= 0 {
		base = time.Now().Unix()
	}
	months := order.Months
	if months <= 0 {
		months = 1
	}
	return purchaseOrderExpiresAtForMonths(base, order.BillingCycle, months)
}

func purchaseOrderExpiresAtForMonths(base int64, billingCycle string, months int) int64 {
	if base <= 0 {
		base = time.Now().Unix()
	}
	if months <= 0 {
		months = 1
	}
	switch strings.ToLower(strings.TrimSpace(billingCycle)) {
	case "quarter", "quarterly":
		return base + int64(months)*90*24*60*60
	case "year", "yearly", "annual":
		return base + int64(months)*365*24*60*60
	case "day", "daily":
		return base + int64(months)*24*60*60
	default:
		return base + int64(months)*30*24*60*60
	}
}

func purchaseProductToDTO(product repo.Product) dto.PurchaseProduct {
	return dto.PurchaseProduct{
		MerchantID:         product.MerchantID,
		MerchantName:       product.MerchantName,
		ProductCode:        product.ProductCode,
		ProductName:        product.ProductName,
		Description:        product.Description,
		ProductType:        product.ProductType,
		PriceCents:         product.PriceCents,
		Currency:           product.Currency,
		BillingCycle:       product.BillingCycle,
		UnitQuantity:       product.UnitQuantity,
		MaxActiveDevices:   product.MaxActiveDevices,
		BandwidthLimitMbps: product.BandwidthLimitMbps,
	}
}

func (s *dbState) purchaseOrderToDTO(ctx context.Context, order repo.PurchaseOrder) dto.PurchaseOrder {
	expiresAt := purchaseOrderExpiresAt(order)
	if strings.EqualFold(strings.TrimSpace(order.ProductType), "addon_device") {
		if boundExpiresAt, err := s.pg.MaxPurchaseOrderDeviceBindingExpiresAt(ctx, order.OrderID); err == nil {
			expiresAt = boundExpiresAt
		}
	}
	return dto.PurchaseOrder{
		OrderID:      order.OrderID,
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

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
