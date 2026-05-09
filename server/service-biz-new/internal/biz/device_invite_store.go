package biz

import (
	"bufio"
	"context"
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
const authCallbackRedisPrefix = "slan:auth_callback:"

type deviceInviteStore interface {
	Save(invite DeviceInvite, ttl time.Duration) error
	Consume(inviteCode string) (DeviceInvite, error)
}

type authCallbackStore interface {
	SaveCallback(callback DeviceLoginCallback, ttl time.Duration, onlyIfAbsent bool) error
	LoadCallback(callbackID string) (DeviceLoginCallback, error)
}

type memoryDeviceInviteStore struct {
	mu        sync.Mutex
	invites   map[string]DeviceInvite
	callbacks map[string]DeviceLoginCallback
}

func newMemoryDeviceInviteStore() *memoryDeviceInviteStore {
	return &memoryDeviceInviteStore{invites: make(map[string]DeviceInvite), callbacks: make(map[string]DeviceLoginCallback)}
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

func (s *memoryDeviceInviteStore) SaveCallback(callback DeviceLoginCallback, ttl time.Duration, onlyIfAbsent bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if onlyIfAbsent {
		if _, ok := s.callbacks[callback.CallbackID]; ok {
			return errConflict
		}
	}
	if callback.ExpiresAt <= 0 {
		callback.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	s.callbacks[callback.CallbackID] = callback
	return nil
}

func (s *memoryDeviceInviteStore) LoadCallback(callbackID string) (DeviceLoginCallback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	callback, ok := s.callbacks[callbackID]
	if !ok || callback.ExpiresAt < time.Now().Unix() {
		return DeviceLoginCallback{}, errNotFound
	}
	return callback, nil
}

type redisDeviceInviteStore struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
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

func (s *redisDeviceInviteStore) SaveCallback(callback DeviceLoginCallback, ttl time.Duration, onlyIfAbsent bool) error {
	payload, err := json.Marshal(callback)
	if err != nil {
		return err
	}
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 600
	}
	args := []string{"SET", authCallbackRedisPrefix + callback.CallbackID, string(payload), "EX", strconv.Itoa(ttlSeconds)}
	if onlyIfAbsent {
		args = append(args, "NX")
	}
	resp, err := s.command(context.Background(), args...)
	if err != nil {
		return err
	}
	if resp.simple == "OK" {
		return nil
	}
	return errConflict
}

func (s *redisDeviceInviteStore) LoadCallback(callbackID string) (DeviceLoginCallback, error) {
	resp, err := s.command(context.Background(), "GET", authCallbackRedisPrefix+callbackID)
	if err != nil {
		return DeviceLoginCallback{}, err
	}
	if resp.nil {
		return DeviceLoginCallback{}, errNotFound
	}
	var callback DeviceLoginCallback
	if err := json.Unmarshal([]byte(resp.bulk), &callback); err != nil {
		return DeviceLoginCallback{}, err
	}
	if callback.ExpiresAt < time.Now().Unix() {
		return DeviceLoginCallback{}, errNotFound
	}
	return callback, nil
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
