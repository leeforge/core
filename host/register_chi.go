package host

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func RegisterAllChi(router chi.Router, opts CoreOptions) error {
	if router == nil {
		return fmt.Errorf("router is nil")
	}

	registry := newRouteRegistry()
	routed := chi.Router(newConflictRouter(router, "", "runtime", registry))
	if existing, ok := router.(*conflictRouter); ok && existing.registry != nil {
		registry = existing.registry
		routed = existing
	}

	if !opts.SkipMigrate && opts.MigrationRunner != nil {
		if err := opts.MigrationRunner(context.Background()); err != nil {
			return &ErrMigration{Cause: err}
		}
	}

	rt := newChiRuntimeAdapter(router)
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	if opts.PluginRegistrar != nil {
		if err := opts.PluginRegistrar(rt, opts.Config, logger); err != nil {
			return &ErrPluginBootstrap{Cause: err}
		}
	}
	if opts.ModuleBootstrapper != nil {
		target := chi.Router(routed)
		if scoped, ok := any(routed).(interface{ WithOwner(string) chi.Router }); ok {
			target = scoped.WithOwner("modules")
		}
		if err := opts.ModuleBootstrapper(target, opts.Config, logger); err != nil {
			return &ErrModuleBootstrap{Cause: err}
		}
	}

	base := opts.BasePath
	if base == "" {
		base = DefaultBasePath
	}
	healthPath := joinPath(base, "/health")
	healthRouter := routed
	if scoped, ok := any(routed).(interface{ WithOwner(string) chi.Router }); ok {
		healthRouter = scoped.WithOwner("core-runtime")
	}
	healthRouter.Get(healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	if registry.conflict != nil {
		return registry.conflict
	}
	return nil
}

type chiRuntimeAdapter struct {
	router  chi.Router
	plugins map[string]any
}

func newChiRuntimeAdapter(router chi.Router) *chiRuntimeAdapter {
	return &chiRuntimeAdapter{
		router:  router,
		plugins: make(map[string]any),
	}
}

func (a *chiRuntimeAdapter) RegisterPlugin(name string, p any) error {
	if name == "" {
		return fmt.Errorf("plugin name is empty")
	}
	if _, exists := a.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	a.plugins[name] = p
	return nil
}

func (a *chiRuntimeAdapter) Router() chi.Router {
	return a.router
}
