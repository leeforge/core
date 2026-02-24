package auditlog

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type AuditLogHandler struct {
	service *AuditLogService
	logger  logging.Logger
}

func NewAuditLogHandler(service *AuditLogService, logger logging.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		service: service,
		logger:  logger,
	}
}

// List returns a list of audit logs.
// @Summary List audit logs
// @Tags Logs
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Param action query string false "Action"
// @Param resource query string false "Resource"
// @Param resource_id query string false "Resource ID"
// @Param user_id query string false "User ID"
// @Param ip query string false "IP address"
// @Param startDate query string false "Start date (RFC3339)"
// @Param endDate query string false "End date (RFC3339)"
// @Success 200 {object} responder.Response
// @Router /logs/audits [get]
func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	input := &LogQueryInput{
		Page:       page,
		PageSize:   pageSize,
		Action:     r.URL.Query().Get("action"),
		Resource:   r.URL.Query().Get("resource"),
		ResourceID: r.URL.Query().Get("resource_id"),
		IPAddress:  r.URL.Query().Get("ip"),
	}

	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		uid, err := uuid.Parse(userIDStr)
		if err == nil {
			input.UserID = &uid
		}
	}
	if startStr := r.URL.Query().Get("startDate"); startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err == nil {
			input.StartDate = &t
		}
	}
	if endStr := r.URL.Query().Get("endDate"); endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err == nil {
			input.EndDate = &t
		}
	}

	logs, total, err := h.service.List(r.Context(), input)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to query audit logs")
		return
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	responder.OK(w, r, logs, responder.WithPagination(&responder.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}))
}

// Delete deletes a single audit log entry.
// @Summary Delete audit log
// @Tags Logs
// @Param id path string true "Log ID"
// @Success 200 {object} responder.Response
// @Router /logs/audits/{id} [delete]
func (h *AuditLogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid log ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		responder.InternalServerError(w, r, "Failed to delete log")
		return
	}

	responder.OK(w, r, nil)
}

// ClearInput defines parameters for clearing logs.
type ClearInput struct {
	RetentionDays int `json:"retentionDays"`
}

// Clear deletes old audit logs.
// @Summary Clear old audit logs
// @Tags Logs
// @Accept json
// @Produce json
// @Param request body ClearInput true "Retention settings"
// @Success 200 {object} responder.Response
// @Router /logs/audits/clear [post]
func (h *AuditLogHandler) Clear(w http.ResponseWriter, r *http.Request) {
	var input ClearInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.RetentionDays = 30
	}
	if input.RetentionDays < 1 {
		input.RetentionDays = 30
	}

	count, err := h.service.ClearOldLogs(r.Context(), input.RetentionDays)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to clear logs")
		return
	}

	responder.OK(w, r, map[string]any{
		"deleted_count":  count,
		"retention_days": input.RetentionDays,
	})
}
