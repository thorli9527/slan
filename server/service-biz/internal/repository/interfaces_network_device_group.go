package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type NetworkDeviceGroupRepository interface {
	ListNetworkDeviceGroupReferences(ctx context.Context, networkID string) ([]model.NetworkDeviceGroupReference, error)
	SaveNetworkDeviceGroupReference(ctx context.Context, item model.NetworkDeviceGroupReference) error
	DeleteNetworkDeviceGroupReference(ctx context.Context, networkID, groupID string) error
	DeleteNetworkDeviceGroupReferencesByGroup(ctx context.Context, groupID string) error
}
