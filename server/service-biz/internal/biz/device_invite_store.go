package biz

import (
	"os"
	"strconv"
	"strings"
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

// Device invite stores are implemented by memory, PostgreSQL, and Redis backends in device_invite_*_store.go.
