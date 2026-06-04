package biz

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) AssignCustomerPlan(customerID, planCode string, expiresAt int64, amount float64, period, operatorEmail string) (CustomerProfile, Renewal, error) {
	customerID = strings.TrimSpace(customerID)
	planCode = strings.TrimSpace(planCode)
	if customerID == "" || planCode == "" {
		return CustomerProfile{}, Renewal{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[customerID]
	if !ok {
		return CustomerProfile{}, Renewal{}, errNotFound
	}
	if _, ok := s.opsPlans[planCode]; !ok {
		return CustomerProfile{}, Renewal{}, errNotFound
	}
	now := time.Now().Unix()
	if expiresAt == 0 {
		expiresAt = now + int64((365 * 24 * time.Hour).Seconds())
	}
	s.customerPlans[customerID] = CustomerPlanAssignment{UserID: customerID, PlanCode: planCode, ExpiresAt: expiresAt, UpdatedAt: now}
	renewal := Renewal{
		RenewalID:     fmt.Sprintf("renew-%06d", s.nextRenewalSeq),
		CustomerID:    customerID,
		CustomerEmail: user.Email,
		PlanCode:      planCode,
		Period:        defaultString(period, "manual"),
		Amount:        amount,
		Currency:      "CNY",
		PaidAt:        now,
		ValidUntil:    expiresAt,
		Source:        "manual",
		Operator:      operatorEmail,
	}
	s.nextRenewalSeq++
	s.renewals[renewal.RenewalID] = renewal
	return s.customerProfileLocked(user), renewal, nil
}
