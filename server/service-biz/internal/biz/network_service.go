package biz

// NetworkService 承载网络、成员、DNS、安全组、公网映射和 relay ticket 业务实现。
type NetworkService struct {
	store BusinessStore
}

func (s NetworkService) CreateNetwork(req CreateNetworkRequest) (Network, SecurityGroup, NetworkDNSZone, error) {
	return s.store.CreateNetwork(req.OwnerUserID, req.Name, req.Code, req.TemplateKey)
}

func (s NetworkService) ListNetworks(userID string) []Network {
	return s.store.ListNetworks(userID)
}

func (s NetworkService) UpdateNetwork(networkID string, req UpdateNetworkRequest) (Network, string, error) {
	network, err := s.store.UpdateNetworkFull(networkID, req.Name, req.Code, req.Status)
	if err != nil {
		return Network{}, "", err
	}
	reason := "network_updated"
	if network.Status == "enabled" {
		reason = "network_enabled"
	} else if network.Status == "disabled" {
		reason = "network_disabled"
	}
	return network, reason, nil
}

func (s NetworkService) IssueRelayTicket(req IssueRelayTicketRequest) (RelayTicket, error) {
	preferred := append([]string{}, req.PreferredRelayEndpointIDs...)
	preferred = append(preferred, req.PreferredDERPNodeIDs...)
	return s.store.IssueRelayTicket(req.NetworkID, req.SrcNodeID, req.DstNodeID, req.DERPClusterID, preferred)
}

func (s NetworkService) CreateDeviceInvite(req CreateDeviceInviteRequest) (DeviceInvite, error) {
	return s.store.CreateDeviceInvite(req.InviterUserID, req.TTLSeconds)
}

func (s NetworkService) ListDeviceInvites(userID string) []DeviceInvite {
	return s.store.ListDeviceInvites(userID)
}

func (s NetworkService) AcceptDeviceInvite(req AcceptDeviceInviteRequest) (DeviceAccessGrant, DeviceInvite, error) {
	return s.store.AcceptDeviceInvite(req.InviteCode, req.DeviceID, req.ActorUserID)
}

func (s NetworkService) ListNetworkDevices(networkID string) []NetworkDevice {
	return s.store.ListNetworkDevices(networkID)
}

func (s NetworkService) AddNetworkDevice(networkID string, req AddNetworkDeviceRequest) (NetworkDevice, bool, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	device, err := s.store.AddNetworkDevice(networkID, req.DeviceID, req.ActorUserID, req.Alias, enabled)
	return device, enabled, err
}

func (s NetworkService) UpdateNetworkDevice(networkID, deviceID string, req UpdateNetworkDeviceRequest) (NetworkDevice, error) {
	return s.store.UpdateNetworkDevice(networkID, deviceID, req.Alias, req.Enabled)
}

func (s NetworkService) RemoveNetworkDevice(networkID, deviceID string) error {
	return s.store.RemoveNetworkDevice(networkID, deviceID)
}

func (s NetworkService) ListDNSZones(networkID string) []NetworkDNSZone {
	return s.store.ListDNSZones(networkID)
}

func (s NetworkService) AddDNSZone(networkID string, req DNSZoneRequest) (NetworkDNSZone, error) {
	return s.store.AddDNSZone(networkID, req.ZoneName, req.ExposeGlobal)
}

func (s NetworkService) UpdateDNSZone(networkID, zoneID string, req DNSZoneRequest) (NetworkDNSZone, error) {
	return s.store.UpdateDNSZone(networkID, zoneID, req.ZoneName, req.ExposeGlobal)
}

func (s NetworkService) DeleteDNSZone(networkID, zoneID string) error {
	return s.store.DeleteDNSZone(networkID, zoneID)
}

func (s NetworkService) ListDNSRecords(networkID string) []NetworkDNSRecord {
	return s.store.ListDNSRecords(networkID)
}

func (s NetworkService) AddDNSRecord(networkID string, req AddDNSRecordRequest) (NetworkDNSRecord, error) {
	return s.store.AddDNSRecord(networkID, req.ZoneID, req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
}

func (s NetworkService) UpdateDNSRecord(networkID, recordID string, req DNSRecordRequest) (NetworkDNSRecord, error) {
	return s.store.UpdateDNSRecord(networkID, recordID, req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
}

func (s NetworkService) DeleteDNSRecord(networkID, recordID string) error {
	return s.store.DeleteDNSRecord(networkID, recordID)
}

func (s NetworkService) ListPublicMappings(networkID string) []PublicDomainMapping {
	return s.store.ListPublicMappings(networkID)
}

func (s NetworkService) UpsertPublicMapping(networkID, mappingID string, req PublicMappingRequest) (PublicDomainMapping, error) {
	return s.store.UpsertPublicMapping(mappingID, networkID, req.Alias, req.PublicDomain, req.SourceRecord, req.DeviceID, req.Protocol, req.Port, req.ExternalPort, req.Status)
}

func (s NetworkService) DeletePublicMapping(networkID, mappingID string) error {
	return s.store.DeletePublicMapping(networkID, mappingID)
}

func (s NetworkService) ListSecurityGroups(networkID string) []SecurityGroup {
	return s.store.ListSecurityGroups(networkID)
}

func (s NetworkService) CreateSecurityGroup(networkID string, req CreateSecurityGroupRequest) (SecurityGroup, error) {
	return s.store.CreateSecurityGroup(networkID, req.Name, req.Description)
}

func (s NetworkService) DeleteSecurityGroup(networkID, securityGroupID string) error {
	return s.store.DeleteSecurityGroup(networkID, securityGroupID)
}

func (s NetworkService) ListSecurityRules(securityGroupID string) []SecurityGroupRule {
	return s.store.ListSecurityGroupRules(securityGroupID)
}

func (s NetworkService) AddSecurityRule(securityGroupID string, req SecurityRuleRequest) (SecurityGroupRule, bool, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.AddSecurityGroupRule(securityGroupID, req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	return rule, enabled, err
}

func (s NetworkService) UpdateSecurityRule(ruleID string, req SecurityRuleRequest) (SecurityGroupRule, bool, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.UpdateSecurityGroupRule(ruleID, req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	return rule, enabled, err
}

func (s NetworkService) DeleteSecurityRule(ruleID string) (SecurityGroupRule, string, error) {
	rule, err := s.store.GetSecurityRule(ruleID)
	if err != nil {
		return SecurityGroupRule{}, "", err
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err != nil {
		return SecurityGroupRule{}, "", err
	}
	if err := s.store.DeleteSecurityGroupRule(ruleID); err != nil {
		return SecurityGroupRule{}, "", err
	}
	return rule, networkID, nil
}

func (s NetworkService) SecurityGroupNetworkID(securityGroupID string) (string, error) {
	return s.store.SecurityGroupNetworkID(securityGroupID)
}

func (s NetworkService) NetworkConfig(networkID, deviceID string) (NetworkConfig, error) {
	return s.store.NetworkConfig(networkID, deviceID)
}

func (s NetworkService) RelayCandidates(networkID, deviceID string) ([]RelayCandidate, error) {
	return s.store.RelayCandidates(networkID, deviceID)
}

func (s NetworkService) ListNetworkDevicesForUser(userID string) []NetworkDevice {
	return s.store.ListNetworkDevicesForUser(userID)
}

func (s NetworkService) ListNetworkDevicesForDevice(deviceID string) []NetworkDevice {
	return s.store.ListNetworkDevicesForDevice(deviceID)
}
