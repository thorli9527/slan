package service

import "strings"

func normalizeNetworkOwnerID(ownerID string) string {
	return strings.TrimSpace(ownerID)
}

func normalizeNetworkID(networkID string) string {
	return strings.TrimSpace(networkID)
}

func normalizeCreateNetworkInput(input CreateNetworkInput) CreateNetworkInput {
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.CIDR = strings.TrimSpace(input.CIDR)
	input.IntraGroupPolicy = strings.TrimSpace(input.IntraGroupPolicy)
	return input
}

func normalizeUpdateNetworkInput(input UpdateNetworkInput) UpdateNetworkInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.CIDR = strings.TrimSpace(input.CIDR)
	input.IntraGroupPolicy = strings.TrimSpace(input.IntraGroupPolicy)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeDeleteNetworkInput(input DeleteNetworkInput) DeleteNetworkInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
