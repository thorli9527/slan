package service

import "github.com/slan/service-biz/internal/model"

func deviceGroupView(group model.DeviceGroup) DeviceGroupView {
	return DeviceGroupView{
		GroupID:   group.GroupID,
		Name:      group.Name,
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}
}
