package permission

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

var (
	validScopes = map[string]struct{}{"api": {}, "ui": {}, "data": {}}
	validStatus = map[string]struct{}{"active": {}, "deprecated": {}}
)

type PermissionHandler struct {
	service *PermissionService
	logger  logging.Logger
}

func NewPermissionHandler(service *PermissionService, logger logging.Logger) *PermissionHandler {
	return &PermissionHandler{
		service: service,
		logger:  logger,
	}
}

// ListPermissions handles listing permission codes.
// @Summary List permissions
// @Tags Permissions
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(50)
// @Param scope query string false "Scope filter (api/ui/data)"
// @Param status query string false "Status filter (active/deprecated)"
// @Param q query string false "Search keyword"
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /permissions [get]
func (h *PermissionHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	if scope != "" {
		if _, ok := validScopes[scope]; !ok {
			responder.BadRequest(w, r, "Invalid scope")
			return
		}
	}

	if status != "" {
		if _, ok := validStatus[status]; !ok {
			responder.BadRequest(w, r, "Invalid status")
			return
		}
	}

	permissions, total, err := h.service.ListPermissions(r.Context(), ListPermissionsOptions{
		Page:     page,
		PageSize: pageSize,
		Scope:    scope,
		Status:   status,
		Keyword:  keyword,
	})
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list permissions", err)
		responder.InternalServerError(w, r, "Failed to list permissions")
		return
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	pagination := &responder.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	responder.WriteList(w, r, http.StatusOK, permissions, pagination)
}

// SyncEndpoints synchronizes all API endpoints from code to database.
// @Summary Sync permissions
// @Description Scan all modules and sync API endpoints to database
// @Tags Permissions
// @Accept json
// @Produce json
// @Success 200 {object} SyncEndpointsResult
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /permissions/sync [post]
func (h *PermissionHandler) SyncEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := h.service.SyncEndpoints(ctx)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to sync endpoints", err)
		responder.InternalServerError(w, r, "Failed to sync endpoints")
		return
	}

	responder.OK(w, r, result)
}

// GetSyncStatus gets the status of endpoint synchronization.
// @Summary Get sync status
// @Description Get the status of API endpoint synchronization
// @Tags Permissions
// @Accept json
// @Produce json
// @Success 200 {object} SyncStatus
// @Failure 500 {object} responder.Error
// @Router /permissions/sync-status [get]
func (h *PermissionHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status, err := h.service.GetSyncStatus(ctx)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to get sync status", err)
		responder.InternalServerError(w, r, "Failed to get sync status")
		return
	}

	responder.OK(w, r, status)
}
