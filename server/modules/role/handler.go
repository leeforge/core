package role

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"

	apperrors "github.com/leeforge/core/server/pkg/errors"
)

type RoleHandler struct {
	roleService *RoleService
	logger      logging.Logger
}

func NewRoleHandler(roleService *RoleService, logger logging.Logger) *RoleHandler {
	return &RoleHandler{
		roleService: roleService,
		logger:      logger,
	}
}

func handleRoleServiceError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		status := apperrors.GetHTTPStatus(appErr.Code)
		responder.CustomError(w, r, status, appErr.Code, appErr.Message, nil)
		return true
	}
	return false
}

// GetRole handles getting role details.
// @Summary Get role details
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id} [get]
func (h *RoleHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	res, err := h.roleService.GetRole(r.Context(), id)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to get role")
		return
	}

	responder.OK(w, r, res)
}

// ListRoles handles listing roles.
// @Summary List roles
// @Tags Roles
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} responder.Response
// @Failure 500 {object} responder.Error
// @Router /roles [get]
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	roles, total, err := h.roleService.ListRoles(r.Context(), page, pageSize)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to list roles")
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	pagination := &responder.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	responder.WriteList(w, r, http.StatusOK, roles, pagination)
}

// CreateRole handles role creation.
// @Summary Create a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param request body CreateRoleRequest true "Role data"
// @Success 201 {object} ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles [post]
func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	res, err := h.roleService.CreateRoleV2(r.Context(), CreateRoleInput{
		Name:             req.Name,
		Code:             req.Code,
		Description:      req.Description,
		Sort:             req.Sort,
		ParentID:         req.ParentID,
		Permissions:      req.Permissions,
		DefaultDataScope: req.DefaultDataScope,
	})
	if err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to create role")
		return
	}

	responder.Created(w, r, res)
}

// UpdateRole handles role update.
// @Summary Update a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body UpdateRoleRequest true "Role data"
// @Success 200 {object} ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id} [put]
func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	res, err := h.roleService.UpdateRoleV2(r.Context(), UpdateRoleInput{
		ID:               id,
		Name:             req.Name,
		Description:      req.Description,
		Sort:             req.Sort,
		ParentID:         req.ParentID,
		HasParentID:      true,
		DefaultDataScope: req.DefaultDataScope,
	})
	if err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to update role")
		return
	}

	responder.OK(w, r, res)
}

// DeleteRole handles role deletion.
// @Summary Delete a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id} [delete]
func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	if err := h.roleService.DeleteRole(r.Context(), id); err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to delete role")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Role deleted successfully"})
}

// CopyRole handles cloning a role.
// @Summary Clone a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Source Role ID"
// @Param request body CopyRoleRequest true "Cloning data"
// @Success 200 {object} ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/copy [post]
func (h *RoleHandler) CopyRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req CopyRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	res, err := h.roleService.CopyRole(r.Context(), id, req.Name, req.Code)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to copy role")
		return
	}

	responder.OK(w, r, res)
}

// SetRoleMenus handles updating role menu permissions.
// @Summary Set role menu permissions
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body SetRoleMenusRequest true "Menu permissions"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/menus [post]
func (h *RoleHandler) SetRoleMenus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req SetRoleMenusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	if err := h.roleService.SetRoleMenus(r.Context(), id, req.MenuIDs); err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to set role menus")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Menu permissions updated"})
}

// SetRolePermissions handles updating role permission codes.
// @Summary Set role permission codes
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body SetRolePermissionsRequest true "Permission codes"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/permissions [post]
func (h *RoleHandler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req SetRolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	if err := h.roleService.SetRolePermissions(r.Context(), id, req.PermissionCodes); err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to set role permissions")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Permission codes updated"})
}

// UpsertRoleDataScopeRule handles creating or updating a role p2 data scope rule.
// @Summary Upsert role data scope rule
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body RoleDataScopeRuleInput true "Data scope rule"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/data-scope-rules [put]
func (h *RoleHandler) UpsertRoleDataScopeRule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req RoleDataScopeRuleInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	if err := h.roleService.UpsertRoleDataScopeRule(r.Context(), id, req); err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to upsert role data scope rule")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Data scope rule upserted"})
}

// ListRoleDataScopeRules handles listing role p2 data scope rules.
// @Summary List role data scope rules
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {array} RoleDataScopeRule
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/data-scope-rules [get]
func (h *RoleHandler) ListRoleDataScopeRules(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	rules, err := h.roleService.ListRoleDataScopeRules(r.Context(), id)
	if err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to list role data scope rules")
		return
	}

	responder.OK(w, r, rules)
}

// DeleteRoleDataScopeRule handles deleting role p2 data scope rule by domain + resourceKey.
// @Summary Delete role data scope rule
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body DeleteRoleDataScopeRuleRequest true "Delete rule input"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/data-scope-rules [delete]
func (h *RoleHandler) DeleteRoleDataScopeRule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	var req DeleteRoleDataScopeRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	if err := h.roleService.DeleteRoleDataScopeRule(r.Context(), id, req.Domain, req.ResourceKey); err != nil {
		if handleRoleServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to delete role data scope rule")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Data scope rule deleted"})
}

// ListRoleUsers handles listing users in a role.
// @Summary List users in a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/users [get]
func (h *RoleHandler) ListRoleUsers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := h.roleService.ListRoleUsers(r.Context(), id, page, pageSize)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to list role users")
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	pagination := &responder.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	responder.WriteList(w, r, http.StatusOK, users, pagination)
}

// GetRoleTree handles getting role tree structure.
// @Summary Get role tree structure
// @Tags Roles
// @Accept json
// @Produce json
// @Success 200 {object} []RoleTreeNode
// @Failure 500 {object} responder.Error
// @Router /roles/tree [get]
func (h *RoleHandler) GetRoleTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.roleService.GetRoleTree(r.Context())
	if err != nil {
		responder.InternalServerError(w, r, "Failed to get role tree")
		return
	}

	responder.OK(w, r, tree)
}

// GetRoleChildren handles getting child roles.
// @Summary Get child roles
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param recursive query bool false "Include all descendants" default(false)
// @Success 200 {object} []ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/children [get]
func (h *RoleHandler) GetRoleChildren(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	recursive := r.URL.Query().Get("recursive") == "true"

	children, err := h.roleService.GetRoleChildren(r.Context(), id, recursive)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to get role children")
		return
	}

	responder.OK(w, r, children)
}

// GetRoleParents handles getting parent role chain.
// @Summary Get parent role chain
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} []ent.Role
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /roles/{id}/parents [get]
func (h *RoleHandler) GetRoleParents(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid role ID")
		return
	}

	parents, err := h.roleService.GetRoleParents(r.Context(), id)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to get role parents")
		return
	}

	responder.OK(w, r, parents)
}

// SetupRoleRoutes registers role management routes.
func SetupRoleRoutes(r chi.Router, h *RoleHandler, jwtMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/roles", func(r chi.Router) {
		r.Use(jwtMiddleware)

		r.Get("/", h.ListRoles)
		r.Post("/", h.CreateRole)
		r.Get("/tree", h.GetRoleTree) // 树形结构查询

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetRole)
			r.Put("/", h.UpdateRole)
			r.Delete("/", h.DeleteRole)

			r.Post("/copy", h.CopyRole)
			r.Post("/menus", h.SetRoleMenus)
			r.Post("/permissions", h.SetRolePermissions)
			r.Put("/data-scope-rules", h.UpsertRoleDataScopeRule)
			r.Get("/data-scope-rules", h.ListRoleDataScopeRules)
			r.Delete("/data-scope-rules", h.DeleteRoleDataScopeRule)
			r.Get("/users", h.ListRoleUsers)
			r.Get("/children", h.GetRoleChildren) // 获取子角色
			r.Get("/parents", h.GetRoleParents)   // 获取父角色链
		})
	})
}
