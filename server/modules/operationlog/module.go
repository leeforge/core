package operationlog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// OperationLogModule manages the operation log module
type OperationLogModule struct {
	logger  logging.Logger
	service *OperationLogService
	handler *OperationLogHandler
}

// NewOperationLogModule creates a new operation log module
func NewOperationLogModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	service := NewOperationLogService(deps.Client, logger)
	handler := NewOperationLogHandler(service, logger)

	return &OperationLogModule{
		logger:  logger,
		service: service,
		handler: handler,
	}
}

func (m *OperationLogModule) Name() string {
	return "operationlog"
}

// RegisterPublicRoutes registers public operation log endpoints (none)
func (m *OperationLogModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for operation logs
}

// RegisterPrivateRoutes registers protected operation log endpoints
func (m *OperationLogModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/logs/operations", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.List,
			framePerm.Private("List operation logs", "log.operation.read"))

		framePerm.Post(r, "/clear", m.handler.Clear,
			framePerm.Private("Clear old operation logs", "log.operation.manage"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Delete(r, "/", m.handler.Delete,
				framePerm.Private("Delete operation log", "log.operation.delete"))
		})
	})
}

// Middleware returns the operation logging middleware
func (m *OperationLogModule) Middleware(logger logging.Logger) func(http.Handler) http.Handler {
	return Middleware(m.service, logger)
}
