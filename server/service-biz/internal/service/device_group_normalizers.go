package service

import "strings"

func normalizeCreateDeviceGroupInput(input CreateDeviceGroupInput) CreateDeviceGroupInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateDeviceGroupInput(input UpdateDeviceGroupInput) UpdateDeviceGroupInput {
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeDeleteDeviceGroupInput(input DeleteDeviceGroupInput) DeleteDeviceGroupInput {
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}

func normalizeSetDeviceGroupsInput(input SetDeviceGroupsInput) SetDeviceGroupsInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	return input
}

func normalizeDeviceGroupID(groupID string) string {
	return strings.TrimSpace(groupID)
}
