package role

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type RoleModule struct {
	logger  logging.Logger
	handler *RoleHandler
}

func NewRoleModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Get RBAC Manager from dependencies
	// Note: Dependencies should include PermissionManager
	// For now, assuming deps.PermManager.RBACManager is available

	// Initialize Service with RBAC Manager
	svc := NewRoleService(deps.Client, logger, deps.PermManager.RBACManager)

	// Initialize Handler
	handler := NewRoleHandler(svc, logger)

	return &RoleModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *RoleModule) Name() string {
	return "role"
}

// RegisterPublicRoutes - role module has no public routes
func (m *RoleModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for role module
}

// RegisterPrivateRoutes registers protected role endpoints
func (m *RoleModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/roles", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.ListRoles, framePerm.Private("List roles", "role.read"))
		framePerm.Post(r, "/", m.handler.CreateRole, framePerm.Private("Create role", "role.write"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Get(r, "/", m.handler.GetRole, framePerm.Private("Get role details", "role.read"))
			framePerm.Put(r, "/", m.handler.UpdateRole, framePerm.Private("Update role", "role.write"))
			framePerm.Delete(r, "/", m.handler.DeleteRole, framePerm.Private("Delete role", "role.delete"))

			framePerm.Post(r, "/copy", m.handler.CopyRole, framePerm.Private("Copy role", "role.write"))
			framePerm.Post(r, "/menus", m.handler.SetRoleMenus, framePerm.Private("Set role menus", "role.manage"))
			framePerm.Post(r, "/permissions", m.handler.SetRolePermissions, framePerm.Private("Set role permissions", "role.manage"))
			framePerm.Put(r, "/data-scope-rules", m.handler.UpsertRoleDataScopeRule, framePerm.Private("Upsert role data scope rule", "role.manage"))
			framePerm.Get(r, "/data-scope-rules", m.handler.ListRoleDataScopeRules, framePerm.Private("List role data scope rules", "role.read"))
			framePerm.Delete(r, "/data-scope-rules", m.handler.DeleteRoleDataScopeRule, framePerm.Private("Delete role data scope rule", "role.manage"))
			framePerm.Get(r, "/users", m.handler.ListRoleUsers, framePerm.Private("List role users", "role.read"))
		})
	})
}
