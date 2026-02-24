package menu

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type MenuHandler struct {
	menuService *MenuService
	logger      logging.Logger
}

func NewMenuHandler(menuService *MenuService, logger logging.Logger) *MenuHandler {
	return &MenuHandler{
		menuService: menuService,
		logger:      logger,
	}
}

// GetMenuTree handles getting menu tree based on user's RBAC permissions.
// @Summary Get menu tree for current user
// @Description Returns menu tree filtered by user's role permissions. Super admin sees all menus.
// @Tags Menus
// @Accept json
// @Produce json
// @Success 200 {object} []MenuNode
// @Failure 401 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /menus/tree [get]
func (h *MenuHandler) GetMenuTree(w http.ResponseWriter, r *http.Request) {
	identity, ok := core.GetIdentity(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "Authentication required")
		return
	}

	tree, err := h.menuService.GetMenuTree(r.Context(), identity.Roles, identity.IsSuperAdmin)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to get menu tree", err)
		responder.InternalServerError(w, r, "Failed to get menu tree")
		return
	}
	responder.OK(w, r, tree)
}

// CreateMenu handles menu creation.
// @Summary Create a menu
// @Tags Menus
// @Accept json
// @Produce json
// @Param request body CreateMenuRequest true "Menu data"
// @Success 201 {object} ent.Menu
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /menus [post]
func (h *MenuHandler) CreateMenu(w http.ResponseWriter, r *http.Request) {
	var input CreateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	// Convert DTO to ent.Menu
	menuInput := &ent.Menu{
		Name:      input.Name,
		Path:      input.Path,
		Component: input.Component,
		Redirect:  input.Redirect,
		Icon:      input.Icon,
		Sort:      input.Sort,
		Hidden:    input.Hidden,
		Affix:     input.Affix,
		Meta:      input.Meta,
		Params:    input.Params,
	}

	if input.ParentID != nil {
		menuInput.ParentID = *input.ParentID
	}

	res, err := h.menuService.CreateMenu(r.Context(), menuInput)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to create menu")
		return
	}
	responder.Created(w, r, res)
}

// UpdateMenu handles menu updates.
// @Summary Update a menu
// @Tags Menus
// @Accept json
// @Produce json
// @Param id path string true "Menu ID"
// @Param request body UpdateMenuRequest true "Menu data"
// @Success 200 {object} ent.Menu
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /menus/{id} [put]
func (h *MenuHandler) UpdateMenu(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid menu ID")
		return
	}

	var input UpdateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	// Convert DTO to ent.Menu
	menuInput := &ent.Menu{
		Name:      input.Name,
		Path:      input.Path,
		Component: input.Component,
		Redirect:  input.Redirect,
		Icon:      input.Icon,
		Sort:      input.Sort,
		Hidden:    input.Hidden,
		Affix:     input.Affix,
		Meta:      input.Meta,
		Params:    input.Params,
	}

	if input.ParentID != nil {
		menuInput.ParentID = *input.ParentID
	}

	res, err := h.menuService.UpdateMenu(r.Context(), id, menuInput)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to update menu")
		return
	}
	responder.OK(w, r, res)
}

// DeleteMenu handles menu deletion.
// @Summary Delete a menu
// @Tags Menus
// @Accept json
// @Produce json
// @Param id path string true "Menu ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /menus/{id} [delete]
func (h *MenuHandler) DeleteMenu(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid menu ID")
		return
	}

	if err := h.menuService.DeleteMenu(r.Context(), id); err != nil {
		responder.InternalServerError(w, r, "Failed to delete menu")
		return
	}
	responder.OK(w, r, map[string]string{"message": "Menu and submenus deleted successfully"})
}

// SetupMenuRoutes registers menu management routes.
func SetupMenuRoutes(r chi.Router, h *MenuHandler, jwtMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/menus", func(r chi.Router) {
		r.Use(jwtMiddleware)

		r.Get("/tree", h.GetMenuTree)
		r.Post("/", h.CreateMenu)

		r.Route("/{id}", func(r chi.Router) {
			r.Put("/", h.UpdateMenu)
			r.Delete("/", h.DeleteMenu)
		})
	})
}
