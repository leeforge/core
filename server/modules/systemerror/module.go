package systemerror

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// SystemErrorModule manages the system error module
type SystemErrorModule struct {
	logger  logging.Logger
	service *SystemErrorService
	handler *SystemErrorHandler
}

// NewSystemErrorModule creates a new system error module
func NewSystemErrorModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	service := NewSystemErrorService(deps.Client, logger)
	handler := NewSystemErrorHandler(service, logger)

	return &SystemErrorModule{
		logger:  logger,
		service: service,
		handler: handler,
	}
}

func (m *SystemErrorModule) Name() string {
	return "systemerror"
}

// RegisterPublicRoutes registers public system error endpoints (none)
func (m *SystemErrorModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for system errors
}

// RegisterPrivateRoutes registers protected system error endpoints
func (m *SystemErrorModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/logs/errors", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.List,
			framePerm.Private("List system errors", "log.error.read"))

		framePerm.Post(r, "/clear", m.handler.Clear,
			framePerm.Private("Clear old system errors", "log.error.manage"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Delete(r, "/", m.handler.Delete,
				framePerm.Private("Delete system error", "log.error.delete"))
		})
	})
}

// Middleware returns the recovery middleware
func (m *SystemErrorModule) Middleware(logger logging.Logger) func(http.Handler) http.Handler {
	return RecoveryMiddleware(m.service, logger)
}
