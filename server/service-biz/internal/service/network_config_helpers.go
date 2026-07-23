package service

import (
	"net"
	"strconv"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

const (
	deviceVirtualIPHostCount       = 254
	deviceVirtualIPFirstTierBlocks = 254
	deviceVirtualIPTierBlocks      = 255
	deviceVirtualIPPoolCapacity    = (deviceVirtualIPFirstTierBlocks + 255*deviceVirtualIPTierBlocks) * deviceVirtualIPHostCount
	deviceVirtualIPCapacity        = 2 * deviceVirtualIPPoolCapacity
)

var deviceVirtualIPPoolPrefixes = [...]byte{10, 100}

func deviceGlobalIP(device model.Device) string {
	return strings.TrimSpace(device.VirtualIP)
}

func managedDeviceVirtualIP(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value)).To4()
	if ip == nil || (ip[0] != 10 && ip[0] != 100) || ip[2] == 255 || ip[3] == 0 || ip[3] == 255 {
		return false
	}
	return ip[1] != 0 || ip[2] != 0
}

func allocatedDeviceVirtualIP(sequenceID string) string {
	trimmed := strings.TrimSpace(sequenceID)
	if trimmed == "" {
		return ""
	}
	value := strings.TrimPrefix(trimmed, "vip-")
	index, err := strconv.Atoi(value)
	if err != nil || index <= 0 || index > deviceVirtualIPCapacity {
		return ""
	}
	zeroBased := index - 1
	poolIndex := zeroBased / deviceVirtualIPPoolCapacity
	poolOffset := zeroBased % deviceVirtualIPPoolCapacity
	subnetIndex := poolOffset / deviceVirtualIPHostCount
	fourth := 1 + poolOffset%deviceVirtualIPHostCount
	first := deviceVirtualIPPoolPrefixes[poolIndex]
	if subnetIndex < deviceVirtualIPFirstTierBlocks {
		return net.IPv4(first, 0, byte(subnetIndex+1), byte(fourth)).String()
	}
	subnetIndex -= deviceVirtualIPFirstTierBlocks
	second := 1 + subnetIndex/deviceVirtualIPTierBlocks
	third := subnetIndex % deviceVirtualIPTierBlocks
	return net.IPv4(first, byte(second), byte(third), byte(fourth)).String()
}

func networkGlobalName(deviceID, alias, name string) string {
	if value := strings.TrimSpace(alias); value != "" {
		return value
	}
	if value := strings.TrimSpace(name); value != "" {
		return value
	}
	return strings.TrimSpace(deviceID)
}
