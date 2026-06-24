package service

import (
	"strconv"

	"github.com/slan/service-biz/internal/model"
)

func applyUpdateCustomerInput(user model.User, input UpdateCustomerInput, now int64) model.User {
	if input.Email != "" {
		user.Email = input.Email
	}
	if input.Name != "" {
		user.Name = input.Name
	}
	user.Country = input.Country
	user.Province = input.Province
	user.City = input.City
	user.IPRegion = input.IPRegion
	if input.Status != "" {
		user.Status = input.Status
	}
	user.UpdatedAt = now
	return user
}

func newManualRenewal(now int64, customerID, customerEmail string, input AssignCustomerPlanInput) model.Renewal {
	return model.Renewal{
		RenewalID:     "renewal-" + strconv.FormatInt(now, 10),
		CustomerID:    customerID,
		CustomerEmail: customerEmail,
		PlanCode:      input.PlanCode,
		Period:        firstNonEmpty(input.Period, "custom"),
		Amount:        input.Amount,
		Status:        "paid",
		RenewAt:       firstNonZero(input.ExpiresAt, now),
		PaidAt:        now,
		Source:        "manual",
		Operator:      "ops",
		UpdatedAt:     now,
	}
}
