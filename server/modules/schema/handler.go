package schema

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

// Handler handles HTTP requests for schema operations
type Handler struct {
	service *Service
	logger  logging.Logger
}

// NewHandler creates a new schema handler
func NewHandler(service *Service, logger logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// ListSchemas handles GET /schemas
// @Summary List all schemas
// @Description Get a list of all available schemas
// @Tags Schema
// @Produce json
// @Success 200 {object} SchemaListResponse
// @Failure 500 {string} string "Internal server error"
// @Router /schemas [get]
func (h *Handler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	schemas, err := h.service.ListSchemas()
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list schemas", err)
		responder.InternalServerError(w, r, "Failed to list schemas")
		return
	}

	responder.OK(w, r, schemas)
}

// GetSchemaByModule handles GET /schemas/module/:module
// @Summary Get schema by module name
// @Description Get detailed schema information for a specific module
// @Tags Schema
// @Produce json
// @Param module path string true "Module name"
// @Success 200 {object} SchemaDetailResponse
// @Failure 400 {string} string "Bad request"
// @Failure 404 {string} string "Schema not found"
// @Failure 500 {string} string "Internal server error"
// @Router /schemas/module/{module} [get]
func (h *Handler) GetSchemaByModule(w http.ResponseWriter, r *http.Request) {
	moduleName := chi.URLParam(r, "module")
	if moduleName == "" {
		responder.BadRequest(w, r, "Module name is required")
		return
	}

	schema, err := h.service.GetSchemaByModule(moduleName)
	if err != nil {
		if errors.Is(err, errors.New("schema not found for module: "+moduleName)) {
			responder.NotFound(w, r, "Schema not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to get schema", err,
			zap.String("module", moduleName),
		)
		responder.InternalServerError(w, r, "Failed to get schema")
		return
	}

	responder.OK(w, r, schema)
}

// GetSchemaByName handles GET /schemas/:name
// @Summary Get schema by schema name
// @Description Get detailed schema information by schema name (e.g., User, Role, Menu)
// @Tags Schema
// @Produce json
// @Param name path string true "Schema name"
// @Success 200 {object} SchemaDetailResponse
// @Failure 400 {string} string "Bad request"
// @Failure 404 {string} string "Schema not found"
// @Failure 500 {string} string "Internal server error"
// @Router /schemas/{name} [get]
func (h *Handler) GetSchemaByName(w http.ResponseWriter, r *http.Request) {
	schemaName := chi.URLParam(r, "name")
	if schemaName == "" {
		responder.BadRequest(w, r, "Schema name is required")
		return
	}

	schema, err := h.service.GetSchemaByName(schemaName)
	if err != nil {
		if errors.Is(err, errors.New("schema not found: "+schemaName)) {
			responder.NotFound(w, r, "Schema not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to get schema", err,
			zap.String("name", schemaName),
		)
		responder.InternalServerError(w, r, "Failed to get schema")
		return
	}

	responder.OK(w, r, schema)
}
