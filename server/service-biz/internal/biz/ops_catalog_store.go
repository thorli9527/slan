package biz

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListPlans() []OpsPlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.opsPlans, func(a, b OpsPlan) bool { return a.Code < b.Code })
}

func (s *Store) UpsertPlan(plan OpsPlan) (OpsPlan, error) {
	plan.Code = sanitizeDNSLabel(plan.Code)
	plan.Name = strings.TrimSpace(plan.Name)
	plan.Status = defaultString(plan.Status, "active")
	if plan.Code == "" || plan.Name == "" {
		return OpsPlan{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opsPlans[plan.Code] = plan
	return plan, nil
}

func (s *Store) ListProducts() []Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.products, func(a, b Product) bool { return a.ProductID < b.ProductID })
}

func (s *Store) UpsertProduct(product Product) (Product, error) {
	product.Name = strings.TrimSpace(product.Name)
	product.Type = defaultString(product.Type, "plan")
	product.Period = defaultString(product.Period, "monthly")
	product.Currency = defaultString(product.Currency, "CNY")
	product.Status = defaultString(product.Status, "active")
	if product.Name == "" || strings.TrimSpace(product.PlanCode) == "" {
		return Product{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.opsPlans[product.PlanCode]; !ok {
		return Product{}, errNotFound
	}
	now := time.Now().Unix()
	if strings.TrimSpace(product.ProductID) == "" {
		return s.addProductLocked(product), nil
	}
	existing, ok := s.products[product.ProductID]
	if !ok {
		return Product{}, errNotFound
	}
	product.CreatedAt = existing.CreatedAt
	product.UpdatedAt = now
	s.products[product.ProductID] = product
	return product, nil
}

func (s *Store) addProductLocked(product Product) Product {
	now := time.Now().Unix()
	product.ProductID = fmt.Sprintf("prod-%06d", s.nextProductSeq)
	s.nextProductSeq++
	product.CreatedAt = now
	product.UpdatedAt = now
	s.products[product.ProductID] = product
	return product
}
