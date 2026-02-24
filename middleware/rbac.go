package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"

	"github.com/leeforge/core/core"
)

// PermissionChecker validates domain-aware permission codes.
type PermissionChecker interface {
	CheckUserPermission(ctx context.Context, userUUID, domain, resource, action string) (bool, error)
}

// RoutePermission describes route-level public/private metadata.
type RoutePermission struct {
	IsPublic    bool
	Permissions []string
}

// RoutePermissionResolver resolves route metadata from a permission snapshot.
type RoutePermissionResolver struct {
	mu    sync.RWMutex
	byKey map[string]RoutePermission
}

// NewRoutePermissionResolver creates a resolver with optional initial snapshot.
func NewRoutePermissionResolver(snapshot *framePerm.Snapshot) *RoutePermissionResolver {
	r := &RoutePermissionResolver{
		byKey: make(map[string]RoutePermission),
	}
	if snapshot != nil {
		r.Update(*snapshot)
	}
	return r
}

// Update replaces route permission map from snapshot.
func (r *RoutePermissionResolver) Update(snapshot framePerm.Snapshot) {
	mapped := make(map[string]RoutePermission, len(snapshot.Routes))
	for _, route := range snapshot.Routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		path := normalizeRoutePath(route.Path)
		if method == "" || path == "" {
			continue
		}
		mapped[routePermissionKey(method, path)] = RoutePermission{
			IsPublic:    route.IsPublic,
			Permissions: clonePermissions(route.Permissions),
		}
	}

	r.mu.Lock()
	r.byKey = mapped
	r.mu.Unlock()
}

// Resolve resolves route permission metadata for a request.
func (r *RoutePermissionResolver) Resolve(req *http.Request) (RoutePermission, bool) {
	if r == nil || req == nil {
		return RoutePermission{}, false
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		return RoutePermission{}, false
	}

	paths := candidateRoutePaths(req)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, path := range paths {
		if permission, ok := r.byKey[routePermissionKey(method, path)]; ok {
			return permission, true
		}
	}
	return RoutePermission{}, false
}

// RBACMiddleware enforces domain-aware RBAC from request ActingContext.
func RBACMiddleware(
	checker PermissionChecker,
	resolver *RoutePermissionResolver,
	logger logging.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if checker == nil || resolver == nil {
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

			routePerm, found := resolver.Resolve(r)
			if !found {
				responder.Forbidden(w, r, "Missing route permission metadata")
				return
			}
			if routePerm.IsPublic || len(routePerm.Permissions) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ac := core.GetActingContext(r.Context())
			if ac == nil {
				responder.Forbidden(w, r, "Missing acting domain")
				return
			}
			domain := ac.CasbinDomain()
			if domain == "" {
				responder.Forbidden(w, r, "Missing acting domain")
				return
			}

			actions := candidateRBACActions(r.Method)
			userUUID := identity.UserID.String()
			for _, permissionCode := range routePerm.Permissions {
				resource := strings.TrimSpace(permissionCode)
				if resource == "" {
					continue
				}
				for _, action := range actions {
					allowed, err := checker.CheckUserPermission(r.Context(), userUUID, domain, resource, action)
					if err != nil {
						logger.Error("RBAC permission check failed",
							zap.Error(err),
							zap.String("user_id", userUUID),
							zap.String("domain", domain),
							zap.String("resource", resource),
							zap.String("action", action),
						)
						responder.InternalServerError(w, r, "Permission check failed")
						return
					}
					if allowed {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			responder.Forbidden(w, r, "Permission denied")
		})
	}
}

func routePermissionKey(method, path string) string {
	return method + " " + path
}

func normalizeRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
	}
	return path
}

func candidateRoutePaths(req *http.Request) []string {
	paths := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	appendPath := func(path string) {
		normalized := normalizeRoutePath(path)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		paths = append(paths, normalized)
	}

	if rctx := chi.RouteContext(req.Context()); rctx != nil {
		appendPath(rctx.RoutePattern())
	}
	appendPath(req.URL.Path)
	return paths
}

func clonePermissions(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func candidateRBACActions(method string) []string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return []string{"read", "execute"}
	case http.MethodPost:
		return []string{"create", "execute"}
	case http.MethodPut, http.MethodPatch:
		return []string{"update", "execute"}
	case http.MethodDelete:
		return []string{"delete", "execute"}
	default:
		return []string{"execute"}
	}
}
