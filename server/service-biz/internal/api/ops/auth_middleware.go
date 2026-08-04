package ops

import (
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func WithOperatorAuth(next http.Handler, sessions servicepkg.OpsAuthSessionUseCase) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !operatorProtectedPath(r.URL.Path) || operatorLoginPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		operator, err := sessions.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			serviceapi.WriteError(w, err)
			return
		}
		if operatorAdminPath(r.URL.Path) && operator.Role != "admin" {
			serviceapi.WriteError(w, servicepkg.ErrForbidden)
			return
		}
		ctx := servicepkg.WithAuthenticatedOperator(r.Context(), operator.OperatorID)
		ctx = servicepkg.WithAuthenticatedOperatorRole(ctx, operator.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func operatorAdminPath(path string) bool {
	return strings.HasPrefix(path, "/api/ops/operators") || strings.HasPrefix(path, "/api/opt/operators")
}

func operatorProtectedPath(path string) bool {
	return strings.HasPrefix(path, "/api/ops/") || strings.HasPrefix(path, "/api/opt/")
}

func operatorLoginPath(path string) bool {
	return path == "/api/ops/auth/login" || path == "/api/opt/auth/login"
}

func bearerToken(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return ""
	}
	return strings.TrimSpace(authorization[7:])
}
