package service

import (
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func networkView(item model.Network) NetworkView {
	return NetworkView{
		NetworkID:        item.NetworkID,
		OwnerID:          item.OwnerID,
		Name:             item.Name,
		CIDR:             item.CIDR,
		IntraGroupPolicy: strings.TrimSpace(item.IntraGroupPolicy),
		Default:          item.Default,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func networkSummaryView(item model.Network) NetworkSummaryView {
	return NetworkSummaryView{
		Network:          networkView(item),
		IntraGroupPolicy: firstNonEmpty(strings.TrimSpace(item.IntraGroupPolicy), "allow"),
		Default:          item.Default,
	}
}

func networkSummaryZoneName(item model.Network, zones []model.DNSZone) string {
	for _, zone := range zones {
		name := strings.TrimSpace(zone.Name)
		if name != "" {
			return name
		}
	}
	code := strings.TrimSpace(item.Name)
	code = strings.ToLower(strings.ReplaceAll(code, " ", "-"))
	if code == "" {
		code = item.NetworkID
	}
	return code + ".internal"
}

func networkRuntimePathView(item model.NetworkDevice, ok bool) NetworkRuntimePathView {
	if !ok {
		return NetworkRuntimePathView{}
	}
	return NetworkRuntimePathView{
		NATType:         item.NATType,
		ActivePath:      item.ActivePath,
		RelayTransport:  item.RelayTransport,
		RelayEndpoint:   item.RelayEndpoint,
		DerpNodeID:      item.DerpNodeID,
		PeerNodeID:      item.PeerNodeID,
		PathScore:       item.PathScore,
		SignalScore:     item.SignalScore,
		SignalQuality:   item.SignalQuality,
		ObservedRttMs:   item.ObservedRttMs,
		PacketLossPpm:   item.PacketLossPpm,
		RelayMtu:        item.RelayMtu,
		MaxFramePayload: item.MaxFramePayload,
		TicketExpiresAt: item.TicketExpiresAt,
		TicketRenewDue:  item.TicketRenewDue,
		PathDowngrades:  item.PathDowngrades,
		PathUpgrades:    item.PathUpgrades,
		LastPathChange:  item.LastPathChange,
		ObservedAt:      item.PathObservedAt,
	}
}

func deviceEndpointViews(items []model.DeviceEndpoint) []DeviceEndpointView {
	if len(items) == 0 {
		return []DeviceEndpointView{}
	}
	views := make([]DeviceEndpointView, 0, len(items))
	for _, item := range items {
		views = append(views, DeviceEndpointView{
			Type:      item.Type,
			Address:   item.Address,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return views
}
