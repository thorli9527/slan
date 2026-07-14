package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func withRequiredUserSession(routes []serviceapi.Route, sessions servicepkg.AuthUserSessionUseCase) []serviceapi.Route {
	protected := make([]serviceapi.Route, 0, len(routes))
	for _, route := range routes {
		current := route
		protected = append(protected, serviceapi.NewRoute(current.Method, current.Path, func(w http.ResponseWriter, r *http.Request) {
			if sessions == nil {
				serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
				return
			}
			auth, err := sessions.GetUserSession(r.Context(), serviceapi.AccessTokenFromRequest(r))
			if err != nil {
				serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
				return
			}
			ctx := serviceapi.WithAuthenticatedUser(r.Context(), auth.User.UserID)
			current.Handler(w, r.WithContext(ctx))
		}))
	}
	return protected
}
