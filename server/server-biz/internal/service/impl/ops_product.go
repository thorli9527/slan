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

const (
	defaultFreeMaxActiveDevices   = 2
	defaultFreeBandwidthLimitMbps = 1
	defaultMerchantID             = "merchant-platform"
	defaultMerchantName           = "SLAN Platform"
)

func (s dbOpsService) ListProducts() ([]dto.OpsProduct, error) {
	products, err := s.state.pg.ListProducts(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]dto.OpsProduct, 0, len(products))
	for _, product := range products {
		out = append(out, productToDTO(product))
	}
	return out, nil
}

func (s dbOpsService) UpsertProduct(req dto.UpsertProductRequest) (dto.OpsProduct, error) {
	code := strings.TrimSpace(strings.ToLower(req.ProductCode))
	name := strings.TrimSpace(req.ProductName)
	if code == "" || name == "" {
		return dto.OpsProduct{}, fmt.Errorf("%w: productCode and productName are required", ErrInvalidArgument)
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	currency := strings.TrimSpace(strings.ToUpper(req.Currency))
	if currency == "" {
		currency = "CNY"
	}
	billingCycle := strings.TrimSpace(req.BillingCycle)
	if billingCycle == "" {
		billingCycle = "month"
	}
	productType := strings.TrimSpace(strings.ToLower(req.ProductType))
	if productType == "" {
		productType = "plan"
	}
	merchantID := strings.TrimSpace(req.MerchantID)
	if merchantID == "" {
		merchantID = defaultMerchantID
	}
	merchantName := strings.TrimSpace(req.MerchantName)
	if merchantName == "" {
		merchantName = defaultMerchantName
	}
	unitQuantity := req.UnitQuantity
	if unitQuantity <= 0 {
		unitQuantity = 1
	}
	maxActiveDevices := req.MaxActiveDevices
	if maxActiveDevices < 0 {
		return dto.OpsProduct{}, fmt.Errorf("%w: maxActiveDevices must not be negative", ErrInvalidArgument)
	}
	bandwidthLimitMbps := req.BandwidthLimitMbps
	if bandwidthLimitMbps < 0 {
		return dto.OpsProduct{}, fmt.Errorf("%w: bandwidthLimitMbps must not be negative", ErrInvalidArgument)
	}
	now := time.Now().Unix()
	record := repo.Product{
		ProductID:          util.NewID("prod"),
		MerchantID:         merchantID,
		MerchantName:       merchantName,
		ProductCode:        code,
		ProductName:        name,
		Description:        strings.TrimSpace(req.Description),
		ProductType:        productType,
		PriceCents:         req.PriceCents,
		Currency:           currency,
		BillingCycle:       billingCycle,
		UnitQuantity:       unitQuantity,
		MaxActiveDevices:   maxActiveDevices,
		BandwidthLimitMbps: bandwidthLimitMbps,
		IsDefault:          req.IsDefault,
		Status:             status,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.state.pg.UpsertProduct(context.Background(), record); err != nil {
		return dto.OpsProduct{}, err
	}
	saved, err := s.state.pg.GetProductByCode(context.Background(), code)
	if err != nil {
		return dto.OpsProduct{}, err
	}
	if saved.IsDefault {
		s.state.syncAllUserEntitlements(context.Background(), "default product changed")
	}
	return productToDTO(saved), nil
}

func productToDTO(product repo.Product) dto.OpsProduct {
	return dto.OpsProduct{
		ProductID:          product.ProductID,
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
		IsDefault:          product.IsDefault,
		Status:             product.Status,
		CreatedAt:          product.CreatedAt,
		UpdatedAt:          product.UpdatedAt,
	}
}

func (s *dbState) currentDefaultProduct(ctx context.Context) repo.Product {
	product, err := s.pg.GetDefaultProduct(ctx)
	if err == nil {
		return product
	}
	return repo.Product{
		ProductID:          "prod-free",
		MerchantID:         defaultMerchantID,
		MerchantName:       defaultMerchantName,
		ProductCode:        "free",
		ProductName:        "Free",
		ProductType:        "plan",
		UnitQuantity:       1,
		MaxActiveDevices:   defaultFreeMaxActiveDevices,
		BandwidthLimitMbps: defaultFreeBandwidthLimitMbps,
		IsDefault:          true,
		Status:             "active",
	}
}
