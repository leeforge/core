package modules

import (
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	corecore "github.com/leeforge/core/core"
)

// Bootstrap registers core-owned business modules on the router.
func Bootstrap(router chi.Router, cfg any, logger *zap.Logger) error {
	return registerBusinessModules(router, cfg, logger)
}

// BootstrapWithExtras registers core-owned business modules plus external module factories on the router.
func BootstrapWithExtras(router chi.Router, cfg any, logger *zap.Logger, extras []corecore.ModuleFactory) error {
	return registerBusinessModules(router, cfg, logger, extras...)
}
