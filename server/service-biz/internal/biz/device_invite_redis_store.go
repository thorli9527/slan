package biz

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strconv"
	"time"
)

type redisDeviceInviteStore struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
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
