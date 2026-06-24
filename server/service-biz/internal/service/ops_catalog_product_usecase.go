package service

import (
	"context"
)

func (s OpsCatalogProductService) ListProducts(ctx context.Context) ([]ProductView, error) {
	items, err := s.Catalog.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	return productViews(items), nil
}

func (s OpsCatalogProductService) CreateProduct(ctx context.Context, input CreateProductInput) (ProductView, error) {
	input = normalizeCreateProductInput(input)
	if input.Name == "" {
		return ProductView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := newProduct(opsProductID(s.Catalog), now, input)
	if err := s.Catalog.SaveProduct(ctx, item); err != nil {
		return ProductView{}, err
	}
	return productView(item), nil
}

func (s OpsCatalogProductService) UpdateProduct(ctx context.Context, input UpdateProductInput) (ProductView, error) {
	input = normalizeUpdateProductInput(input)
	if input.ProductID == "" {
		return ProductView{}, ErrInvalidArgument
	}
	item, err := requireOpsProduct(ctx, s.Catalog, input.ProductID)
	if err != nil {
		return ProductView{}, err
	}
	item = applyUpdateProductInput(item, input, opsNow(s.Now).Unix())
	if err := s.Catalog.SaveProduct(ctx, item); err != nil {
		return ProductView{}, err
	}
	return productView(item), nil
}
