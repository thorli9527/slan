package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type NetworkEventDeliveryRepository interface {
	GetNetworkEventDelivery(ctx context.Context, eventID, targetDeviceID string) (model.NetworkEventDelivery, bool, error)
	ListDueNetworkEventDeliveries(ctx context.Context, dueAt int64, limit int) ([]model.NetworkEventDelivery, error)
	ClaimDueNetworkEventDeliveries(ctx context.Context, dueAt, leaseUntil int64, limit int) ([]model.NetworkEventDelivery, error)
	SaveNetworkEventDelivery(ctx context.Context, item model.NetworkEventDelivery) error
	UpdatePendingNetworkEventDelivery(ctx context.Context, item model.NetworkEventDelivery) (bool, error)
	DeleteTerminalNetworkEventDeliveriesBefore(ctx context.Context, cutoff int64) (int64, error)
}

type NetworkEventDeliveryStore interface {
	NetworkEventDeliveryRepository
	ListNetworkDevices(ctx context.Context, networkID string) ([]model.NetworkDevice, error)
}
