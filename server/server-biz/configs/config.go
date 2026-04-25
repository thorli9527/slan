package configs

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// HTTPConfig 描述 server-biz 两套 HTTP listener 的监听地址。
type HTTPConfig struct {
	// Address 是对外客户 HTTP API 服务的监听地址。
	Address string `yaml:"address"`
	// OpsAddress 是运营管理 HTTP API 服务的监听地址。
	OpsAddress string `yaml:"ops_address"`
	// PublicHost 是客户端生成控制面 WSURL 时使用的可访问主机名或 host:port。
	PublicHost string `yaml:"public_host"`
	// PublicScheme 是客户端访问控制面的外部协议，例如 http 或 https。
	PublicScheme string `yaml:"public_scheme"`
}

// WSConfig 描述控制面 WebSocket 相关配置。
type WSConfig struct {
	// Path 是控制通道使用的 WebSocket 升级端点路径。
	Path string `yaml:"path"`
}

// RelayNodeConfig 描述配置文件中的一个 relay 节点。
type RelayNodeConfig struct {
	// NodeID 是 relay 节点唯一标识。
	NodeID string `yaml:"node_id"`
	// Transport 是节点支持的传输类型，例如 udp / tcp / quic。
	Transport string `yaml:"transport"`
	// Address 是公网接入地址。
	Address string `yaml:"address"`
	// Priority 是控制面建议优先级，数值越小越优先。
	Priority int `yaml:"priority"`
	// Tags 是可选标签。
	Tags []string `yaml:"tags"`
}

// RelayClusterConfig 描述一个城市下的 relay 集群。
type RelayClusterConfig struct {
	// ClusterID 是集群唯一标识。
	ClusterID string `yaml:"cluster_id"`
	// ClusterName 是集群展示名称。
	ClusterName string `yaml:"cluster_name"`
	// Nodes 是集群内节点列表。
	Nodes []RelayNodeConfig `yaml:"nodes"`
}

// RelayCityConfig 描述一个国家下的 relay 城市分组。
type RelayCityConfig struct {
	// CityCode 是城市编码，例如 bj / sh / gz。
	CityCode string `yaml:"city_code"`
	// CityName 是城市名称。
	CityName string `yaml:"city_name"`
	// Clusters 是城市下的 relay 集群。
	Clusters []RelayClusterConfig `yaml:"clusters"`
}

// RelayCountryConfig 描述顶层国家维度的 relay 拓扑配置。
type RelayCountryConfig struct {
	// CountryCode 是国家编码，例如 CN / SG。
	CountryCode string `yaml:"country_code"`
	// CountryName 是国家名称。
	CountryName string `yaml:"country_name"`
	// Cities 是国家下的 relay 城市列表。
	Cities []RelayCityConfig `yaml:"cities"`
}

// RelayConfig 描述 relay/DERP 相关的控制面配置。
type RelayConfig struct {
	// DefaultClusterID 是控制面默认选择的集群。
	DefaultClusterID string `yaml:"default_cluster_id"`
	// TicketSigningSecret 是 relay ticket 的 HMAC 签名密钥。
	TicketSigningSecret string `yaml:"ticket_signing_secret"`
	// Countries 是 relay 四层拓扑配置。
	Countries []RelayCountryConfig `yaml:"countries"`
}

// BootstrapConfig 描述客户端 bootstrap 阶段使用的默认配置。
type BootstrapConfig struct {
	// STUNServers 是 bootstrap 返回给客户端的默认 STUN 列表。
	STUNServers []string `yaml:"stun_servers"`
}

// OpsConfig 描述运营管理入口相关配置。
type OpsConfig struct {
	// AccessToken 是运营入口使用的静态访问令牌。
	AccessToken string `yaml:"access_token"`
	// DefaultAdmin 描述启动时自动灌入的默认管理员账号。
	DefaultAdmin OpsDefaultAdminConfig `yaml:"default_admin"`
}

// OpsDefaultAdminConfig 描述默认管理员 seed 配置。
type OpsDefaultAdminConfig struct {
	// Enabled 控制是否自动灌入默认管理员。
	Enabled bool `yaml:"enabled"`
	// Email 是默认管理员绑定的业务用户邮箱。
	Email string `yaml:"email"`
	// Password 是默认管理员初始密码。
	Password string `yaml:"password"`
	// LoginName 是默认管理员登录名。
	LoginName string `yaml:"login_name"`
	// DisplayName 是默认管理员显示名称。
	DisplayName string `yaml:"display_name"`
}

// PostgresConfig 描述 PostgreSQL 连接与连接池参数。
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

// RedisConfig 描述 Redis 连接与连接池参数。
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
	// DialTimeoutSeconds 是 Redis 建连超时时间。
	DialTimeoutSeconds int `yaml:"dial_timeout_seconds"`
	// ReadTimeoutSeconds 是 Redis 读超时时间。
	ReadTimeoutSeconds int `yaml:"read_timeout_seconds"`
	// WriteTimeoutSeconds 是 Redis 写超时时间。
	WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
}

// Config 是 server-biz 的运行时配置。
type Config struct {
	// HTTP 包含 public/ops 两套 HTTP listener 配置。
	HTTP HTTPConfig `yaml:"http"`
	// WS 包含控制面 WebSocket 端点配置。
	WS WSConfig `yaml:"ws"`
	// Relay 包含 relay/DERP 拓扑与票据签名配置。
	Relay RelayConfig `yaml:"relay"`
	// Bootstrap 包含客户端启动阶段的默认配置。
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
	// Ops 包含运营入口鉴权配置。
	Ops OpsConfig `yaml:"ops"`
	// Postgres 是 PostgreSQL 连接配置。
	Postgres PostgresConfig `yaml:"postgres"`
	// Redis 是 Redis 连接配置。
	Redis RedisConfig `yaml:"redis"`
}

// DefaultConfig 返回适用于本地开发环境的控制面默认配置。
func DefaultConfig() Config {
	cfg := Config{}
	cfg.HTTP.Address = ":8080"
	cfg.HTTP.OpsAddress = ":8081"
	cfg.HTTP.PublicHost = "127.0.0.1:8080"
	cfg.HTTP.PublicScheme = "http"
	cfg.WS.Path = "/control/ws"
	cfg.Relay.DefaultClusterID = "cn-local-a"
	cfg.Relay.TicketSigningSecret = "dev-relay-ticket-secret"
	cfg.Relay.Countries = []RelayCountryConfig{
		{
			CountryCode: "CN",
			CountryName: "China",
			Cities: []RelayCityConfig{
				{
					CityCode: "local",
					CityName: "Local",
					Clusters: []RelayClusterConfig{
						{
							ClusterID:   "cn-local-a",
							ClusterName: "CN Local A",
							Nodes: []RelayNodeConfig{
								{NodeID: "relay-cn-local-udp", Transport: "udp", Address: "127.0.0.1:9000", Priority: 10},
								{NodeID: "relay-cn-local-tcp", Transport: "tcp", Address: "127.0.0.1:9001", Priority: 20},
							},
						},
					},
				},
			},
		},
	}
	cfg.Bootstrap.STUNServers = []string{"stun:stun.l.google.com:19302"}
	cfg.Ops.AccessToken = "dev-ops-token"
	cfg.Ops.DefaultAdmin = OpsDefaultAdminConfig{
		Enabled:     true,
		Email:       "admin@local.slan",
		Password:    "change-me-admin-password",
		LoginName:   "admin",
		DisplayName: "Default Admin",
	}
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
	cfg.Redis.DialTimeoutSeconds = 5
	cfg.Redis.ReadTimeoutSeconds = 3
	cfg.Redis.WriteTimeoutSeconds = 3
	return cfg
}

// LoadConfig 从 YAML 文件加载配置；空路径时返回默认配置。
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		applyEnvOverrides(&cfg)
		if err := validateProductionConfig(cfg); err != nil {
			return Config{}, err
		}
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
	if cfg.HTTP.OpsAddress == "" {
		cfg.HTTP.OpsAddress = DefaultConfig().HTTP.OpsAddress
	}
	if cfg.HTTP.PublicHost == "" {
		cfg.HTTP.PublicHost = DefaultConfig().HTTP.PublicHost
	}
	if cfg.HTTP.PublicScheme == "" {
		cfg.HTTP.PublicScheme = DefaultConfig().HTTP.PublicScheme
	}
	if cfg.WS.Path == "" {
		cfg.WS.Path = DefaultConfig().WS.Path
	}
	if cfg.Relay.DefaultClusterID == "" || len(cfg.Relay.Countries) == 0 {
		cfg.Relay = DefaultConfig().Relay
	}
	if cfg.Relay.TicketSigningSecret == "" {
		cfg.Relay.TicketSigningSecret = DefaultConfig().Relay.TicketSigningSecret
	}
	if len(cfg.Bootstrap.STUNServers) == 0 {
		cfg.Bootstrap.STUNServers = DefaultConfig().Bootstrap.STUNServers
	}
	if cfg.Ops.AccessToken == "" {
		cfg.Ops.AccessToken = DefaultConfig().Ops.AccessToken
	}
	if cfg.Ops.DefaultAdmin.Email == "" {
		cfg.Ops.DefaultAdmin.Email = DefaultConfig().Ops.DefaultAdmin.Email
	}
	if cfg.Ops.DefaultAdmin.Password == "" {
		cfg.Ops.DefaultAdmin.Password = DefaultConfig().Ops.DefaultAdmin.Password
	}
	if cfg.Ops.DefaultAdmin.LoginName == "" {
		cfg.Ops.DefaultAdmin.LoginName = DefaultConfig().Ops.DefaultAdmin.LoginName
	}
	if cfg.Ops.DefaultAdmin.DisplayName == "" {
		cfg.Ops.DefaultAdmin.DisplayName = DefaultConfig().Ops.DefaultAdmin.DisplayName
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
	if cfg.Redis.DialTimeoutSeconds == 0 {
		cfg.Redis.DialTimeoutSeconds = DefaultConfig().Redis.DialTimeoutSeconds
	}
	if cfg.Redis.ReadTimeoutSeconds == 0 {
		cfg.Redis.ReadTimeoutSeconds = DefaultConfig().Redis.ReadTimeoutSeconds
	}
	if cfg.Redis.WriteTimeoutSeconds == 0 {
		cfg.Redis.WriteTimeoutSeconds = DefaultConfig().Redis.WriteTimeoutSeconds
	}
	applyEnvOverrides(&cfg)
	if err := validateProductionConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if value := os.Getenv("SLAN_HTTP_PUBLIC_HOST"); value != "" {
		cfg.HTTP.PublicHost = value
	}
	if value := os.Getenv("SLAN_HTTP_PUBLIC_SCHEME"); value != "" {
		cfg.HTTP.PublicScheme = value
	}
	if value := os.Getenv("SLAN_RELAY_TICKET_SIGNING_SECRET"); value != "" {
		cfg.Relay.TicketSigningSecret = value
	}
	if value := os.Getenv("SLAN_OPS_ACCESS_TOKEN"); value != "" {
		cfg.Ops.AccessToken = value
	}
	if value := os.Getenv("SLAN_OPS_DEFAULT_ADMIN_PASSWORD"); value != "" {
		cfg.Ops.DefaultAdmin.Password = value
	}
	if value := os.Getenv("SLAN_POSTGRES_PASSWORD"); value != "" {
		cfg.Postgres.Password = value
	}
	if value := os.Getenv("SLAN_REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}
}

func validateProductionConfig(cfg Config) error {
	if !isProductionEnv() {
		return nil
	}
	defaults := DefaultConfig()
	var problems []string
	if strings.ToLower(strings.TrimSpace(cfg.HTTP.PublicScheme)) != "https" {
		problems = append(problems, "http.public_scheme must be https")
	}
	if isLoopbackPublicHost(cfg.HTTP.PublicHost) {
		problems = append(problems, "http.public_host must not be a loopback host")
	}
	if weakSecret(cfg.Relay.TicketSigningSecret, defaults.Relay.TicketSigningSecret, "change-me") {
		problems = append(problems, "relay.ticket_signing_secret must be replaced")
	}
	if weakSecret(cfg.Ops.AccessToken, defaults.Ops.AccessToken, "change-me") {
		problems = append(problems, "ops.access_token must be replaced")
	}
	if cfg.Ops.DefaultAdmin.Enabled &&
		weakSecret(cfg.Ops.DefaultAdmin.Password, defaults.Ops.DefaultAdmin.Password, "change-me") {
		problems = append(problems, "ops.default_admin.password must be replaced or default admin disabled")
	}
	if weakSecret(cfg.Postgres.Password, defaults.Postgres.Password, "change-me") {
		problems = append(problems, "postgres.password must be replaced")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid production config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func isProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SLAN_ENV")))
	return env == "prod" || env == "production"
}

func weakSecret(value, defaultValue, placeholder string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == defaultValue {
		return true
	}
	return strings.Contains(strings.ToLower(trimmed), placeholder)
}

func isLoopbackPublicHost(host string) bool {
	value := strings.ToLower(strings.TrimSpace(host))
	if value == "" {
		return true
	}
	hostOnly := value
	if strings.HasPrefix(hostOnly, "[::1]") {
		return true
	}
	if index := strings.LastIndex(hostOnly, ":"); index > -1 {
		hostOnly = hostOnly[:index]
	}
	return hostOnly == "localhost" ||
		hostOnly == "127.0.0.1" ||
		strings.HasPrefix(hostOnly, "127.") ||
		hostOnly == "::1"
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
