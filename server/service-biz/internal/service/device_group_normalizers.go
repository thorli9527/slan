package service

import "strings"

func normalizeCreateDeviceGroupInput(input CreateDeviceGroupInput) CreateDeviceGroupInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateDeviceGroupInput(input UpdateDeviceGroupInput) UpdateDeviceGroupInput {
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeDeleteDeviceGroupInput(input DeleteDeviceGroupInput) DeleteDeviceGroupInput {
	input.GroupID = strings.TrimSpace(input.GroupID)
	return input
}

func normalizeSetDeviceGroupsInput(input SetDeviceGroupsInput) SetDeviceGroupsInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	seen := make(map[string]struct{}, len(input.GroupIDs))
	groupIDs := make([]string, 0, len(input.GroupIDs))
	for _, groupID := range input.GroupIDs {
		groupID = normalizeDeviceGroupID(groupID)
		if groupID == "" {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		groupIDs = append(groupIDs, groupID)
	}
	input.GroupIDs = groupIDs
	return input
}

func normalizeDeviceGroupID(groupID string) string {
	return strings.TrimSpace(groupID)
}
