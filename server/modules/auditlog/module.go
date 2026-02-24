package auditlog

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// AuditLogModule manages the audit log module.
type AuditLogModule struct {
	logger  logging.Logger
	service *AuditLogService
	handler *AuditLogHandler
}

// NewAuditLogModule creates a new audit log module.
func NewAuditLogModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	service := NewAuditLogService(deps.Client, logger)
	handler := NewAuditLogHandler(service, logger)

	return &AuditLogModule{
		logger:  logger,
		service: service,
		handler: handler,
	}
}

func (m *AuditLogModule) Name() string {
	return "auditlog"
}

// RegisterPublicRoutes registers public audit log endpoints (none).
func (m *AuditLogModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for audit logs.
}

// RegisterPrivateRoutes registers protected audit log endpoints.
func (m *AuditLogModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/logs/audits", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.List,
			framePerm.Private("List audit logs", "log.audit.read"))

		framePerm.Post(r, "/clear", m.handler.Clear,
			framePerm.Private("Clear old audit logs", "log.audit.manage"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Delete(r, "/", m.handler.Delete,
				framePerm.Private("Delete audit log", "log.audit.delete"))
		})
	})
}
