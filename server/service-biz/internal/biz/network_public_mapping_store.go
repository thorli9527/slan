package biz

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListPublicMappings(networkID string) []PublicDomainMapping {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres public mappings failed: %v", err)
	}
	out := make([]PublicDomainMapping, 0)
	for _, mapping := range s.publicMappings {
		if networkID == "" || mapping.NetworkID == networkID {
			out = append(out, mapping)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublicDomain < out[j].PublicDomain })
	return out
}

func (s *Store) UpsertPublicMapping(mappingID, networkID, alias, publicDomain, sourceRecord, deviceID, protocol, port, externalPort, status string) (PublicDomainMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return PublicDomainMapping{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return PublicDomainMapping{}, errNotFound
	}
	deviceID = strings.TrimSpace(deviceID)
	device, ok := s.devices[deviceID]
	if !ok {
		return PublicDomainMapping{}, errNotFound
	}
	now := time.Now().Unix()
	mapping := PublicDomainMapping{}
	if mappingID != "" {
		existing, ok := s.publicMappings[mappingID]
		if !ok || existing.NetworkID != networkID {
			return PublicDomainMapping{}, errNotFound
		}
		mapping = existing
	} else {
		mapping.MappingID = fmt.Sprintf("pub-%06d", s.nextPublicMapSeq)
		mapping.NetworkID = networkID
		mapping.CreatedAt = now
		s.nextPublicMapSeq++
	}
	mapping.Alias = sanitizeDNSLabel(alias)
	mapping.PublicDomain = strings.TrimSpace(publicDomain)
	mapping.SourceRecord = defaultString(strings.TrimSpace(sourceRecord), defaultString(device.Alias, device.DeviceID))
	mapping.DeviceID = deviceID
	mapping.Protocol = defaultString(protocol, "HTTP")
	mapping.ExternalPort = strings.TrimSpace(externalPort)
	mapping.Port = defaultString(strings.TrimSpace(port), mapping.ExternalPort)
	mapping.Status = defaultString(status, "enabled")
	mapping.UpdatedAt = now
	s.publicMappings[mapping.MappingID] = mapping
	if err := s.persistPostgresPublicMappingUpsertTxLocked(ctx, postgresTx, mapping); err != nil {
		return PublicDomainMapping{}, err
	}
	postgresTx = nil
	return mapping, nil
}

func (s *Store) DeletePublicMapping(networkID, mappingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	mapping, ok := s.publicMappings[mappingID]
	if !ok || mapping.NetworkID != networkID {
		return errNotFound
	}
	delete(s.publicMappings, mappingID)
	if err := s.persistPostgresPublicMappingDeleteTxLocked(ctx, postgresTx, networkID, mappingID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}
