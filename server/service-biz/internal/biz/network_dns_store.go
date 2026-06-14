package biz

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) AddDNSZone(networkID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return NetworkDNSZone{}, errNotFound
	}
	zone := s.addDNSZoneLocked(networkID, zoneName, exposeGlobal, time.Now().Unix())
	if err := s.persistPostgresDNSZoneUpsertTxLocked(ctx, postgresTx, zone); err != nil {
		return NetworkDNSZone{}, err
	}
	postgresTx = nil
	return zone, nil
}

func (s *Store) UpdateDNSZone(networkID, zoneID, zoneName string, exposeGlobal bool) (NetworkDNSZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return NetworkDNSZone{}, errNotFound
	}
	if zoneName = strings.TrimSpace(zoneName); zoneName != "" {
		zone.ZoneName = zoneName
	}
	zone.ExposeGlobal = exposeGlobal
	s.dnsZones[zoneID] = zone
	if err := s.persistPostgresDNSZoneUpsertTxLocked(ctx, postgresTx, zone); err != nil {
		return NetworkDNSZone{}, err
	}
	postgresTx = nil
	return zone, nil
}

func (s *Store) DeleteDNSZone(networkID, zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return errNotFound
	}
	delete(s.dnsZones, zoneID)
	for recordID, record := range s.dnsRecords {
		if record.ZoneID == zoneID {
			delete(s.dnsRecords, recordID)
		}
	}
	if err := s.persistPostgresDNSZoneDeleteTxLocked(ctx, postgresTx, networkID, zoneID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) AddDNSRecord(networkID, zoneID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSRecord{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	zone, ok := s.dnsZones[zoneID]
	if !ok || zone.NetworkID != networkID {
		return NetworkDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return NetworkDNSRecord{}, errNotFound
		}
		if targetIP == "" {
			targetIP = device.GlobalIP
		}
	}
	now := time.Now().Unix()
	record := NetworkDNSRecord{
		RecordID:       newCompactUUID(),
		ZoneID:         zoneID,
		NetworkID:      networkID,
		Name:           name,
		FQDN:           sanitizeDNSLabel(name) + "." + strings.TrimSuffix(zone.ZoneName, "."),
		RecordType:     defaultString(recordType, "A"),
		TargetDeviceID: strings.TrimSpace(targetDeviceID),
		TargetIP:       strings.TrimSpace(targetIP),
		CNAME:          strings.TrimSpace(cname),
		Port:           strings.TrimSpace(port),
		TTL:            defaultInt(ttl, 60),
		Status:         "active",
		CreatedAt:      now,
	}
	s.nextRecordSeq++
	s.dnsRecords[record.RecordID] = record
	if err := s.persistPostgresDNSRecordUpsertTxLocked(ctx, postgresTx, record); err != nil {
		return NetworkDNSRecord{}, err
	}
	postgresTx = nil
	return record, nil
}

func (s *Store) UpdateDNSRecord(networkID, recordID, name, recordType, targetDeviceID, targetIP, cname, port string, ttl int) (NetworkDNSRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkDNSRecord{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDNSRecord{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	record, ok := s.dnsRecords[recordID]
	if !ok || record.NetworkID != networkID {
		return NetworkDNSRecord{}, errNotFound
	}
	zone, ok := s.dnsZones[record.ZoneID]
	if !ok {
		return NetworkDNSRecord{}, errNotFound
	}
	if targetDeviceID != "" {
		device, ok := s.devices[targetDeviceID]
		if !ok {
			return NetworkDNSRecord{}, errNotFound
		}
		if targetIP == "" {
			targetIP = device.GlobalIP
		}
	}
	record.Name = name
	record.FQDN = sanitizeDNSLabel(name) + "." + strings.TrimSuffix(zone.ZoneName, ".")
	record.RecordType = defaultString(recordType, "A")
	record.TargetDeviceID = strings.TrimSpace(targetDeviceID)
	record.TargetIP = strings.TrimSpace(targetIP)
	record.CNAME = strings.TrimSpace(cname)
	record.Port = strings.TrimSpace(port)
	record.TTL = defaultInt(ttl, 60)
	s.dnsRecords[recordID] = record
	if err := s.persistPostgresDNSRecordUpsertTxLocked(ctx, postgresTx, record); err != nil {
		return NetworkDNSRecord{}, err
	}
	postgresTx = nil
	return record, nil
}

func (s *Store) DeleteDNSRecord(networkID, recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	record, ok := s.dnsRecords[recordID]
	if !ok || record.NetworkID != networkID {
		return errNotFound
	}
	delete(s.dnsRecords, recordID)
	if err := s.persistPostgresDNSRecordDeleteTxLocked(ctx, postgresTx, networkID, recordID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) ListDNSZones(networkID string) []NetworkDNSZone {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres dns zones failed: %v", err)
	}
	out := make([]NetworkDNSZone, 0)
	for _, zone := range s.dnsZones {
		if networkID == "" || zone.NetworkID == networkID {
			out = append(out, zone)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ZoneName < out[j].ZoneName })
	return out
}

func (s *Store) ListDNSRecords(networkID string) []NetworkDNSRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres dns records failed: %v", err)
	}
	out := make([]NetworkDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if networkID == "" || record.NetworkID == networkID {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FQDN < out[j].FQDN })
	return out
}
