package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type deviceRegistrationTestDevices struct {
	networkRuntimeTestDevices
	nextVirtualIP int
}

func (s *deviceRegistrationTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	if s.devices == nil {
		s.devices = map[string]model.Device{}
	}
	s.devices[device.DeviceID] = device
	return nil
}

func (s *deviceRegistrationTestDevices) NewDeviceVirtualIPID() string {
	s.nextVirtualIP++
	return "vip-" + leftPadInt(s.nextVirtualIP, 6)
}

func leftPadInt(value, width int) string {
	text := ""
	for current := value; current > 0; current /= 10 {
		text = string(rune('0'+(current%10))) + text
	}
	if text == "" {
		text = "0"
	}
	for len(text) < width {
		text = "0" + text
	}
	return text
}

type deviceRegistrationTestNetworks struct {
	networkRuntimeTestNetworks
}
