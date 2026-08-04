package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	opsapi "github.com/slan/service-biz/internal/api/ops"
)

type Server struct {
	container Container
}

func NewServer() *Server {
	return NewServerWithContainer(NewDefaultContainer())
}

func NewServerWithContainer(container Container) *Server {
	return &Server{container: container}
}

func (s *Server) Container() Container {
	return s.container
}

func (s *Server) Routes() http.Handler {
	return s.RoutesFor(RouteSetAll)
}

func (s *Server) RoutesFor(routeSet string) http.Handler {
	mux := http.NewServeMux()
	serviceapi.RegisterRoutes(mux, s.baseRoutes())
	serviceapi.RegisterRoutes(mux, s.routesFor(routeSet))
	return serviceapi.WithCORS(opsapi.WithOperatorAuth(mux, s.container.RouteUseCases.Ops.SessionAuth))
}

func (s *Server) baseRoutes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/healthz", func(w http.ResponseWriter, r *http.Request) {
			serviceapi.WriteEnvelope(w, http.StatusOK, "status", "ok")
		}),
	}
}
