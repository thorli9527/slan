package biz

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListOrders() []Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.orders, func(a, b Order) bool { return a.CreatedAt > b.CreatedAt })
}

func (s *Store) UpsertOrder(order Order) (Order, error) {
	order.CustomerEmail = strings.ToLower(strings.TrimSpace(order.CustomerEmail))
	order.ProductID = strings.TrimSpace(order.ProductID)
	order.PayStatus = defaultString(order.PayStatus, "pending")
	order.ProvisionStatus = defaultString(order.ProvisionStatus, "pending")
	order.Currency = defaultString(order.Currency, "CNY")
	order.Channel = defaultString(order.Channel, "manual")
	if order.CustomerEmail == "" || order.ProductID == "" {
		return Order{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.userByEmail[order.CustomerEmail]
	if !ok {
		return Order{}, errNotFound
	}
	product, ok := s.products[order.ProductID]
	if !ok {
		return Order{}, errNotFound
	}
	now := time.Now().Unix()
	if strings.TrimSpace(order.OrderID) == "" {
		order.OrderID = fmt.Sprintf("ord-%06d", s.nextOrderSeq)
		s.nextOrderSeq++
		order.CreatedAt = now
	} else {
		existing, ok := s.orders[order.OrderID]
		if !ok {
			return Order{}, errNotFound
		}
		if order.CreatedAt == 0 {
			order.CreatedAt = existing.CreatedAt
		}
		if order.PaidAt == 0 {
			order.PaidAt = existing.PaidAt
		}
		if order.ValidUntil == 0 {
			order.ValidUntil = existing.ValidUntil
		}
	}
	order.CustomerID = userID
	order.ProductName = product.Name
	order.ProductType = product.Type
	if order.Amount == 0 {
		order.Amount = product.SalePrice
	}
	if order.ValidUntil == 0 && product.ValidDays > 0 {
		order.ValidUntil = now + int64(time.Duration(product.ValidDays)*24*time.Hour/time.Second)
	}
	s.orders[order.OrderID] = order
	return order, nil
}

func (s *Store) ListRenewals() []Renewal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.renewals, func(a, b Renewal) bool { return a.PaidAt > b.PaidAt })
}

func (s *Store) UpdateRenewal(renewal Renewal) (Renewal, error) {
	renewal.RenewalID = strings.TrimSpace(renewal.RenewalID)
	renewal.CustomerEmail = strings.ToLower(strings.TrimSpace(renewal.CustomerEmail))
	renewal.PlanCode = strings.TrimSpace(renewal.PlanCode)
	renewal.Period = defaultString(renewal.Period, "manual")
	renewal.Currency = defaultString(renewal.Currency, "CNY")
	renewal.Source = defaultString(renewal.Source, "manual")
	if renewal.RenewalID == "" || renewal.CustomerEmail == "" || renewal.PlanCode == "" {
		return Renewal{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.renewals[renewal.RenewalID]
	if !ok {
		return Renewal{}, errNotFound
	}
	userID, ok := s.userByEmail[renewal.CustomerEmail]
	if !ok {
		return Renewal{}, errNotFound
	}
	if _, ok := s.opsPlans[renewal.PlanCode]; !ok {
		return Renewal{}, errNotFound
	}
	if renewal.PaidAt == 0 {
		renewal.PaidAt = existing.PaidAt
	}
	if renewal.ValidUntil == 0 {
		renewal.ValidUntil = existing.ValidUntil
	}
	renewal.CustomerID = userID
	s.renewals[renewal.RenewalID] = renewal
	s.customerPlans[userID] = CustomerPlanAssignment{
		UserID:    userID,
		PlanCode:  renewal.PlanCode,
		ExpiresAt: renewal.ValidUntil,
		UpdatedAt: time.Now().Unix(),
	}
	return renewal, nil
}
