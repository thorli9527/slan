package service

import (
	"context"
	"net"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildNetworkSummaryView(ctx context.Context, devices repository.DeviceRepository, networks repository.NetworkRepository, item model.Network) (NetworkSummaryView, error) {
	view := networkSummaryView(item)
	memberships, err := networks.ListNetworkDevices(ctx, item.NetworkID)
	if err != nil {
		return NetworkSummaryView{}, err
	}
	deviceCount := 0
	memberOwners := make(map[string]struct{})
	for _, membership := range memberships {
		if !networkMemberActive(membership) {
			continue
		}
		deviceCount++
		device, ok, err := devices.GetDevice(ctx, membership.DeviceID)
		if err != nil {
			return NetworkSummaryView{}, err
		}
		if ok && device.OwnerID != "" {
			memberOwners[device.OwnerID] = struct{}{}
		}
	}
	zones, err := networks.ListDNSZones(ctx, item.NetworkID)
	if err != nil {
		return NetworkSummaryView{}, err
	}
	view.DeviceCount = deviceCount
	view.MemberCount = len(memberOwners)
	view.ZoneName = networkSummaryZoneName(item, zones)
	return view, nil
}

func buildNetworkConfigView(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	network model.Network,
	device model.Device,
) (NetworkConfigView, error) {
	networkDevices, err := networks.ListNetworkDevices(ctx, network.NetworkID)
	if err != nil {
		return NetworkConfigView{}, err
	}
	zones, err := networks.ListDNSZones(ctx, network.NetworkID)
	if err != nil {
		return NetworkConfigView{}, err
	}
	records, err := networks.ListDNSRecords(ctx, network.NetworkID)
	if err != nil {
		return NetworkConfigView{}, err
	}
	securityGroups, err := networks.ListSecurityGroups(ctx, network.NetworkID)
	if err != nil {
		return NetworkConfigView{}, err
	}

	zoneViews := make([]DNSZoneView, 0, len(zones))
	for _, item := range zones {
		zoneViews = append(zoneViews, dnsZoneView(item))
	}
	recordViews := make([]DNSRecordView, 0, len(records))
	for _, item := range records {
		recordViews = append(recordViews, dnsRecordView(item))
	}
	securityGroupViews := make([]SecurityGroupView, 0, len(securityGroups))
	for _, item := range securityGroups {
		securityGroupViews = append(securityGroupViews, securityGroupView(item))
	}
	securityRules, err := buildNetworkSecurityRuleViews(ctx, networks, securityGroups)
	if err != nil {
		return NetworkConfigView{}, err
	}
	configVersion := int64(0)
	if version, ok, err := networks.GetNetworkVersion(ctx, network.NetworkID); err != nil {
		return NetworkConfigView{}, err
	} else if ok {
		configVersion = version.Version
	}

	deviceIDs := buildNetworkConfigDeviceIDs(device.DeviceID, networkDevices)
	deviceByID := map[string]model.Device{
		device.DeviceID: device,
	}
	for _, membership := range networkDevices {
		if membership.DeviceID == "" {
			continue
		}
		if _, ok := deviceByID[membership.DeviceID]; ok {
			continue
		}
		item, ok, err := devices.GetDevice(ctx, membership.DeviceID)
		if err != nil {
			return NetworkConfigView{}, err
		}
		if ok {
			deviceByID[membership.DeviceID] = item
		}
	}
	globalIPs := make(map[string]string, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if deviceID == "" {
			continue
		}
		item, ok := deviceByID[deviceID]
		if !ok {
			item = model.Device{DeviceID: deviceID}
		}
		globalIPs[deviceID] = deviceGlobalIP(item)
	}
	prefixLen := 0
	if _, ipnet, err := net.ParseCIDR(network.CIDR); err == nil && ipnet != nil {
		prefixLen, _ = ipnet.Mask.Size()
	}
	deviceGroupsByDevice, err := buildNetworkConfigDeviceGroupsByDevice(ctx, devices, network.OwnerID, deviceIDs)
	if err != nil {
		return NetworkConfigView{}, err
	}
	localMembership, localMembershipFound := findLocalNetworkMembership(device.DeviceID, networkDevices)
	peers, err := buildNetworkConfigPeerViews(ctx, users, devices, device.DeviceID, networkDevices, globalIPs)
	if err != nil {
		return NetworkConfigView{}, err
	}

	return NetworkConfigView{
		Network:              networkView(network),
		ConfigVersion:        configVersion,
		DeviceID:             device.DeviceID,
		NodeID:               "node-" + device.DeviceID,
		GlobalIP:             globalIPs[device.DeviceID],
		PrefixLen:            prefixLen,
		GlobalName:           networkGlobalName(device.DeviceID, device.Alias, device.Name),
		DeviceGroupsByDevice: deviceGroupsByDevice,
		RuntimePath:          networkRuntimePathView(localMembership, localMembershipFound),
		Peers:                peers,
		DNSZones:             zoneViews,
		DNSRecords:           recordViews,
		SecurityGroups:       securityGroupViews,
		SecurityRules:        securityRules,
	}, nil
}

func buildNetworkSecurityRuleViews(ctx context.Context, networks repository.NetworkRepository, groups []model.SecurityGroup) ([]SecurityRuleView, error) {
	views := make([]SecurityRuleView, 0)
	for _, group := range groups {
		items, err := networks.ListSecurityRules(ctx, group.SecurityGroupID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !supportedSecurityRulePeerType(item.PeerType) {
				continue
			}
			views = append(views, securityRuleView(item))
		}
	}
	return views, nil
}

func buildNetworkConfigDeviceGroupsByDevice(
	ctx context.Context,
	devices repository.DeviceRepository,
	ownerUserID string,
	deviceIDs []string,
) (map[string][]string, error) {
	out := make(map[string][]string)
	if devices == nil || strings.TrimSpace(ownerUserID) == "" {
		return out, nil
	}
	assignments, err := devices.ListDeviceGroupAssignments(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		allowed[deviceID] = struct{}{}
	}
	for _, assignment := range assignments {
		deviceID := strings.TrimSpace(assignment.DeviceID)
		if deviceID == "" {
			continue
		}
		if _, ok := allowed[deviceID]; !ok {
			continue
		}
		for _, groupID := range assignment.GroupIDs {
			groupID = strings.TrimSpace(groupID)
			if groupID == "" {
				continue
			}
			out[deviceID] = append(out[deviceID], groupID)
		}
	}
	return out, nil
}

func buildNetworkConfigDeviceIDs(localDeviceID string, memberships []model.NetworkDevice) []string {
	deviceIDs := make([]string, 0, len(memberships)+1)
	deviceIDSet := map[string]struct{}{localDeviceID: {}}
	deviceIDs = append(deviceIDs, localDeviceID)
	for _, membership := range memberships {
		if !networkMemberActive(membership) {
			continue
		}
		if _, ok := deviceIDSet[membership.DeviceID]; ok {
			continue
		}
		deviceIDSet[membership.DeviceID] = struct{}{}
		deviceIDs = append(deviceIDs, membership.DeviceID)
	}
	return deviceIDs
}

func findLocalNetworkMembership(deviceID string, memberships []model.NetworkDevice) (model.NetworkDevice, bool) {
	for _, membership := range memberships {
		if membership.DeviceID == deviceID {
			return membership, true
		}
	}
	return model.NetworkDevice{}, false
}

func buildNetworkConfigPeerViews(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	localDeviceID string,
	memberships []model.NetworkDevice,
	globalIPs map[string]string,
) ([]NetworkConfigPeerView, error) {
	peers := make([]NetworkConfigPeerView, 0, len(memberships))
	for _, membership := range memberships {
		if membership.DeviceID == localDeviceID || !networkMemberActive(membership) {
			continue
		}
		peerDevice, ok, err := devices.GetDevice(ctx, membership.DeviceID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		ownerEmail := ""
		if peerDevice.OwnerID != "" {
			if owner, ok, err := users.GetUser(ctx, peerDevice.OwnerID); err != nil {
				return nil, err
			} else if ok {
				ownerEmail = owner.Email
			}
		}
		peers = append(peers, NetworkConfigPeerView{
			DeviceID:   peerDevice.DeviceID,
			OwnerID:    peerDevice.OwnerID,
			OwnerEmail: ownerEmail,
			Alias:      peerDevice.Alias,
			GlobalIP:   globalIPs[peerDevice.DeviceID],
			GlobalName: networkGlobalName(peerDevice.DeviceID, peerDevice.Alias, peerDevice.Name),
			Status:     peerDevice.Status,
			Endpoints:  deviceEndpointViews(membership.Endpoints),
		})
	}
	return peers, nil
}
