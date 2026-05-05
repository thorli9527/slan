package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/slan/server/server-wire/internal/model"
)

type PostgresStore struct {
	db      *sql.DB
	derpMap model.DerpMap
}

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	st := &PostgresStore{db: db, derpMap: defaultDerpMap()}
	if err := st.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS wire_peers (
			peer_id TEXT PRIMARY KEY,
			network_id TEXT NOT NULL DEFAULT '',
			node_id TEXT NOT NULL DEFAULT '',
			public_key TEXT NOT NULL DEFAULT '',
			virtual_ips JSONB NOT NULL DEFAULT '[]',
			allowed_ips JSONB NOT NULL DEFAULT '[]',
			supports_lan_direct BOOLEAN NOT NULL DEFAULT false,
			supports_ipv6_direct BOOLEAN NOT NULL DEFAULT false,
			supports_direct_udp BOOLEAN NOT NULL DEFAULT false,
			supports_relay_udp BOOLEAN NOT NULL DEFAULT false,
			supports_derp_tcp_tls_443 BOOLEAN NOT NULL DEFAULT false,
			prefer_ipv6 BOOLEAN NOT NULL DEFAULT false,
			prefer_lan BOOLEAN NOT NULL DEFAULT false,
			allow_endpoint_roaming BOOLEAN NOT NULL DEFAULT false,
			allow_fast_reselection BOOLEAN NOT NULL DEFAULT false,
			allow_relay_ticket_renewal BOOLEAN NOT NULL DEFAULT false,
			keepalive_interval_secs INTEGER NOT NULL DEFAULT 0,
			endpoints JSONB NOT NULL DEFAULT '[]',
			relay_ticket JSONB NOT NULL DEFAULT '{}',
			recent_path_downgrades INTEGER NOT NULL DEFAULT 0,
			recent_path_upgrades INTEGER NOT NULL DEFAULT 0,
			require_mtu_refresh BOOLEAN NOT NULL DEFAULT false,
			endpoint_changed BOOLEAN NOT NULL DEFAULT false,
			updated_at_ms BIGINT NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS wire_path_probes (
			peer_id TEXT NOT NULL REFERENCES wire_peers(peer_id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			reachable BOOLEAN NOT NULL DEFAULT false,
			rtt_ms INTEGER NOT NULL DEFAULT 0,
			loss_ppm INTEGER NOT NULL DEFAULT 0,
			jitter_ms INTEGER NOT NULL DEFAULT 0,
			consecutive_fails INTEGER NOT NULL DEFAULT 0,
			mtu INTEGER NOT NULL DEFAULT 0,
			observed_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (peer_id, path)
		)`,
		`CREATE TABLE IF NOT EXISTS wire_derp_health (
			peer_id TEXT NOT NULL REFERENCES wire_peers(peer_id) ON DELETE CASCADE,
			region_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			reachable BOOLEAN NOT NULL DEFAULT false,
			rtt_ms INTEGER NOT NULL DEFAULT 0,
			observed_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (peer_id, region_id, node_id)
		)`,
		`CREATE TABLE IF NOT EXISTS wire_active_path (
			peer_id TEXT PRIMARY KEY REFERENCES wire_peers(peer_id) ON DELETE CASCADE,
			path TEXT NOT NULL DEFAULT '',
			updated_at_ms BIGINT NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wire_peers_network_id ON wire_peers(network_id)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) RegisterPeer(reg model.PeerRegistration) (model.PeerRecord, error) {
	if reg.PeerID == "" {
		return model.PeerRecord{}, fmt.Errorf("peerId is required")
	}
	ctx := context.Background()
	current, ok := s.GetPeer(reg.PeerID)
	endpoints := reg.Endpoints
	if ok && len(current.Endpoints) > 0 {
		endpoints = current.Endpoints
	}
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `INSERT INTO wire_peers (
		peer_id, network_id, node_id, public_key, virtual_ips, allowed_ips,
		supports_lan_direct, supports_ipv6_direct, supports_direct_udp, supports_relay_udp, supports_derp_tcp_tls_443,
		prefer_ipv6, prefer_lan, allow_endpoint_roaming, allow_fast_reselection, allow_relay_ticket_renewal,
		keepalive_interval_secs, endpoints, updated_at_ms
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	ON CONFLICT (peer_id) DO UPDATE SET
		network_id = EXCLUDED.network_id,
		node_id = EXCLUDED.node_id,
		public_key = EXCLUDED.public_key,
		virtual_ips = EXCLUDED.virtual_ips,
		allowed_ips = EXCLUDED.allowed_ips,
		supports_lan_direct = EXCLUDED.supports_lan_direct,
		supports_ipv6_direct = EXCLUDED.supports_ipv6_direct,
		supports_direct_udp = EXCLUDED.supports_direct_udp,
		supports_relay_udp = EXCLUDED.supports_relay_udp,
		supports_derp_tcp_tls_443 = EXCLUDED.supports_derp_tcp_tls_443,
		prefer_ipv6 = EXCLUDED.prefer_ipv6,
		prefer_lan = EXCLUDED.prefer_lan,
		allow_endpoint_roaming = EXCLUDED.allow_endpoint_roaming,
		allow_fast_reselection = EXCLUDED.allow_fast_reselection,
		allow_relay_ticket_renewal = EXCLUDED.allow_relay_ticket_renewal,
		keepalive_interval_secs = EXCLUDED.keepalive_interval_secs,
		endpoints = EXCLUDED.endpoints,
		updated_at_ms = EXCLUDED.updated_at_ms`,
		reg.PeerID, reg.NetworkID, reg.NodeID, reg.PublicKey, jsonValue(reg.VirtualIPs), jsonValue(reg.AllowedIPs),
		reg.SupportsLANDirect, reg.SupportsIPv6Direct, reg.SupportsDirectUDP, reg.SupportsRelayUDP, reg.SupportsDerpTCPTLS443,
		reg.PreferIPv6, reg.PreferLAN, reg.AllowEndpointRoaming, reg.AllowFastReselection, reg.AllowRelayTicketRenewal,
		reg.KeepaliveIntervalSecs, jsonValue(endpoints), now,
	)
	if err != nil {
		return model.PeerRecord{}, err
	}
	peer, ok := s.GetPeer(reg.PeerID)
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	return peer, nil
}

func (s *PostgresStore) UpdateEndpoints(peerID string, endpoints []model.Endpoint) (model.PeerRecord, error) {
	res, err := s.db.ExecContext(context.Background(), `UPDATE wire_peers SET endpoints=$2, endpoint_changed=true, require_mtu_refresh=true, updated_at_ms=$3 WHERE peer_id=$1`, peerID, jsonValue(endpoints), time.Now().UnixMilli())
	if err != nil {
		return model.PeerRecord{}, err
	}
	return s.changedPeer(peerID, res)
}

func (s *PostgresStore) DerpMap() model.DerpMap {
	return cloneDerpMap(s.derpMap)
}

func (s *PostgresStore) IssueDerpTicket(peerID, regionID, nodeID string, ttl time.Duration, renewAfter time.Duration) (model.DerpTicket, error) {
	current, ok := s.GetPeer(peerID)
	if !ok {
		return model.DerpTicket{}, ErrPeerNotFound
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	derpMap := cloneDerpMap(s.derpMap)
	if regionID == "" {
		regionID = derpMap.PreferredRegionID
	}
	node := model.DerpNode{RegionID: regionID, NodeID: nodeID}
	if regionID == "" || nodeID == "" {
		found := findDerpNode(derpMap, regionID, nodeID)
		if found == nil {
			return model.DerpTicket{}, fmt.Errorf("derp node not found")
		}
		node = *found
	}
	ticket := model.DerpTicket{
		TicketID:     randomID("derp"),
		PeerID:       peerID,
		NetworkID:    current.NetworkID,
		Path:         model.PathDerpTCP443,
		RegionID:     node.RegionID,
		NodeID:       node.NodeID,
		ExpiresAt:    time.Now().Add(ttl),
		ExpiresInMs:  ttl.Milliseconds(),
		RenewAfterMs: renewAfter.Milliseconds(),
	}
	ticket.Signature = signDerpTicket(ticket)
	return ticket, nil
}

func (s *PostgresStore) UpdatePathHealth(peerID string, probes []model.PathProbe) (model.PeerRecord, error) {
	if !s.peerExists(peerID) {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PeerRecord{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM wire_path_probes WHERE peer_id=$1`, peerID); err != nil {
		return model.PeerRecord{}, err
	}
	for _, probe := range probes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO wire_path_probes (peer_id,path,reachable,rtt_ms,loss_ppm,jitter_ms,consecutive_fails,mtu,observed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			peerID, string(probe.Path), probe.Reachable, probe.RTTMs, probe.LossPPM, probe.JitterMs, probe.ConsecutiveFails, probe.MTU, probe.ObservedAt); err != nil {
			return model.PeerRecord{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE wire_peers SET updated_at_ms=$2 WHERE peer_id=$1`, peerID, time.Now().UnixMilli()); err != nil {
		return model.PeerRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PeerRecord{}, err
	}
	peer, _ := s.GetPeer(peerID)
	return peer, nil
}

func (s *PostgresStore) UpdateDerpHealth(peerID string, samples []model.DerpHealthSample) (model.PeerRecord, error) {
	if !s.peerExists(peerID) {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PeerRecord{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM wire_derp_health WHERE peer_id=$1`, peerID); err != nil {
		return model.PeerRecord{}, err
	}
	for _, sample := range samples {
		if _, err := tx.ExecContext(ctx, `INSERT INTO wire_derp_health (peer_id,region_id,node_id,reachable,rtt_ms,observed_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			peerID, sample.RegionID, sample.NodeID, sample.Reachable, sample.RTTMs, sample.ObservedAt); err != nil {
			return model.PeerRecord{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE wire_peers SET updated_at_ms=$2 WHERE peer_id=$1`, peerID, time.Now().UnixMilli()); err != nil {
		return model.PeerRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PeerRecord{}, err
	}
	peer, _ := s.GetPeer(peerID)
	return peer, nil
}

func (s *PostgresStore) UpdateActivePath(peerID string, path model.PathKind) (model.PeerRecord, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PeerRecord{}, err
	}
	defer tx.Rollback()

	var currentPath sql.NullString
	var downgrades int
	var upgrades int
	err = tx.QueryRowContext(ctx, `
SELECT COALESCE(ap.path, ''), p.recent_path_downgrades, p.recent_path_upgrades
FROM wire_peers p
LEFT JOIN wire_active_path ap ON ap.peer_id = p.peer_id
WHERE p.peer_id = $1
FOR UPDATE OF p
`, peerID).Scan(&currentPath, &downgrades, &upgrades)
	if err == sql.ErrNoRows {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	if err != nil {
		return model.PeerRecord{}, err
	}
	if currentPath.String != "" && model.PathKind(currentPath.String) != path {
		if path == model.PathRelayUDP {
			downgrades++
		} else {
			upgrades++
		}
	}
	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `INSERT INTO wire_active_path (peer_id,path,updated_at_ms) VALUES ($1,$2,$3) ON CONFLICT (peer_id) DO UPDATE SET path=EXCLUDED.path, updated_at_ms=EXCLUDED.updated_at_ms`, peerID, string(path), now); err != nil {
		return model.PeerRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE wire_peers SET recent_path_downgrades=$2, recent_path_upgrades=$3, endpoint_changed=false, require_mtu_refresh=false, updated_at_ms=$4 WHERE peer_id=$1`, peerID, downgrades, upgrades, now); err != nil {
		return model.PeerRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PeerRecord{}, err
	}
	peer, _ := s.GetPeer(peerID)
	return peer, nil
}

func (s *PostgresStore) IssueRelayTicket(peerID string, relay model.RelayNode, ttl time.Duration, renewAfter time.Duration) (model.RelayTicket, error) {
	if !s.peerExists(peerID) {
		return model.RelayTicket{}, ErrPeerNotFound
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	ticket := model.RelayTicket{
		TicketID:     randomID("relay"),
		PeerID:       peerID,
		SessionID:    randomID("relay-session"),
		Path:         model.PathRelayUDP,
		RegionID:     relay.RegionID,
		NodeID:       relay.NodeID,
		Host:         relay.Host,
		UDPPort:      relay.UDPPort,
		Present:      true,
		ExpiresAt:    time.Now().Add(ttl),
		ExpiresInMs:  ttl.Milliseconds(),
		RenewAfterMs: renewAfter.Milliseconds(),
	}
	ticket.Signature = signRelayTicket(ticket)
	_, err := s.db.ExecContext(context.Background(), `UPDATE wire_peers SET relay_ticket=$2, updated_at_ms=$3 WHERE peer_id=$1`, peerID, jsonValue(ticket), time.Now().UnixMilli())
	if err != nil {
		return model.RelayTicket{}, err
	}
	return ticket, nil
}

func (s *PostgresStore) GetPeer(peerID string) (model.PeerRecord, bool) {
	var peer model.PeerRecord
	var virtualIPs, allowedIPs, endpoints, relayTicket []byte
	var activePath sql.NullString
	row := s.db.QueryRowContext(context.Background(), `SELECT
		p.peer_id, p.network_id, p.node_id, p.public_key, p.virtual_ips, p.allowed_ips,
		p.supports_lan_direct, p.supports_ipv6_direct, p.supports_direct_udp, p.supports_relay_udp, p.supports_derp_tcp_tls_443,
		p.prefer_ipv6, p.prefer_lan, p.allow_endpoint_roaming, p.allow_fast_reselection, p.allow_relay_ticket_renewal,
		p.keepalive_interval_secs, p.endpoints, p.relay_ticket, p.recent_path_downgrades, p.recent_path_upgrades,
		p.require_mtu_refresh, p.endpoint_changed, p.updated_at_ms, a.path
		FROM wire_peers p LEFT JOIN wire_active_path a ON a.peer_id = p.peer_id WHERE p.peer_id=$1`, peerID)
	err := row.Scan(
		&peer.PeerID, &peer.NetworkID, &peer.NodeID, &peer.PublicKey, &virtualIPs, &allowedIPs,
		&peer.SupportsLANDirect, &peer.SupportsIPv6Direct, &peer.SupportsDirectUDP, &peer.SupportsRelayUDP, &peer.SupportsDerpTCPTLS443,
		&peer.PreferIPv6, &peer.PreferLAN, &peer.AllowEndpointRoaming, &peer.AllowFastReselection, &peer.AllowRelayTicketRenewal,
		&peer.KeepaliveIntervalSecs, &endpoints, &relayTicket, &peer.RecentPathDowngrades, &peer.RecentPathUpgrades,
		&peer.RequireMtuRefresh, &peer.EndpointChanged, &peer.UpdatedAt, &activePath,
	)
	if err == sql.ErrNoRows {
		return model.PeerRecord{}, false
	}
	if err != nil {
		return model.PeerRecord{}, false
	}
	_ = json.Unmarshal(virtualIPs, &peer.VirtualIPs)
	_ = json.Unmarshal(allowedIPs, &peer.AllowedIPs)
	_ = json.Unmarshal(endpoints, &peer.Endpoints)
	_ = json.Unmarshal(relayTicket, &peer.RelayTicket)
	if activePath.Valid {
		peer.ActivePath = model.PathKind(activePath.String)
	}
	peer.Probes = s.pathProbes(peerID)
	peer.DerpHealth = s.derpHealth(peerID)
	return clonePeer(peer), true
}

func (s *PostgresStore) ListPeersByNetwork(networkID string) []model.PeerRecord {
	rows, err := s.db.QueryContext(context.Background(), `SELECT peer_id FROM wire_peers WHERE network_id=$1 ORDER BY peer_id`, networkID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var peers []model.PeerRecord
	for rows.Next() {
		var peerID string
		if err := rows.Scan(&peerID); err != nil {
			return nil
		}
		if peer, ok := s.GetPeer(peerID); ok {
			peers = append(peers, peer)
		}
	}
	return peers
}

func (s *PostgresStore) changedPeer(peerID string, res sql.Result) (model.PeerRecord, error) {
	changed, err := res.RowsAffected()
	if err != nil {
		return model.PeerRecord{}, err
	}
	if changed == 0 {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	peer, ok := s.GetPeer(peerID)
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	return peer, nil
}

func (s *PostgresStore) peerExists(peerID string) bool {
	var exists bool
	err := s.db.QueryRowContext(context.Background(), `SELECT EXISTS (SELECT 1 FROM wire_peers WHERE peer_id=$1)`, peerID).Scan(&exists)
	return err == nil && exists
}

func (s *PostgresStore) pathProbes(peerID string) []model.PathProbe {
	rows, err := s.db.QueryContext(context.Background(), `SELECT path, reachable, rtt_ms, loss_ppm, jitter_ms, consecutive_fails, mtu, observed_at FROM wire_path_probes WHERE peer_id=$1 ORDER BY path`, peerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var probes []model.PathProbe
	for rows.Next() {
		var probe model.PathProbe
		var path string
		if err := rows.Scan(&path, &probe.Reachable, &probe.RTTMs, &probe.LossPPM, &probe.JitterMs, &probe.ConsecutiveFails, &probe.MTU, &probe.ObservedAt); err != nil {
			return nil
		}
		probe.Path = model.PathKind(path)
		probes = append(probes, probe)
	}
	return probes
}

func (s *PostgresStore) derpHealth(peerID string) []model.DerpHealthSample {
	rows, err := s.db.QueryContext(context.Background(), `SELECT region_id, node_id, reachable, rtt_ms, observed_at FROM wire_derp_health WHERE peer_id=$1 ORDER BY region_id, node_id`, peerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var samples []model.DerpHealthSample
	for rows.Next() {
		var sample model.DerpHealthSample
		if err := rows.Scan(&sample.RegionID, &sample.NodeID, &sample.Reachable, &sample.RTTMs, &sample.ObservedAt); err != nil {
			return nil
		}
		samples = append(samples, sample)
	}
	return samples
}

func jsonValue(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("null")
	}
	return payload
}
