package infra

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type HTTPConfig struct {
	// Address 是 HTTP API 服务的监听地址。
	Address string `yaml:"address"`
}

type WSConfig struct {
	// Path 是控制通道使用的 WebSocket 升级端点路径。
	Path string `yaml:"path"`
}

type RelayConfig struct {
	// Region 标识中继集群或部署区域。
	Region string `yaml:"region"`
	// UDPEndpoint 是返回给客户端的公网 UDP 中继端点。
	UDPEndpoint string `yaml:"udp_endpoint"`
	// TCPEndpoint 预留给 TCP 中继回退使用。
	TCPEndpoint string `yaml:"tcp_endpoint"`
}

type BootstrapConfig struct {
	// STUNServers 是 bootstrap 返回给客户端的默认 STUN 列表。
	STUNServers []string `yaml:"stun_servers"`
}

type PostgresConfig struct {
	// Host 是 PostgreSQL 主机地址。
	Host string `yaml:"host"`
	// Port 是 PostgreSQL 端口。
	Port int `yaml:"port"`
	// Database 是数据库名。
	Database string `yaml:"database"`
	// Username 是登录用户名。
	Username string `yaml:"username"`
	// Password 是登录密码。
	Password string `yaml:"password"`
	// SSLMode 是 pg 连接安全模式。
	SSLMode string `yaml:"ssl_mode"`
	// MaxOpenConns 是最大连接数。
	MaxOpenConns int `yaml:"max_open_conns"`
	// MinIdleConns 是最小空闲连接数。
	MinIdleConns int `yaml:"min_idle_conns"`
	// ConnMaxLifetimeSeconds 是连接最大存活时间。
	ConnMaxLifetimeSeconds int `yaml:"conn_max_lifetime_seconds"`
	// ConnMaxIdleTimeSeconds 是连接最大空闲时间。
	ConnMaxIdleTimeSeconds int `yaml:"conn_max_idle_time_seconds"`
}

type RedisConfig struct {
	// Host 是 Redis 主机地址。
	Host string `yaml:"host"`
	// Port 是 Redis 端口。
	Port int `yaml:"port"`
	// Password 是 Redis 密码。
	Password string `yaml:"password"`
	// Database 是 Redis 库编号。
	Database int `yaml:"database"`
	// PoolSize 是连接池大小。
	PoolSize int `yaml:"pool_size"`
	// MinIdleConns 是最小空闲连接数。
	MinIdleConns int `yaml:"min_idle_conns"`
}

// Config 是 server-biz 的运行时配置。
type Config struct {
	HTTP      HTTPConfig      `yaml:"http"`
	WS        WSConfig        `yaml:"ws"`
	Relay     RelayConfig     `yaml:"relay"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
	Postgres  PostgresConfig  `yaml:"postgres"`
	Redis     RedisConfig     `yaml:"redis"`
}

// DefaultConfig 返回适用于本地开发环境的控制面默认配置。
func DefaultConfig() Config {
	cfg := Config{}
	cfg.HTTP.Address = ":8080"
	cfg.WS.Path = "/control/ws"
	cfg.Relay.Region = "local"
	cfg.Relay.UDPEndpoint = "127.0.0.1:9000"
	cfg.Bootstrap.STUNServers = []string{"stun:stun.l.google.com:19302"}
	cfg.Postgres.Host = "127.0.0.1"
	cfg.Postgres.Port = 5432
	cfg.Postgres.Database = "slan"
	cfg.Postgres.Username = "postgres"
	cfg.Postgres.Password = "123456"
	cfg.Postgres.SSLMode = "disable"
	cfg.Postgres.MaxOpenConns = 10
	cfg.Postgres.MinIdleConns = 2
	cfg.Postgres.ConnMaxLifetimeSeconds = 1800
	cfg.Postgres.ConnMaxIdleTimeSeconds = 300
	cfg.Redis.Host = "127.0.0.1"
	cfg.Redis.Port = 6379
	cfg.Redis.Password = ""
	cfg.Redis.Database = 1
	cfg.Redis.PoolSize = 10
	cfg.Redis.MinIdleConns = 2
	return cfg
}

// LoadConfig 从 YAML 文件加载配置；空路径时返回默认配置。
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(payload, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = DefaultConfig().HTTP.Address
	}
	if cfg.WS.Path == "" {
		cfg.WS.Path = DefaultConfig().WS.Path
	}
	if len(cfg.Bootstrap.STUNServers) == 0 {
		cfg.Bootstrap.STUNServers = DefaultConfig().Bootstrap.STUNServers
	}
	if cfg.Postgres.Host == "" {
		cfg.Postgres = DefaultConfig().Postgres
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = DefaultConfig().Postgres.Port
	}
	if cfg.Postgres.Database == "" {
		cfg.Postgres.Database = DefaultConfig().Postgres.Database
	}
	if cfg.Postgres.Username == "" {
		cfg.Postgres.Username = DefaultConfig().Postgres.Username
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = DefaultConfig().Postgres.SSLMode
	}
	if cfg.Postgres.MaxOpenConns == 0 {
		cfg.Postgres.MaxOpenConns = DefaultConfig().Postgres.MaxOpenConns
	}
	if cfg.Postgres.MinIdleConns == 0 {
		cfg.Postgres.MinIdleConns = DefaultConfig().Postgres.MinIdleConns
	}
	if cfg.Postgres.ConnMaxLifetimeSeconds == 0 {
		cfg.Postgres.ConnMaxLifetimeSeconds = DefaultConfig().Postgres.ConnMaxLifetimeSeconds
	}
	if cfg.Postgres.ConnMaxIdleTimeSeconds == 0 {
		cfg.Postgres.ConnMaxIdleTimeSeconds = DefaultConfig().Postgres.ConnMaxIdleTimeSeconds
	}
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = DefaultConfig().Redis.Host
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = DefaultConfig().Redis.Port
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = DefaultConfig().Redis.PoolSize
	}
	if cfg.Redis.MinIdleConns == 0 {
		cfg.Redis.MinIdleConns = DefaultConfig().Redis.MinIdleConns
	}
	return cfg, nil
}

// DSN 返回 PostgreSQL DSN。
func (c Config) DSN() string {
	return c.Postgres.DSN()
}

// DSN 返回 PostgreSQL 连接串。
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.Username,
		p.Password,
		p.Host,
		p.Port,
		p.Database,
		p.SSLMode,
	)
}

// Address 返回 Redis 地址。
func (r RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}
