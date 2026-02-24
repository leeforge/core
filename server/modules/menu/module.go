package menu

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type MenuModule struct {
	logger  logging.Logger
	handler *MenuHandler
}

func NewMenuModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize Service
	svc := NewMenuService(deps.Client, logger)

	// Initialize Handler
	handler := NewMenuHandler(svc, logger)

	return &MenuModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *MenuModule) Name() string {
	return "menu"
}

// RegisterPublicRoutes - menu module has no public routes
func (m *MenuModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for menu module
}

// RegisterPrivateRoutes registers protected menu endpoints
func (m *MenuModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/menus", func(r chi.Router) {
		framePerm.Get(r, "/tree", m.handler.GetMenuTree, framePerm.Private("Get menu tree", "menu.read"))
		framePerm.Post(r, "/", m.handler.CreateMenu, framePerm.Private("Create menu", "menu.write"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Put(r, "/", m.handler.UpdateMenu, framePerm.Private("Update menu", "menu.write"))
			framePerm.Delete(r, "/", m.handler.DeleteMenu, framePerm.Private("Delete menu", "menu.delete"))
		})
	})
}
