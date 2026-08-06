package app

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/repository"
)

func newDeviceRuntimeRepository() repository.DeviceRuntimeRepository {
	database, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("SLAN_BIZ_REDIS_DB")))
	address := strings.TrimSpace(os.Getenv("SLAN_BIZ_REDIS_ADDR"))
	if address == "" {
		address = "127.0.0.1:6379"
	}
	return repository.NewRedisDeviceRuntimeStore(repository.RedisDeviceRuntimeConfig{
		Address:  address,
		Password: os.Getenv("SLAN_BIZ_REDIS_PASSWORD"),
		DB:       database,
	})
}

func deviceRuntimeTTL() time.Duration {
	if value, ok := envDuration("SLAN_DEVICE_RUNTIME_TTL"); ok && value > 0 {
		return value
	}
	return 45 * time.Second
}
