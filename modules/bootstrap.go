package modules

import (
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Bootstrap registers core-owned business modules on the router.
func Bootstrap(router chi.Router, cfg any, logger *zap.Logger) error {
	return registerBusinessModules(router, cfg, logger)
}
