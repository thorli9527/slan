package service

import "context"

func (s NetworkDNSService) ListDNSZones(ctx context.Context, networkID string) ([]DNSZoneView, error) {
	items, err := s.Networks.ListDNSZones(ctx, normalizeNetworkID(networkID))
	if err != nil {
		return nil, err
	}
	return dnsZoneViews(items), nil
}

func (s NetworkDNSService) AddDNSZone(ctx context.Context, input CreateDNSZoneInput) (DNSZoneView, error) {
	input = normalizeCreateDNSZoneInput(input)
	if input.NetworkID == "" || input.Name == "" {
		return DNSZoneView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return DNSZoneView{}, err
	}
	now := networkNow(s.Now).Unix()
	item := newManagedDNSZone(newManagedDNSZoneID(s.Networks, s.NewDNSZoneID), now, input)
	if err := s.Networks.SaveDNSZone(ctx, item); err != nil {
		return DNSZoneView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "dns_zone_created")
	if err != nil {
		return DNSZoneView{}, err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSZoneView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSZoneView{}, err
	}
	return dnsZoneView(item), nil
}

func (s NetworkDNSService) UpdateDNSZone(ctx context.Context, input UpdateDNSZoneInput) (DNSZoneView, error) {
	input = normalizeUpdateDNSZoneInput(input)
	if input.ZoneID == "" {
		return DNSZoneView{}, ErrInvalidArgument
	}
	item, err := requireOwnedManagedDNSZone(ctx, s.Users, s.Networks, input.ActorUserID, input.ZoneID)
	if err != nil {
		return DNSZoneView{}, err
	}
	item = applyUpdateDNSZoneInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveDNSZone(ctx, item); err != nil {
		return DNSZoneView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "dns_zone_updated")
	if err != nil {
		return DNSZoneView{}, err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSZoneView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSZoneView{}, err
	}
	return dnsZoneView(item), nil
}

func (s NetworkDNSService) DeleteDNSZone(ctx context.Context, input DeleteDNSZoneInput) error {
	input = normalizeDeleteDNSZoneInput(input)
	if input.ZoneID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedDNSZone(ctx, s.Users, s.Networks, input.ActorUserID, input.ZoneID); err != nil {
		return err
	}
	zone, err := requireManagedDNSZone(ctx, s.Networks, input.ZoneID)
	if err != nil {
		return err
	}
	if err := s.Networks.DeleteDNSZone(ctx, input.ZoneID); err != nil {
		return err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, zone.NetworkID, "dns_zone_deleted")
	if err != nil {
		return err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, zone.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	return publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, zone.NetworkID, version.Version, version.Reason)
}

func (s NetworkDNSService) ListDNSRecords(ctx context.Context, networkID string) ([]DNSRecordView, error) {
	items, err := s.Networks.ListDNSRecords(ctx, normalizeNetworkID(networkID))
	if err != nil {
		return nil, err
	}
	return dnsRecordViews(items), nil
}

func (s NetworkDNSService) AddDNSRecord(ctx context.Context, input CreateDNSRecordInput) (DNSRecordView, error) {
	input = normalizeCreateDNSRecordInput(input)
	if input.NetworkID == "" || input.Name == "" || input.Type == "" || input.Value == "" {
		return DNSRecordView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return DNSRecordView{}, err
	}
	if err := validateManagedDNSRecordTarget(ctx, s.Networks, input.NetworkID, input.Type, input.Value); err != nil {
		return DNSRecordView{}, err
	}
	now := networkNow(s.Now).Unix()
	item := newManagedDNSRecord(newManagedDNSRecordID(s.Networks, s.NewDNSRecordID), now, input)
	if err := s.Networks.SaveDNSRecord(ctx, item); err != nil {
		return DNSRecordView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "dns_record_created")
	if err != nil {
		return DNSRecordView{}, err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSRecordView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSRecordView{}, err
	}
	return dnsRecordView(item), nil
}

func (s NetworkDNSService) UpdateDNSRecord(ctx context.Context, input UpdateDNSRecordInput) (DNSRecordView, error) {
	input = normalizeUpdateDNSRecordInput(input)
	if input.RecordID == "" {
		return DNSRecordView{}, ErrInvalidArgument
	}
	item, err := requireOwnedManagedDNSRecord(ctx, s.Users, s.Networks, input.ActorUserID, input.RecordID)
	if err != nil {
		return DNSRecordView{}, err
	}
	item = applyUpdateDNSRecordInput(item, input, networkNow(s.Now).Unix())
	if err := validateManagedDNSRecordTarget(ctx, s.Networks, item.NetworkID, item.Type, item.Value); err != nil {
		return DNSRecordView{}, err
	}
	if err := s.Networks.SaveDNSRecord(ctx, item); err != nil {
		return DNSRecordView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "dns_record_updated")
	if err != nil {
		return DNSRecordView{}, err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSRecordView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return DNSRecordView{}, err
	}
	return dnsRecordView(item), nil
}

func (s NetworkDNSService) DeleteDNSRecord(ctx context.Context, input DeleteDNSRecordInput) error {
	input = normalizeDeleteDNSRecordInput(input)
	if input.RecordID == "" {
		return ErrInvalidArgument
	}
	record, err := requireOwnedManagedDNSRecord(ctx, s.Users, s.Networks, input.ActorUserID, input.RecordID)
	if err != nil {
		return err
	}
	if err := s.Networks.DeleteDNSRecord(ctx, input.RecordID); err != nil {
		return err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, record.NetworkID, "dns_record_deleted")
	if err != nil {
		return err
	}
	if err := publishDNSChanged(ctx, s.Networks, s.EventPublisher, s.Now, record.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	return publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, record.NetworkID, version.Version, version.Reason)
}
