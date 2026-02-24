package operationlog

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

type OperationLogHandler struct {
	service *OperationLogService
	logger  logging.Logger
}

func NewOperationLogHandler(service *OperationLogService, logger logging.Logger) *OperationLogHandler {
	return &OperationLogHandler{
		service: service,
		logger:  logger,
	}
}

// List returns a list of operation logs
// @Summary List operation logs
// @Tags Logs
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Param method query string false "HTTP Method"
// @Param status query int false "HTTP Status"
// @Param startDate query string false "Start date (RFC3339)"
// @Param endDate query string false "End date (RFC3339)"
// @Success 200 {object} responder.Response
// @Router /logs/operations [get]
func (h *OperationLogHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	input := &LogQueryInput{
		Page:      page,
		PageSize:  pageSize,
		Method:    r.URL.Query().Get("method"),
		Path:      r.URL.Query().Get("path"),
		ClientIP:  r.URL.Query().Get("ip"),
		RequestID: r.URL.Query().Get("request_id"),
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status, err := strconv.Atoi(statusStr)
		if err == nil {
			input.Status = &status
		}
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
		responder.InternalServerError(w, r, "Failed to query logs")
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

// Delete deletes a single log entry
// @Summary Delete operation log
// @Tags Logs
// @Param id path string true "Log ID"
// @Success 200 {object} responder.Response
// @Router /logs/operations/{id} [delete]
func (h *OperationLogHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// ClearInput defines parameters for clearing logs
type ClearInput struct {
	RetentionDays int `json:"retentionDays"`
}

// Clear deletes old logs
// @Summary Clear old operation logs
// @Tags Logs
// @Accept json
// @Produce json
// @Param request body ClearInput true "Retention settings"
// @Success 200 {object} responder.Response
// @Router /logs/operations/clear [post]
func (h *OperationLogHandler) Clear(w http.ResponseWriter, r *http.Request) {
	var input ClearInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		// Default to 30 days if body is invalid or empty
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
