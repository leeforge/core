package modules

import (
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	corecore "github.com/leeforge/core/core"
)

// Bootstrap registers core-owned business modules plus external module factories on the router.
func Bootstrap(router chi.Router, cfg any, logger *zap.Logger, factories ...corecore.ModuleFactory) error {
	return registerBusinessModules(router, cfg, logger, factories...)
}
