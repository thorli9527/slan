package biz

import (
	"database/sql"
	"net/http"
)

// Server 是 service-biz 的 HTTP/MQTT 业务入口。
// 它只依赖 BusinessStore 抽象，避免入口层直接绑定具体存储实现。
type Server struct {
	store    BusinessStore
	services ApplicationServices
	mqtt     MQTTConfig
}

// NewServer 使用默认 Store 创建服务，适合本地测试或无显式数据库注入的场景。
func NewServer() *Server {
	return NewServerWithStore(NewStore())
}

// NewServerWithPostgres 使用 Postgres 初始化默认 Store，并返回业务服务。
func NewServerWithPostgres(db *sql.DB) *Server {
	return NewServerWithStore(NewStoreWithPostgres(db))
}

// NewServerWithStore 使用调用方提供的业务存储实现创建服务。
func NewServerWithStore(store BusinessStore) *Server {
	if store == nil {
		store = NewStore()
	}
	mqtt := mqttConfigFromEnv()
	server := &Server{
		store:    store,
		services: newApplicationServices(store, mqtt),
		mqtt:     mqtt,
	}
	server.startMQTTControlSubscriber()
	server.startMQTTDeliveryRetryWorker()
	return server
}

func (s *Server) refreshServices() {
	s.services = newApplicationServices(s.store, s.mqtt)
}

func (s *Server) ensureServices() {
	if s.services.Auth.store == nil {
		s.refreshServices()
	}
}

// Routes 注册 service-biz 对外 HTTP、内部 wire、ops 和 MQTT webhook 路由。
func (s *Server) Routes() http.Handler {
	s.refreshServices()
	mux := http.NewServeMux()
	s.registerBaseRoutes(mux)
	s.registerExternalAPIRoutes(mux)
	s.registerInternalWireRoutes(mux)
	s.registerOpsRoutes(mux)
	s.registerMQTTRoutes(mux)
	return withCORS(mux)
}
