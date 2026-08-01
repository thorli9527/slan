package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func withRequiredOperatorSession(routes []serviceapi.Route, sessions servicepkg.OpsAuthSessionUseCase) []serviceapi.Route {
	protected := make([]serviceapi.Route, 0, len(routes))
	for _, route := range routes {
		current := route
		if current.Method == http.MethodPost && (current.Path == "/api/ops/auth/login" || current.Path == "/api/opt/auth/login") {
			protected = append(protected, current)
			continue
		}
		protected = append(protected, serviceapi.NewRoute(current.Method, current.Path, func(w http.ResponseWriter, r *http.Request) {
			if sessions == nil {
				serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
				return
			}
			auth, err := sessions.Authenticate(r.Context(), serviceapi.AccessTokenFromRequest(r))
			if err != nil {
				serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
				return
			}
			ctx := serviceapi.WithAuthenticatedOperator(r.Context(), auth.Operator.OperatorID)
			current.Handler(w, r.WithContext(ctx))
		}))
	}
	return protected
}
