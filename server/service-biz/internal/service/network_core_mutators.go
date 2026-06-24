package service

import "github.com/slan/service-biz/internal/model"

func newManagedNetwork(now int64, id string, input CreateNetworkInput) model.Network {
	return model.Network{
		NetworkID:        id,
		OwnerID:          input.OwnerID,
		Name:             input.Name,
		CIDR:             input.CIDR,
		Code:             input.Code,
		TemplateKey:      firstNonEmpty(input.TemplateKey, input.Code, "custom"),
		IntraGroupPolicy: firstNonEmpty(input.IntraGroupPolicy, "allow"),
		Default:          input.Default,
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func applyUpdateNetworkInput(item model.Network, input UpdateNetworkInput, now int64) model.Network {
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.CIDR != "" {
		item.CIDR = input.CIDR
	}
	if input.Code != "" {
		item.Code = input.Code
	}
	if input.TemplateKey != "" {
		item.TemplateKey = input.TemplateKey
	}
	if input.IntraGroupPolicy != "" {
		item.IntraGroupPolicy = input.IntraGroupPolicy
	}
	if input.Default != nil {
		item.Default = *input.Default
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	item.UpdatedAt = now
	return item
}
