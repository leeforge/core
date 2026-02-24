package middleware

import (
	"net/http"
	"strings"

	"github.com/leeforge/framework/http/responder"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/services/datascope"
)

// DataScopeMiddleware resolves and injects data-scope filter into request context.
func DataScopeMiddleware(scopeService *datascope.Service, resolver *RoutePermissionResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if scopeService == nil {
				next.ServeHTTP(w, r)
				return
			}

			ac := core.GetActingContext(r.Context())
			if ac == nil {
				next.ServeHTTP(w, r)
				return
			}
			// Skip data scope for platform domain (uses Domain.TypeCode when available)
			if ac.IsPlatformDomain() {
				next.ServeHTTP(w, r)
				return
			}

			identity, ok := core.GetIdentity(r.Context())
			if !ok {
				responder.Unauthorized(w, r, "Missing identity")
				return
			}

			resourceKey := resolveResourceKey(resolver, r)
			if resourceKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			fc, err := scopeService.GetUserDataScope(
				r.Context(),
				identity.UserID,
				ac.CasbinDomain(),
				resourceKey,
			)
			if err != nil {
				if err == datascope.ErrNoRole {
					responder.Forbidden(w, r, "No role for data access in acting domain")
					return
				}
				responder.InternalServerError(w, r, "Failed to resolve data scope")
				return
			}

			ctx := datascope.WithFilterCondition(r.Context(), fc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveResourceKey(resolver *RoutePermissionResolver, r *http.Request) string {
	if r == nil || resolver == nil {
		return ""
	}

	routePerm, found := resolver.Resolve(r)
	if !found || len(routePerm.Permissions) == 0 {
		return ""
	}

	for _, code := range routePerm.Permissions {
		trimmed := strings.TrimSpace(code)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
