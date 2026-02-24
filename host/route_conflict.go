package host

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type routeRegistry struct {
	byPath   map[string]map[string]string
	conflict *ErrRouteConflict
}

func newRouteRegistry() *routeRegistry {
	return &routeRegistry{
		byPath: make(map[string]map[string]string),
	}
}

func (r *routeRegistry) register(method, path, owner string) {
	if r == nil || r.conflict != nil {
		return
	}
	m := normalizeMethod(method)
	p := normalizeRoutePath(path)
	if m == "" || p == "" {
		return
	}
	if owner == "" {
		owner = "unknown"
	}

	methods := r.byPath[p]
	if methods == nil {
		methods = make(map[string]string)
		r.byPath[p] = methods
	}

	if m == "*" {
		if len(methods) > 0 {
			for _, existing := range methods {
				r.conflict = &ErrRouteConflict{
					Method:         m,
					Path:           p,
					OwnerModule:    owner,
					ConflictModule: existing,
				}
				return
			}
		}
		methods[m] = owner
		return
	}

	if existing, ok := methods[m]; ok {
		r.conflict = &ErrRouteConflict{
			Method:         m,
			Path:           p,
			OwnerModule:    owner,
			ConflictModule: existing,
		}
		return
	}
	if existing, ok := methods["*"]; ok {
		r.conflict = &ErrRouteConflict{
			Method:         m,
			Path:           p,
			OwnerModule:    owner,
			ConflictModule: existing,
		}
		return
	}
	methods[m] = owner
}

func (r *routeRegistry) lookup(method, path string) (string, bool) {
	if r == nil {
		return "", false
	}
	p := normalizeRoutePath(path)
	methods := r.byPath[p]
	if methods == nil {
		return "", false
	}
	m := normalizeMethod(method)
	if owner, ok := methods[m]; ok {
		return owner, true
	}
	if owner, ok := methods["*"]; ok {
		return owner, true
	}
	return "", false
}

type conflictRouter struct {
	chi.Router
	prefix   string
	owner    string
	registry *routeRegistry
}

func newConflictRouter(base chi.Router, prefix, owner string, registry *routeRegistry) *conflictRouter {
	if base == nil {
		base = chi.NewRouter()
	}
	if registry == nil {
		registry = newRouteRegistry()
	}
	return &conflictRouter{
		Router:   base,
		prefix:   prefix,
		owner:    owner,
		registry: registry,
	}
}

// NewRouteRegistryRouter creates a chi router that tracks route ownership.
func NewRouteRegistryRouter(base chi.Router, owner string) chi.Router {
	return newConflictRouter(base, "", owner, newRouteRegistry())
}

func (r *conflictRouter) WithOwner(owner string) chi.Router {
	return newConflictRouter(r.Router, r.prefix, owner, r.registry)
}

func (r *conflictRouter) LookupRouteOwner(method, path string) (string, bool) {
	return r.registry.lookup(method, path)
}

func (r *conflictRouter) Method(method, pattern string, handler http.Handler) {
	full := joinPath(r.prefix, pattern)
	r.registry.register(method, full, r.owner)
	r.Router.Method(method, pattern, handler)
}

func (r *conflictRouter) MethodFunc(method, pattern string, handler http.HandlerFunc) {
	full := joinPath(r.prefix, pattern)
	r.registry.register(method, full, r.owner)
	r.Router.MethodFunc(method, pattern, handler)
}

func (r *conflictRouter) Handle(pattern string, handler http.Handler) {
	full := joinPath(r.prefix, pattern)
	r.registry.register("*", full, r.owner)
	r.Router.Handle(pattern, handler)
}

func (r *conflictRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	full := joinPath(r.prefix, pattern)
	r.registry.register("*", full, r.owner)
	r.Router.HandleFunc(pattern, handler)
}

func (r *conflictRouter) Route(pattern string, fn func(chi.Router)) chi.Router {
	nextPrefix := joinPath(r.prefix, pattern)
	return r.Router.Route(pattern, func(sr chi.Router) {
		if fn == nil {
			return
		}
		fn(newConflictRouter(sr, nextPrefix, r.owner, r.registry))
	})
}

func (r *conflictRouter) Mount(pattern string, handler http.Handler) {
	full := joinPath(r.prefix, pattern)
	if routes, ok := handler.(chi.Routes); ok {
		_ = chi.Walk(routes, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			r.registry.register(method, joinPath(full, route), r.owner)
			return nil
		})
	} else {
		r.registry.register("*", full, r.owner)
	}
	r.Router.Mount(pattern, handler)
}

func (r *conflictRouter) With(middlewares ...func(http.Handler) http.Handler) chi.Router {
	return newConflictRouter(r.Router.With(middlewares...), r.prefix, r.owner, r.registry)
}

func (r *conflictRouter) Group(fn func(chi.Router)) chi.Router {
	return r.Router.Group(func(sr chi.Router) {
		if fn == nil {
			return
		}
		fn(newConflictRouter(sr, r.prefix, r.owner, r.registry))
	})
}

func (r *conflictRouter) Get(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodGet, pattern, handler)
}

func (r *conflictRouter) Post(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodPost, pattern, handler)
}

func (r *conflictRouter) Put(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodPut, pattern, handler)
}

func (r *conflictRouter) Patch(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodPatch, pattern, handler)
}

func (r *conflictRouter) Delete(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodDelete, pattern, handler)
}

func (r *conflictRouter) Options(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodOptions, pattern, handler)
}

func (r *conflictRouter) Head(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodHead, pattern, handler)
}

func (r *conflictRouter) Trace(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodTrace, pattern, handler)
}

func (r *conflictRouter) Connect(pattern string, handler http.HandlerFunc) {
	r.Method(http.MethodConnect, pattern, handler)
}

func normalizeMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizeRoutePath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return "/"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if clean != "/" {
		clean = strings.TrimSuffix(clean, "/")
	}
	return clean
}

func joinPath(prefix, route string) string {
	left := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	right := strings.TrimSpace(route)
	if right == "" {
		right = "/"
	}
	if !strings.HasPrefix(right, "/") {
		right = "/" + right
	}
	if right != "/" {
		right = strings.TrimSuffix(right, "/")
	}
	if left == "" {
		return right
	}
	return left + right
}
