package biz

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListIPAMSubnets() []IPAMSubnet {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres ipam subnets failed: %v", err)
	}
	return sortedValues(s.ipamSubnets, func(a, b IPAMSubnet) bool { return a.StartOffset < b.StartOffset })
}

func (s *Store) CreateNetwork(ownerUserID, name, code, templateKey string) (Network, SecurityGroup, NetworkDNSZone, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	name = strings.TrimSpace(name)
	if ownerUserID == "" || name == "" {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.users[ownerUserID]; !ok {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, errNotFound
	}
	now := time.Now().Unix()
	id := fmt.Sprintf("network-%06d", s.nextNetworkSeq)
	s.nextNetworkSeq++
	network := Network{NetworkID: id, OwnerUserID: ownerUserID, Name: name, Code: defaultString(sanitizeDNSLabel(code), sanitizeDNSLabel(name)), TemplateKey: defaultString(templateKey, "custom"), Status: "enabled", CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	group := s.addSecurityGroupLocked(id, "默认安全组", "网络默认安全组", "deny", now)
	zone := s.addDNSZoneLocked(id, networkZoneName(network), false, now)
	if err := s.persistPostgresNetworkCreateTxLocked(ctx, postgresTx, network, group, zone); err != nil {
		return Network{}, SecurityGroup{}, NetworkDNSZone{}, err
	}
	postgresTx = nil
	return network, group, zone, nil
}

func (s *Store) ListNetworks(userID string) []Network {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres networks failed: %v", err)
	}
	out := make([]Network, 0)
	for _, network := range s.networks {
		if network.Status == "deleted" {
			continue
		}
		if userID == "" || network.OwnerUserID == userID {
			out = append(out, network)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NetworkID < out[j].NetworkID })
	return out
}

func (s *Store) UpdateNetwork(networkID, name, status string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	network, ok := s.networks[networkID]
	if !ok {
		return Network{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		network.Name = name
	}
	if status = strings.TrimSpace(status); status != "" {
		network.Status = status
	}
	network.UpdatedAt = time.Now().Unix()
	s.networks[networkID] = network
	if err := s.persistPostgresNetworkUpdateTxLocked(ctx, postgresTx, network); err != nil {
		return Network{}, err
	}
	postgresTx = nil
	return network, nil
}

func (s *Store) UpdateNetworkFull(networkID, name, code, status string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	network, ok := s.networks[networkID]
	if !ok {
		return Network{}, errNotFound
	}
	if name = strings.TrimSpace(name); name != "" {
		network.Name = name
	}
	if code = sanitizeDNSLabel(code); code != "" {
		for _, existing := range s.networks {
			if existing.NetworkID != networkID && existing.OwnerUserID == network.OwnerUserID && existing.Code == code && existing.Status != "deleted" {
				return Network{}, errConflict
			}
		}
		network.Code = code
		network.TemplateKey = defaultString(network.TemplateKey, code)
	}
	if status = strings.TrimSpace(status); status != "" {
		network.Status = status
	}
	network.UpdatedAt = time.Now().Unix()
	s.networks[networkID] = network
	if err := s.persistPostgresNetworkUpdateTxLocked(ctx, postgresTx, network); err != nil {
		return Network{}, err
	}
	postgresTx = nil
	return network, nil
}
