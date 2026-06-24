package service

import (
	"context"
)

func (s OpsCustomerService) ListCustomers(ctx context.Context) ([]OpsCustomerView, error) {
	users, err := s.Users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	renewals, _ := s.Catalog.ListRenewals(ctx)
	planExpires := latestRenewalExpiryByCustomer(renewals)
	return buildOpsCustomerViews(ctx, users, s.Catalog, s.Devices, planExpires), nil
}

func (s OpsCustomerService) UpdateCustomer(ctx context.Context, input UpdateCustomerInput) (OpsCustomerView, error) {
	input = normalizeUpdateCustomerInput(input)
	if input.CustomerID == "" {
		return OpsCustomerView{}, ErrInvalidArgument
	}
	user, err := requireOpsUser(ctx, s.Users, input.CustomerID)
	if err != nil {
		return OpsCustomerView{}, err
	}
	user = applyUpdateCustomerInput(user, input, opsNow(s.Now).Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return OpsCustomerView{}, err
	}
	renewals, _ := s.Catalog.ListRenewals(ctx)
	return buildOpsCustomerView(ctx, s.Catalog, s.Devices, opsCustomerFromUser(ctx, s.Catalog, user), latestRenewalExpiryByCustomer(renewals)[user.UserID]), nil
}

func (s OpsCustomerService) AssignCustomerPlan(ctx context.Context, input AssignCustomerPlanInput) (OpsCustomerPlanAssignmentView, error) {
	input = normalizeAssignCustomerPlanInput(input)
	if input.CustomerID == "" || input.PlanCode == "" {
		return OpsCustomerPlanAssignmentView{}, ErrInvalidArgument
	}
	if _, err := requireOpsUser(ctx, s.Users, input.CustomerID); err != nil {
		return OpsCustomerPlanAssignmentView{}, err
	}
	if err := s.Catalog.SaveCustomerPlan(ctx, input.CustomerID, input.PlanCode); err != nil {
		return OpsCustomerPlanAssignmentView{}, err
	}
	now := opsNow(s.Now).Unix()
	customer, err := s.UpdateCustomer(ctx, UpdateCustomerInput{CustomerID: input.CustomerID})
	if err != nil {
		return OpsCustomerPlanAssignmentView{}, err
	}
	renewal := newManualRenewal(now, input.CustomerID, customer.Customer.Email, input)
	if err := s.Catalog.SaveRenewal(ctx, renewal); err != nil {
		return OpsCustomerPlanAssignmentView{}, err
	}
	return OpsCustomerPlanAssignmentView{
		Customer:   customer,
		PlanCode:   input.PlanCode,
		Period:     renewal.Period,
		Amount:     renewal.Amount,
		PaidAt:     renewal.PaidAt,
		ValidUntil: renewal.RenewAt,
	}, nil
}
