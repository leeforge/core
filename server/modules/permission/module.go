package permission

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type PermissionModule struct {
	logger  logging.Logger
	handler *PermissionHandler
}

func NewPermissionModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	svc := NewPermissionService(deps.Client, logger, deps.Router, deps.PermSyncer)
	handler := NewPermissionHandler(svc, logger)

	return &PermissionModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *PermissionModule) Name() string {
	return "permission"
}

// RegisterPublicRoutes - permission module has no public routes.
func (m *PermissionModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for permission module
}

// RegisterPrivateRoutes registers protected permission endpoints.
func (m *PermissionModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/permissions", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.ListPermissions, framePerm.Private("List permissions", "permission.read"))
		framePerm.Post(r, "/sync", m.handler.SyncEndpoints, framePerm.Private("Sync permissions", "permission.manage"))
		framePerm.Get(r, "/sync-status", m.handler.GetSyncStatus, framePerm.Private("Get sync status", "permission.read"))
	})
}

// Ensure PermissionModule implements Module interface.
var _ core.Module = (*PermissionModule)(nil)
