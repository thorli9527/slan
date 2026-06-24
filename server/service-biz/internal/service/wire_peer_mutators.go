package service

import (
	"time"

	"github.com/slan/service-biz/internal/model"
)

func applyWirePeerPathHealth(item model.NetworkDevice, probe WirePathProbeInput, now time.Time) model.NetworkDevice {
	item.ActivePath = probe.Path
	item.PathObservedAt = probe.ObservedAt
	item.PathScore = int64(wireProbeSortScore(probe))
	item.ObservedRttMs = int64(probe.RTTMs)
	item.PacketLossPpm = int64(probe.LossPPM)
	item.RelayMtu = probe.MTU
	if item.PathObservedAt <= 0 {
		item.PathObservedAt = now.UnixMilli()
	}
	item.UpdatedAt = now.Unix()
	return item
}
