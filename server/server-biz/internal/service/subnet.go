package service

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
)

func newSubnet(networkID, subnetID, name, cidr, gatewayIP, startIP, endIP string, isDefault bool) (dto.Subnet, error) {
	prefix, start, end, err := subnetRange(cidr, gatewayIP, startIP, endIP)
	if err != nil {
		return dto.Subnet{}, err
	}
	if gatewayIP == "" {
		gatewayIP = uint32ToAddr(networkBase(prefix) + 1).String()
	}
	if startIP == "" {
		startIP = uint32ToAddr(start).String()
	}
	if endIP == "" {
		endIP = uint32ToAddr(end).String()
	}

	return dto.Subnet{
		SubnetID:          subnetID,
		NetworkID:         networkID,
		Name:              strings.TrimSpace(name),
		CIDR:              prefix.String(),
		GatewayIP:         gatewayIP,
		AllocationStartIP: startIP,
		AllocationEndIP:   endIP,
		IsDefault:         isDefault,
		Status:            "active",
	}, nil
}

func subnetRange(cidr, gatewayIP, startIP, endIP string) (netip.Prefix, uint32, uint32, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: invalid cidr", ErrInvalidArgument)
	}
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: only IPv4 is supported", ErrInvalidArgument)
	}

	base := networkBase(prefix)
	broadcast := base | ^mask(prefix)
	if broadcast-base < 3 {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: subnet too small", ErrInvalidArgument)
	}

	gateway := base + 1
	start := base + 2
	end := broadcast - 1

	if gatewayIP != "" {
		gatewayAddr, err := parseIPv4InPrefix(gatewayIP, prefix, "gatewayIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		gateway = gatewayAddr
		if start <= gateway {
			start = gateway + 1
		}
	}
	if startIP != "" {
		addr, err := parseIPv4InPrefix(startIP, prefix, "allocationStartIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		start = addr
	}
	if endIP != "" {
		addr, err := parseIPv4InPrefix(endIP, prefix, "allocationEndIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		end = addr
	}
	if start <= gateway || end <= start {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: invalid allocation range", ErrInvalidArgument)
	}
	return prefix, start, end, nil
}

func parseIPv4InPrefix(raw string, prefix netip.Prefix, field string) (uint32, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !addr.Is4() || !prefix.Contains(addr) {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidArgument, field)
	}
	return addrToUint32(addr), nil
}

func networkBase(prefix netip.Prefix) uint32 {
	return addrToUint32(prefix.Masked().Addr())
}

func mask(prefix netip.Prefix) uint32 {
	bits := prefix.Bits()
	if bits == 0 {
		return 0
	}
	return ^uint32(0) << (32 - bits)
}

func addrToUint32(addr netip.Addr) uint32 {
	bytes := addr.As4()
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func uint32ToAddr(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
}
