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
	return s.Networks.DeleteDNSZone(ctx, input.ZoneID)
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
	now := networkNow(s.Now).Unix()
	item := newManagedDNSRecord(newManagedDNSRecordID(s.Networks, s.NewDNSRecordID), now, input)
	if err := s.Networks.SaveDNSRecord(ctx, item); err != nil {
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
	if err := s.Networks.SaveDNSRecord(ctx, item); err != nil {
		return DNSRecordView{}, err
	}
	return dnsRecordView(item), nil
}

func (s NetworkDNSService) DeleteDNSRecord(ctx context.Context, input DeleteDNSRecordInput) error {
	input = normalizeDeleteDNSRecordInput(input)
	if input.RecordID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedDNSRecord(ctx, s.Users, s.Networks, input.ActorUserID, input.RecordID); err != nil {
		return err
	}
	return s.Networks.DeleteDNSRecord(ctx, input.RecordID)
}
