package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func managedDeviceGlobalIP(ctx context.Context, networks repository.NetworkRepository, deviceID string, network model.Network) string {
	deviceIDs := []string{deviceID}
	deviceIDSet := map[string]struct{}{deviceID: {}}
	members, err := networks.ListNetworkDevices(ctx, network.NetworkID)
	if err != nil {
		return ""
	}
	for _, member := range members {
		if !member.Enabled || member.Status != "active" || member.DeviceID == "" {
			continue
		}
		if _, ok := deviceIDSet[member.DeviceID]; ok {
			continue
		}
		deviceIDSet[member.DeviceID] = struct{}{}
		deviceIDs = append(deviceIDs, member.DeviceID)
	}
	globalIPs, _ := assignedNetworkIPMap(network.CIDR, deviceIDs)
	return globalIPs[deviceID]
}
