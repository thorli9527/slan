package repository

import "fmt"

func (s *GormStore) NewUserID() string   { return s.nextID("user", "user") }
func (s *GormStore) NewDeviceID() string { return s.nextID("device", "dev") }
func (s *GormStore) NewDeviceVirtualIPID() string {
	value := s.nextCounterValue("device_virtual_ip")
	if value <= 0 {
		return "vip-000001"
	}
	return fmt.Sprintf("vip-%06d", value)
}
func (s *GormStore) NewNetworkID() string { return s.nextID("network", "net") }
func (s *GormStore) NewInviteID() string  { return s.nextID("invite", "invite") }
func (s *GormStore) NewSessionID(prefix string) string {
	if prefix == "" {
		prefix = "sess"
	}
	return s.nextID("session:"+prefix, prefix)
}
func (s *GormStore) NewOperatorID() string           { return s.nextID("operator", "op") }
func (s *GormStore) NewDeviceBootstrapKeyID() string { return s.nextID("device_bootstrap_key", "dbk") }
func (s *GormStore) NewDeviceGroupID() string        { return s.nextID("device_group", "dgrp") }
func (s *GormStore) NewDNSZoneID() string            { return s.nextID("dns_zone", "zone") }
func (s *GormStore) NewDNSRecordID() string          { return s.nextID("dns_record", "rec") }
func (s *GormStore) NewPublicMappingID() string      { return s.nextID("public_mapping", "map") }
func (s *GormStore) NewSecurityGroupID() string      { return s.nextID("security_group", "sg") }
func (s *GormStore) NewSecurityRuleID() string       { return s.nextID("security_rule", "sgr") }
func (s *GormStore) NewRelayNodeID() string          { return s.nextID("relay_node", "relay") }
func (s *GormStore) NewPunchNodeID() string          { return s.nextID("punch_node", "punch") }
