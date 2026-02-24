package systemerror

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

type SystemErrorHandler struct {
	service *SystemErrorService
	logger  logging.Logger
}

func NewSystemErrorHandler(service *SystemErrorService, logger logging.Logger) *SystemErrorHandler {
	return &SystemErrorHandler{
		service: service,
		logger:  logger,
	}
}

// List returns a list of system errors
// @Summary List system errors
// @Tags Logs
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Param level query string false "Error Level"
// @Param type query string false "Error Type"
// @Param request_id query string false "Request ID"
// @Success 200 {object} responder.Response
// @Router /logs/errors [get]
func (h *SystemErrorHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	input := &ErrorQueryInput{
		Page:      page,
		PageSize:  pageSize,
		Level:     r.URL.Query().Get("level"),
		Type:      r.URL.Query().Get("type"),
		RequestID: r.URL.Query().Get("request_id"),
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
		responder.InternalServerError(w, r, "Failed to query system errors")
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

// Delete deletes a single error entry
// @Summary Delete system error log
// @Tags Logs
// @Param id path string true "Log ID"
// @Success 200 {object} responder.Response
// @Router /logs/errors/{id} [delete]
func (h *SystemErrorHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Clear deletes old errors
// @Summary Clear old system errors
// @Tags Logs
// @Accept json
// @Produce json
// @Param request body ClearInput true "Retention settings"
// @Success 200 {object} responder.Response
// @Router /logs/errors/clear [post]
func (h *SystemErrorHandler) Clear(w http.ResponseWriter, r *http.Request) {
	var input ClearInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.RetentionDays = 30
	}
	if input.RetentionDays < 1 {
		input.RetentionDays = 30
	}

	count, err := h.service.ClearOldErrors(r.Context(), input.RetentionDays)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to clear errors")
		return
	}

	responder.OK(w, r, map[string]any{
		"deleted_count":  count,
		"retention_days": input.RetentionDays,
	})
}
