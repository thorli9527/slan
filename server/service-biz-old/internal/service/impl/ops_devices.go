package impl

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
)

func (s dbOpsService) ListDevices() ([]dto.OpsDevice, error) {
	ctx := context.Background()
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodeIDsByDevice := make(map[string][]string)
	for _, node := range nodes {
		nodeIDsByDevice[node.DeviceID] = append(nodeIDsByDevice[node.DeviceID], node.NodeID)
	}
	out := make([]dto.OpsDevice, 0, len(devices))
	for _, device := range devices {
		networkIDs, err := s.state.deviceNetworkIDs(ctx, device.DeviceID)
		if err != nil {
			return nil, err
		}
		item := dto.OpsDevice{
			Device: dto.Device{
				DeviceID:   device.DeviceID,
				Name:       device.Name,
				Platform:   device.Platform,
				Status:     device.Status,
				NetworkIDs: networkIDs,
			},
			UserID:    device.UserID,
			NodeCount: len(nodeIDsByDevice[device.DeviceID]),
			NodeIDs:   append([]string(nil), nodeIDsByDevice[device.DeviceID]...),
		}
		if device.PublicKey != nil {
			item.PublicKey = *device.PublicKey
		}
		out = append(out, item)
	}
	return out, nil
}
