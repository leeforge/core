package initmod

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type InitModule struct {
	logger  logging.Logger
	handler *InitHandler
}

func NewInitModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize Service
	svc := NewInitService(deps.Client, deps.Config, logger, deps.DomainService)

	// Initialize Handler
	handler := NewInitHandler(svc, logger)

	return &InitModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *InitModule) Name() string {
	return "init"
}

// RegisterPublicRoutes registers public init endpoints (status, setup)
func (m *InitModule) RegisterPublicRoutes(r chi.Router) {
	r.Route("/init", func(r chi.Router) {
		framePerm.Get(r, "/status", m.handler.Status, framePerm.Public("Check initialization status", "system.init.status"))
		framePerm.Post(r, "/setup", m.handler.Setup, framePerm.Public("Initialize system", "system.init.setup"))
	})
}

// RegisterPrivateRoutes - init module has no private routes
func (m *InitModule) RegisterPrivateRoutes(r chi.Router) {
	// No private routes for init module
}

// Ensure InitModule implements Module interface
var _ core.Module = (*InitModule)(nil)
