package biz

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const deviceInviteRedisPrefix = "slan:device_invite:"
const deviceBootstrapKeyRedisPrefix = "slan:device_bootstrap:"

type deviceInviteStore interface {
	Save(invite DeviceInvite, ttl time.Duration) error
	Consume(inviteCode string) (DeviceInvite, error)
}

type deviceBootstrapKeyStore interface {
	SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error
	ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error)
}

type memoryDeviceInviteStore struct {
	mu            sync.Mutex
	invites       map[string]DeviceInvite
	bootstrapKeys map[string]DeviceBootstrapKey
}

func newMemoryDeviceInviteStore() *memoryDeviceInviteStore {
	return &memoryDeviceInviteStore{
		invites:       make(map[string]DeviceInvite),
		bootstrapKeys: make(map[string]DeviceBootstrapKey),
	}
}

func (s *memoryDeviceInviteStore) Save(invite DeviceInvite, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	invite.ExpiresAt = time.Now().Add(ttl).Unix()
	s.invites[invite.InviteCode] = invite
	return nil
}

func (s *memoryDeviceInviteStore) Consume(inviteCode string) (DeviceInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invite, ok := s.invites[inviteCode]
	if !ok || invite.Status != "pending" || invite.ExpiresAt < time.Now().Unix() {
		return DeviceInvite{}, errNotFound
	}
	delete(s.invites, inviteCode)
	return invite, nil
}

func (s *memoryDeviceInviteStore) SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key.ExpiresAt <= 0 {
		key.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	if _, ok := s.bootstrapKeys[key.KeyHash]; ok {
		return errConflict
	}
	s.bootstrapKeys[key.KeyHash] = key
	return nil
}

func (s *memoryDeviceInviteStore) ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.bootstrapKeys[keyHash]
	if !ok || key.Status != "unused" || key.ExpiresAt < time.Now().Unix() || key.RevokedAt > 0 {
		return DeviceBootstrapKey{}, errNotFound
	}
	delete(s.bootstrapKeys, keyHash)
	return key, nil
}

type redisDeviceInviteStore struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
}

type postgresDeviceInviteStore struct {
	db *sql.DB
}

func newDeviceInviteStoreFromEnv() deviceInviteStore {
	addr := strings.TrimSpace(os.Getenv("SLAN_BIZ_REDIS_ADDR"))
	if addr == "" {
		addr = strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	}
	if addr == "" {
		return newMemoryDeviceInviteStore()
	}
	db, _ := strconv.Atoi(os.Getenv("SLAN_BIZ_REDIS_DB"))
	return &redisDeviceInviteStore{
		addr:     addr,
		password: os.Getenv("SLAN_BIZ_REDIS_PASSWORD"),
		db:       db,
		timeout:  2 * time.Second,
	}
}

func (s *postgresDeviceInviteStore) Save(invite DeviceInvite, ttl time.Duration) error {
	if s == nil || s.db == nil {
		return errNotFound
	}
	if invite.ExpiresAt <= 0 {
		invite.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	_, err := s.db.ExecContext(context.Background(), `insert into device_invites(id,inviter_user_id,invite_code,status,created_at,expires_at,accepted_device_id,accepted_user_id,accepted_at)
		values($1,$2,$3,$4,to_timestamp($5),to_timestamp($6),nullif($7,''),nullif($8,''),to_timestamp(nullif($9,0)))`,
		invite.InviteID, invite.InviterUserID, invite.InviteCode, invite.Status, invite.CreatedAt, invite.ExpiresAt, invite.AcceptedDeviceID, invite.AcceptedUserID, invite.AcceptedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *postgresDeviceInviteStore) Consume(inviteCode string) (DeviceInvite, error) {
	if s == nil || s.db == nil {
		return DeviceInvite{}, errNotFound
	}
	var invite DeviceInvite
	err := s.db.QueryRowContext(context.Background(), `select id,inviter_user_id,invite_code,status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(accepted_device_id,''),coalesce(accepted_user_id,''),coalesce(extract(epoch from accepted_at)::bigint,0)
		from device_invites where invite_code=$1 and status='pending' and expires_at > now()`, strings.TrimSpace(inviteCode)).
		Scan(&invite.InviteID, &invite.InviterUserID, &invite.InviteCode, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt, &invite.AcceptedDeviceID, &invite.AcceptedUserID, &invite.AcceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceInvite{}, errNotFound
	}
	if err != nil {
		return DeviceInvite{}, err
	}
	return invite, nil
}

func (s *postgresDeviceInviteStore) SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error {
	if s == nil || s.db == nil {
		return errNotFound
	}
	if key.ExpiresAt <= 0 {
		key.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	_, err := s.db.ExecContext(context.Background(), `insert into device_bootstrap_keys(id,key_hash,created_by_user_id,network_id,device_alias,status,created_at,expires_at,used_at,used_by_device_id,revoked_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp($7),to_timestamp($8),to_timestamp(nullif($9,0)),nullif($10,''),to_timestamp(nullif($11,0)))`,
		key.KeyID, key.KeyHash, key.CreatedByUserID, key.NetworkID, key.DeviceAlias, key.Status, key.CreatedAt, key.ExpiresAt, key.UsedAt, key.UsedByDeviceID, key.RevokedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *postgresDeviceInviteStore) ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error) {
	if s == nil || s.db == nil {
		return DeviceBootstrapKey{}, errNotFound
	}
	var key DeviceBootstrapKey
	err := s.db.QueryRowContext(context.Background(), `select id,key_hash,created_by_user_id,network_id,coalesce(device_alias,''),status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from used_at)::bigint,0),coalesce(used_by_device_id,''),coalesce(extract(epoch from revoked_at)::bigint,0)
		from device_bootstrap_keys where key_hash=$1 and status='unused' and expires_at > now() and revoked_at is null`, strings.TrimSpace(keyHash)).
		Scan(&key.KeyID, &key.KeyHash, &key.CreatedByUserID, &key.NetworkID, &key.DeviceAlias, &key.Status, &key.CreatedAt, &key.ExpiresAt, &key.UsedAt, &key.UsedByDeviceID, &key.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceBootstrapKey{}, errNotFound
	}
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	return key, nil
}

func (s *redisDeviceInviteStore) Save(invite DeviceInvite, ttl time.Duration) error {
	payload, err := json.Marshal(invite)
	if err != nil {
		return err
	}
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 1800
	}
	resp, err := s.command(context.Background(), "SET", deviceInviteRedisPrefix+invite.InviteCode, string(payload), "EX", strconv.Itoa(ttlSeconds), "NX")
	if err != nil {
		return err
	}
	if resp.simple == "OK" {
		return nil
	}
	return errConflict
}

func (s *redisDeviceInviteStore) Consume(inviteCode string) (DeviceInvite, error) {
	resp, err := s.command(context.Background(), "GETDEL", deviceInviteRedisPrefix+inviteCode)
	if err != nil {
		return DeviceInvite{}, err
	}
	if resp.nil {
		return DeviceInvite{}, errNotFound
	}
	var invite DeviceInvite
	if err := json.Unmarshal([]byte(resp.bulk), &invite); err != nil {
		return DeviceInvite{}, err
	}
	if invite.Status != "pending" || invite.ExpiresAt < time.Now().Unix() {
		return DeviceInvite{}, errNotFound
	}
	return invite, nil
}

func (s *redisDeviceInviteStore) SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error {
	payload, err := json.Marshal(key)
	if err != nil {
		return err
	}
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 1800
	}
	resp, err := s.command(context.Background(), "SET", deviceBootstrapKeyRedisPrefix+key.KeyHash, string(payload), "EX", strconv.Itoa(ttlSeconds), "NX")
	if err != nil {
		return err
	}
	if resp.simple == "OK" {
		return nil
	}
	return errConflict
}

func (s *redisDeviceInviteStore) ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error) {
	resp, err := s.command(context.Background(), "GETDEL", deviceBootstrapKeyRedisPrefix+keyHash)
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	if resp.nil {
		return DeviceBootstrapKey{}, errNotFound
	}
	var key DeviceBootstrapKey
	if err := json.Unmarshal([]byte(resp.bulk), &key); err != nil {
		return DeviceBootstrapKey{}, err
	}
	if key.Status != "unused" || key.ExpiresAt < time.Now().Unix() || key.RevokedAt > 0 {
		return DeviceBootstrapKey{}, errNotFound
	}
	return key, nil
}

type redisResp struct {
	simple string
	bulk   string
	nil    bool
}

func (s *redisDeviceInviteStore) command(ctx context.Context, args ...string) (redisResp, error) {
	var zero redisResp
	dialer := net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return zero, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
	reader := bufio.NewReader(conn)
	if s.password != "" {
		if _, err := conn.Write(redisCommand("AUTH", s.password)); err != nil {
			return zero, err
		}
		if _, err := readRedisResp(reader); err != nil {
			return zero, err
		}
	}
	if s.db > 0 {
		if _, err := conn.Write(redisCommand("SELECT", strconv.Itoa(s.db))); err != nil {
			return zero, err
		}
		if _, err := readRedisResp(reader); err != nil {
			return zero, err
		}
	}
	if _, err := conn.Write(redisCommand(args...)); err != nil {
		return zero, err
	}
	return readRedisResp(reader)
}

func redisCommand(args ...string) []byte {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, arg := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteString("\r\n")
		b.WriteString(arg)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func readRedisResp(reader *bufio.Reader) (redisResp, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return redisResp{}, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return redisResp{}, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return redisResp{simple: line}, nil
	case '-':
		return redisResp{}, errors.New(line)
	case ':':
		return redisResp{simple: line}, nil
	case '$':
		size, err := strconv.Atoi(line)
		if err != nil {
			return redisResp{}, err
		}
		if size < 0 {
			return redisResp{nil: true}, nil
		}
		buf := make([]byte, size+2)
		if _, err := reader.Read(buf); err != nil {
			return redisResp{}, err
		}
		return redisResp{bulk: string(buf[:size])}, nil
	default:
		return redisResp{}, fmt.Errorf("unsupported redis response %q", prefix)
	}
}
