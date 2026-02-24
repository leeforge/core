package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/leeforge/framework/http/responder"

	"github.com/leeforge/core/core"
)

// ABACChecker abstracts ABAC permission checks.
type ABACChecker interface {
	CheckABACPermission(
		ctx context.Context,
		userAttrs map[string]any,
		resourceAttrs map[string]any,
		action string,
		contextAttrs map[string]any,
	) (bool, error)
}

// ABACConfig controls ABAC middleware behavior.
type ABACConfig struct {
	Enabled bool
}

// DefaultABACConfig returns default ABAC config.
func DefaultABACConfig() *ABACConfig {
	return &ABACConfig{Enabled: false}
}

// ABACMiddleware evaluates dynamic policy constraints after DataScope.
func ABACMiddleware(checker ABACChecker, cfg *ABACConfig) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultABACConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled || checker == nil {
				next.ServeHTTP(w, r)
				return
			}

			identity, ok := core.GetIdentity(r.Context())
			if !ok {
				responder.Unauthorized(w, r, "Missing identity")
				return
			}
			if identity.IsSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			tenantID, _ := core.GetTenantID(r.Context())
			projectID, _ := core.GetProjectID(r.Context())

			userAttrs := map[string]any{
				"user_id":        identity.UserID.String(),
				"username":       identity.Username,
				"identity_type":  string(identity.Type),
				"is_super_admin": identity.IsSuperAdmin,
				"tenant_id":      tenantID,
			}
			resourceAttrs := map[string]any{
				"path":       r.URL.Path,
				"method":     strings.ToUpper(strings.TrimSpace(r.Method)),
				"tenant_id":  tenantID,
				"project_id": projectID,
			}
			contextAttrs := map[string]any{
				"ip":        r.RemoteAddr,
				"userAgent": r.UserAgent(),
				"time":      time.Now().UTC().Format(time.RFC3339),
			}

			action := abacActionFromMethod(r.Method)
			allowed, err := checker.CheckABACPermission(r.Context(), userAttrs, resourceAttrs, action, contextAttrs)
			if err != nil {
				responder.InternalServerError(w, r, "ABAC evaluation failed")
				return
			}
			if !allowed {
				responder.Forbidden(w, r, "ABAC policy denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func abacActionFromMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return "read"
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "execute"
	}
}
