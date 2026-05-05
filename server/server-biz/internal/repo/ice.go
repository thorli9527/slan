package repo

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IceServer struct {
	ServerID  string `gorm:"column:server_id;primaryKey"`
	Name      string `gorm:"column:name;not null"`
	Provider  string `gorm:"column:provider;not null"`
	Region    string `gorm:"column:region;index;not null"`
	Country   string `gorm:"column:country;not null;default:'CN'"`
	PublicIP  string `gorm:"column:public_ip;not null"`
	UDPAddr   string `gorm:"column:udp_addr;not null"`
	STUNPort  int    `gorm:"column:stun_port;not null;default:3478"`
	Priority  int    `gorm:"column:priority;index;not null;default:100"`
	Weight    int    `gorm:"column:weight;not null;default:100"`
	Status    string `gorm:"column:status;index;not null;default:'enabled'"`
	Features  string `gorm:"column:features;not null;default:''"`
	Remark    string `gorm:"column:remark;not null;default:''"`
	CreatedAt int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt int64  `gorm:"column:updated_at;index;not null;default:0"`
}

func (IceServer) TableName() string { return "ice_servers" }

func (m IceServer) ToDTO() dto.IceServer {
	return dto.IceServer{
		ServerID:  m.ServerID,
		Name:      m.Name,
		Provider:  m.Provider,
		Region:    m.Region,
		Country:   m.Country,
		PublicIP:  m.PublicIP,
		UDPAddr:   m.UDPAddr,
		STUNPort:  m.STUNPort,
		Priority:  m.Priority,
		Weight:    m.Weight,
		Status:    m.Status,
		Features:  decodeCSVList(m.Features),
		Remark:    m.Remark,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

type PeerCandidate struct {
	PeerID         string `gorm:"column:peer_id;primaryKey"`
	CandidateID    string `gorm:"column:candidate_id;primaryKey"`
	NetworkID      string `gorm:"column:network_id;index;not null;default:''"`
	CandidateType  string `gorm:"column:candidate_type;not null"`
	Addr           string `gorm:"column:addr;not null"`
	SourceServerID string `gorm:"column:source_server_id;index;not null;default:''"`
	Priority       int    `gorm:"column:priority;not null;default:100"`
	UDPAvailable   bool   `gorm:"column:udp_available;not null;default:true"`
	NATLevel       string `gorm:"column:nat_level;not null;default:''"`
	MappingStable  bool   `gorm:"column:mapping_stable;not null;default:false"`
	LastSeenAt     int64  `gorm:"column:last_seen_at;index;not null"`
	CreatedAt      int64  `gorm:"column:created_at;not null"`
}

func (PeerCandidate) TableName() string { return "peer_candidates" }

func (m PeerCandidate) ToDTO() dto.Candidate {
	return dto.Candidate{
		CandidateID:    m.CandidateID,
		CandidateType:  m.CandidateType,
		Addr:           m.Addr,
		SourceServerID: m.SourceServerID,
		Priority:       m.Priority,
	}
}

type PunchSession struct {
	SessionID string `gorm:"column:session_id;primaryKey"`
	NetworkID string `gorm:"column:network_id;index;not null;default:''"`
	PeerA     string `gorm:"column:peer_a;index;not null"`
	PeerB     string `gorm:"column:peer_b;index;not null"`
	Status    string `gorm:"column:status;index;not null;default:'created'"`
	CreatedAt int64  `gorm:"column:created_at;index;not null"`
	UpdatedAt int64  `gorm:"column:updated_at;index;not null"`
}

func (PunchSession) TableName() string { return "punch_sessions" }

type PunchResult struct {
	ResultID          string  `gorm:"column:result_id;primaryKey"`
	SessionID         string  `gorm:"column:session_id;index;not null"`
	ReporterPeerID    string  `gorm:"column:reporter_peer_id;index;not null"`
	RemotePeerID      string  `gorm:"column:remote_peer_id;index;not null"`
	Success           bool    `gorm:"column:success;index;not null"`
	PathKind          string  `gorm:"column:path_kind;not null;default:''"`
	LocalCandidateID  string  `gorm:"column:local_candidate_id;not null;default:''"`
	RemoteCandidateID string  `gorm:"column:remote_candidate_id;not null;default:''"`
	LocalSrflxAddr    string  `gorm:"column:local_srflx_addr;not null;default:''"`
	RemoteAddrSeen    string  `gorm:"column:remote_addr_seen;not null;default:''"`
	RTTMS             int     `gorm:"column:rtt_ms;not null;default:0"`
	LossRate          float64 `gorm:"column:loss_rate;not null;default:0"`
	JitterMS          int     `gorm:"column:jitter_ms;not null;default:0"`
	FailureReason     string  `gorm:"column:failure_reason;not null;default:''"`
	TriedPairs        int     `gorm:"column:tried_pairs;not null;default:0"`
	TimeoutMS         int     `gorm:"column:timeout_ms;not null;default:0"`
	FallbackPath      string  `gorm:"column:fallback_path;not null;default:''"`
	FallbackRelayID   string  `gorm:"column:fallback_relay_id;not null;default:''"`
	ReportedAt        int64   `gorm:"column:reported_at;index;not null"`
}

func (PunchResult) TableName() string { return "punch_results" }

type IceServerStat struct {
	ServerID        string  `gorm:"column:server_id;primaryKey"`
	Region          string  `gorm:"column:region;index;not null;default:''"`
	ProbeCount      int64   `gorm:"column:probe_count;not null;default:0"`
	CandidateCount  int64   `gorm:"column:candidate_count;not null;default:0"`
	P2PSuccessCount int64   `gorm:"column:p2p_success_count;not null;default:0"`
	P2PFailureCount int64   `gorm:"column:p2p_failure_count;not null;default:0"`
	AvgProbeRTTMS   int     `gorm:"column:avg_probe_rtt_ms;not null;default:0"`
	SuccessRate     float64 `gorm:"column:success_rate;not null;default:0"`
	UpdatedAt       int64   `gorm:"column:updated_at;index;not null"`
}

func (IceServerStat) TableName() string { return "ice_server_stats" }

func (m IceServerStat) ToDTO() dto.IceServerStats {
	return dto.IceServerStats{
		ServerID:        m.ServerID,
		Region:          m.Region,
		ProbeCount:      m.ProbeCount,
		CandidateCount:  m.CandidateCount,
		P2PSuccessCount: m.P2PSuccessCount,
		P2PFailureCount: m.P2PFailureCount,
		AvgProbeRTTMS:   m.AvgProbeRTTMS,
		SuccessRate:     m.SuccessRate,
		UpdatedAt:       m.UpdatedAt,
	}
}

func (r *PostgresRepository) ListIceServers(ctx context.Context) ([]IceServer, error) {
	var out []IceServer
	err := r.db.WithContext(ctx).Order("priority asc, weight desc, server_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) UpsertIceServer(ctx context.Context, record IceServer) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "server_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "provider", "region", "country", "public_ip", "udp_addr", "stun_port",
			"priority", "weight", "status", "features", "remark", "updated_at",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) GetIceServer(ctx context.Context, serverID string) (IceServer, error) {
	var record IceServer
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) UpdateIceServerStatus(ctx context.Context, serverID, status string, updatedAt int64) error {
	return r.db.WithContext(ctx).Model(&IceServer{}).Where("server_id = ?", serverID).Updates(map[string]any{
		"status":     status,
		"updated_at": updatedAt,
	}).Error
}

func (r *PostgresRepository) UpsertPeerCandidates(ctx context.Context, records []PeerCandidate) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "peer_id"}, {Name: "candidate_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"network_id", "candidate_type", "addr", "source_server_id", "priority",
			"udp_available", "nat_level", "mapping_stable", "last_seen_at",
		}),
	}).Create(&records).Error
}

func (r *PostgresRepository) ListFreshPeerCandidates(ctx context.Context, networkID, peerID string, cutoff int64) ([]PeerCandidate, error) {
	var out []PeerCandidate
	q := r.db.WithContext(ctx).Where("peer_id = ? AND last_seen_at >= ?", peerID, cutoff)
	if networkID != "" {
		q = q.Where("network_id = ? OR network_id = ''", networkID)
	}
	err := q.Order("priority asc, last_seen_at desc, candidate_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetPeerCandidate(ctx context.Context, peerID, candidateID string) (PeerCandidate, error) {
	var record PeerCandidate
	err := r.db.WithContext(ctx).Where("peer_id = ? AND candidate_id = ?", peerID, candidateID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) CreatePunchSession(ctx context.Context, record PunchSession) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) GetPunchSession(ctx context.Context, sessionID string) (PunchSession, error) {
	var record PunchSession
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) InsertPunchResult(ctx context.Context, result PunchResult) error {
	return r.db.WithContext(ctx).Create(&result).Error
}

func (r *PostgresRepository) UpdatePunchSessionStatus(ctx context.Context, sessionID, status string, updatedAt int64) error {
	return r.db.WithContext(ctx).Model(&PunchSession{}).Where("session_id = ?", sessionID).Updates(map[string]any{
		"status":     status,
		"updated_at": updatedAt,
	}).Error
}

func (r *PostgresRepository) ListIceServerStats(ctx context.Context) ([]IceServerStat, error) {
	var out []IceServerStat
	err := r.db.WithContext(ctx).Order("server_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) IncrementIceCandidateStats(ctx context.Context, serverIDs []string) error {
	now := time.Now().Unix()
	for _, serverID := range serverIDs {
		if serverID == "" {
			continue
		}
		server, _ := r.GetIceServer(ctx, serverID)
		stat := IceServerStat{ServerID: serverID, Region: server.Region, UpdatedAt: now}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "server_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"candidate_count": gorm.Expr("ice_server_stats.candidate_count + 1"),
				"region":          server.Region,
				"updated_at":      now,
			}),
		}).Create(&stat).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) IncrementIcePunchStats(ctx context.Context, serverIDs []string, success bool) error {
	now := time.Now().Unix()
	column := "p2p_failure_count"
	if success {
		column = "p2p_success_count"
	}
	for _, serverID := range serverIDs {
		if serverID == "" {
			continue
		}
		server, _ := r.GetIceServer(ctx, serverID)
		assignments := map[string]any{
			column:       gorm.Expr("ice_server_stats." + column + " + 1"),
			"region":     server.Region,
			"updated_at": now,
		}
		if success {
			assignments["success_rate"] = gorm.Expr(
				"(ice_server_stats.p2p_success_count + 1) * 1.0 / NULLIF(ice_server_stats.p2p_success_count + ice_server_stats.p2p_failure_count + 1, 0)",
			)
		} else {
			assignments["success_rate"] = gorm.Expr(
				"ice_server_stats.p2p_success_count * 1.0 / NULLIF(ice_server_stats.p2p_success_count + ice_server_stats.p2p_failure_count + 1, 0)",
			)
		}
		stat := IceServerStat{ServerID: serverID, Region: server.Region, UpdatedAt: now}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}},
			DoUpdates: clause.Assignments(assignments),
		}).Create(&stat).Error; err != nil {
			return err
		}
	}
	return nil
}
