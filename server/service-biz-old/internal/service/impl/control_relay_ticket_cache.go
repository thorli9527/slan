package impl

import (
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
)

// relayTicketFromCache returns a still-valid cached relay ticket for the exact
// same request dimensions.
func (s *dbState) relayTicketFromCache(req dto.RelayTicketRequest) (dto.RelayTicket, bool) {
	key := relayTicketCacheKey(req)
	now := time.Now()

	s.controlMu.Lock()
	defer s.controlMu.Unlock()

	cached, ok := s.cachedRelayTicket[key]
	if !ok {
		return dto.RelayTicket{}, false
	}
	if !cached.validUntil.After(now) {
		delete(s.cachedRelayTicket, key)
		return dto.RelayTicket{}, false
	}
	return cached.ticket, true
}

// storeRelayTicket caches a relay ticket until shortly before its expiry time.
func (s *dbState) storeRelayTicket(req dto.RelayTicketRequest, ticket dto.RelayTicket) {
	expiresAt, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil {
		return
	}

	validUntil := expiresAt.Add(-time.Minute)
	now := time.Now()
	if !validUntil.After(now) {
		validUntil = now.Add(30 * time.Second)
	}

	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.cachedRelayTicket[relayTicketCacheKey(req)] = cachedRelayTicket{
		ticket:     ticket,
		validUntil: validUntil,
	}
}

// pruneExpiredRelayTickets clears stale relay tickets from the in-memory cache.
func (s *dbState) pruneExpiredRelayTickets(now time.Time) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()

	for key, cached := range s.cachedRelayTicket {
		if !cached.validUntil.After(now) {
			delete(s.cachedRelayTicket, key)
		}
	}
}

// relayTicketCacheKey canonicalizes request fields that affect the signed
// ticket so the cache remains stable across equivalent input ordering.
func relayTicketCacheKey(req dto.RelayTicketRequest) string {
	preferredNodeIDs := append([]string(nil), req.PreferredDerpNodeIDs...)
	sort.Strings(preferredNodeIDs)
	return strings.Join([]string{
		req.NetworkID,
		req.SrcNodeID,
		req.DstNodeID,
		req.DerpClusterID,
		strings.Join(preferredNodeIDs, ","),
		req.Reason,
	}, "|")
}
