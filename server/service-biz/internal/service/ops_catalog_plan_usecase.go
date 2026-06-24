package service

import (
	"context"
)

func (s OpsCatalogPlanService) ListPlans(ctx context.Context) ([]PlanView, error) {
	items, err := s.Catalog.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	return planViews(items), nil
}

func (s OpsCatalogPlanService) UpsertPlan(ctx context.Context, input UpsertPlanInput) (PlanView, error) {
	input = normalizeUpsertPlanInput(input)
	if input.PlanCode == "" {
		return PlanView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := newPlan(now, input)
	if current, ok, err := s.Catalog.GetPlan(ctx, item.PlanCode); err != nil {
		return PlanView{}, err
	} else if ok {
		item = mergePlan(current, input, now)
	}
	if err := s.Catalog.SavePlan(ctx, item); err != nil {
		return PlanView{}, err
	}
	return planView(item), nil
}
