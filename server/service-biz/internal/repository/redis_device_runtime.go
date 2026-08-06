package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/slan/service-biz/internal/model"
)

const deviceRuntimeKeyPrefix = "slan:device-runtime:v1:"

type RedisDeviceRuntimeConfig struct {
	Address  string
	Password string
	DB       int
}

type RedisDeviceRuntimeStore struct {
	client *redis.Client
}

func NewRedisDeviceRuntimeStore(config RedisDeviceRuntimeConfig) *RedisDeviceRuntimeStore {
	return &RedisDeviceRuntimeStore{client: redis.NewClient(&redis.Options{
		Addr:     strings.TrimSpace(config.Address),
		Password: config.Password,
		DB:       config.DB,
	})}
}

func (s *RedisDeviceRuntimeStore) GetDeviceRuntime(ctx context.Context, deviceID string) (model.DeviceRuntimeState, bool, error) {
	values, err := s.client.HGetAll(ctx, deviceRuntimeKey(deviceID)).Result()
	if err != nil {
		return model.DeviceRuntimeState{}, false, err
	}
	if len(values) == 0 {
		return model.DeviceRuntimeState{}, false, nil
	}
	return model.DeviceRuntimeState{
		DeviceID:           strings.TrimSpace(deviceID),
		ApplicationState:   values["applicationState"],
		Activated:          parseRedisBool(values["activated"]),
		NetworkEnabled:     parseRedisBool(values["networkEnabled"]),
		VirtualIP:          values["virtualIp"],
		LastSeenAt:         parseRedisInt64(values["lastSeenAt"]),
		LastHeartbeatAt:    parseRedisInt64(values["lastHeartbeatAt"]),
		LastRuntimeStateAt: parseRedisInt64(values["lastRuntimeStateAt"]),
	}, true, nil
}

func (s *RedisDeviceRuntimeStore) RefreshDeviceHeartbeat(ctx context.Context, state model.DeviceRuntimeState, ttl time.Duration) error {
	return s.refresh(ctx, state.DeviceID, map[string]any{
		"applicationState": state.ApplicationState,
		"activated":        state.Activated,
		"lastSeenAt":       state.LastSeenAt,
		"lastHeartbeatAt":  state.LastHeartbeatAt,
	}, ttl)
}

func (s *RedisDeviceRuntimeStore) RefreshDeviceNetworkState(ctx context.Context, state model.DeviceRuntimeState, ttl time.Duration) error {
	return s.refresh(ctx, state.DeviceID, map[string]any{
		"applicationState":   state.ApplicationState,
		"activated":          state.Activated,
		"networkEnabled":     state.NetworkEnabled,
		"virtualIp":          state.VirtualIP,
		"lastSeenAt":         state.LastSeenAt,
		"lastRuntimeStateAt": state.LastRuntimeStateAt,
	}, ttl)
}

func (s *RedisDeviceRuntimeStore) refresh(ctx context.Context, deviceID string, values map[string]any, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	key := deviceRuntimeKey(deviceID)
	pipe.HSet(ctx, key, values)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func deviceRuntimeKey(deviceID string) string {
	return deviceRuntimeKeyPrefix + strings.TrimSpace(deviceID)
}

func parseRedisBool(value string) bool {
	parsed, _ := strconv.ParseBool(value)
	return parsed
}

func parseRedisInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
