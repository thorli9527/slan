package impl

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
	end := broadcast - 1
	start := base + 10
	if start >= end {
		start = base + 2
	}
	startRaw := strings.TrimSpace(startIP)
	endRaw := strings.TrimSpace(endIP)
	if (startRaw == "") != (endRaw == "") {
		return netip.Prefix{}, 0, 0, fmt.Errorf(
			"%w: allocationStartIp and allocationEndIp must be provided together",
			ErrInvalidArgument,
		)
	}

	if gatewayIP != "" {
		gatewayAddr, err := parseIPv4InPrefix(gatewayIP, prefix, "gatewayIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if gatewayAddr == base || gatewayAddr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: gatewayIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		gateway = gatewayAddr
		if start <= gateway {
			start = gateway + 1
		}
	}
	if startRaw != "" {
		addr, err := parseIPv4InPrefix(startIP, prefix, "allocationStartIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if addr == base || addr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: allocationStartIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		start = addr
	}
	if endRaw != "" {
		addr, err := parseIPv4InPrefix(endIP, prefix, "allocationEndIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if addr == base || addr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: allocationEndIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		end = addr
	}
	if start <= gateway || end <= start {
		return netip.Prefix{}, 0, 0, fmt.Errorf(
			"%w: invalid allocation range, ensure gateway < allocationStartIp < allocationEndIp",
			ErrInvalidArgument,
		)
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

func mustParseAddr(raw string) netip.Addr {
	addr, _ := netip.ParseAddr(strings.TrimSpace(raw))
	return addr
}

type subnetTemplate struct {
	name   string
	remark string
}

var defaultSubnetTemplates = []subnetTemplate{
	{name: "默认网络", remark: "默认地址池，用于当前网络设备 IP 分配"},
}

func createNetworkCIDR(cidr string) string {
	if strings.TrimSpace(cidr) != "" {
		return strings.TrimSpace(cidr)
	}
	return "100.64.0.0/24"
}

func defaultSubnetsForNetwork(networkID, cidr string, newID func() string) ([]dto.Subnet, error) {
	cidrs, err := subdivideSubnetCIDRs(cidr, len(defaultSubnetTemplates))
	if err != nil {
		return nil, err
	}
	templates := defaultSubnetTemplates
	if len(cidrs) == 1 {
		templates = templates[:1]
	}
	subnets := make([]dto.Subnet, 0, len(cidrs))
	for index, subnetCIDR := range cidrs {
		template := templates[index]
		subnet, err := newSubnet(networkID, newID(), template.name, subnetCIDR, "", "", "", index == 0)
		if err != nil {
			return nil, err
		}
		subnet.Remark = template.remark
		subnets = append(subnets, subnet)
	}
	return subnets, nil
}

func subdivideSubnetCIDRs(cidr string, count int) ([]string, error) {
	prefix, _, _, err := subnetRange(cidr, "", "", "")
	if err != nil {
		return nil, err
	}
	if count <= 1 {
		return []string{prefix.String()}, nil
	}
	extraBits := 0
	for slots := 1; slots < count; slots <<= 1 {
		extraBits++
	}
	childBits := prefix.Bits() + extraBits
	if childBits > 29 {
		return []string{prefix.String()}, nil
	}
	base := networkBase(prefix)
	step := uint32(1) << uint(32-childBits)
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		child := netip.PrefixFrom(uint32ToAddr(base+uint32(i)*step), childBits).Masked()
		out = append(out, child.String())
	}
	return out, nil
}
