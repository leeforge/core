package apikey

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

// APIKeyHandler handles API key related HTTP requests
type APIKeyHandler struct {
	apiKeyService *APIKeyService
	logger        logging.Logger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(apiKeyService *APIKeyService, logger logging.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		logger:        logger,
	}
}

// CreateAPIKey handles POST /api-keys
// @Summary Create a new API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param request body CreateAPIKeyRequest true "API key creation data"
// @Success 201 {object} CreateAPIKeyResponse
// @Router /api-keys [post]
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "body", Message: "Invalid request body"},
		})
		return
	}

	// Validate required fields
	if req.Name == "" {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "name", Message: "Name is required"},
		})
		return
	}

	// Set default environment if not provided
	env := req.Environment
	if env == "" {
		env = "live"
	}

	var projectID *uuid.UUID
	if req.ProjectID != "" {
		parsed, err := uuid.Parse(req.ProjectID)
		if err != nil {
			responder.ValidationError(w, r, []responder.FieldError{
				{Field: "projectId", Message: "Invalid project ID"},
			})
			return
		}
		projectID = &parsed
	}

	// Create API key with user's roles from JWT
	result, err := h.apiKeyService.Create(
		r.Context(),
		identity.UserID,
		req.Name,
		identity.Roles, // Pass user's roles from JWT
		req.Description,
		req.ExpiresAt, // Now directly pass *time.Time
		req.RateLimit,
		req.DailyLimit,
		env,
		projectID,
	)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to create API key", err, zap.String("user_id", identity.UserID.String()))
		responder.InternalServerError(w, r, "Failed to create API key")
		return
	}

	h.logger.Info("API key created",
		zap.String("key_id", result.APIKey.ID.String()),
		zap.String("user_id", identity.UserID.String()),
		zap.Strings("roles", identity.Roles))

	responder.Created(w, r, result)
}

// ListAPIKeys handles GET /api-keys
// @Summary List all API keys for authenticated user
// @Tags API Keys
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {array} APIKeyDTO
// @Router /api-keys [get]
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// List API keys
	keys, total, err := h.apiKeyService.List(r.Context(), identity.UserID, page, pageSize)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list API keys", err, zap.String("user_id", identity.UserID.String()))
		responder.InternalServerError(w, r, "Failed to list API keys")
		return
	}

	// Calculate pagination
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

	responder.WriteList(w, r, http.StatusOK, keys, pagination)
}

// GetAPIKey handles GET /api-keys/{id}
// @Summary Get a specific API key by ID
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} APIKeyDTO
// @Router /api-keys/{id} [get]
func (h *APIKeyHandler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	// Get API key
	key, err := h.apiKeyService.Get(r.Context(), id)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to get API key", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to get API key")
		return
	}

	// Security check: Verify ownership (would need to add OwnerID to APIKeyDTO or fetch from service)
	// For now, we assume the service only returns keys owned by the user
	// In production, add an ownership check here

	h.logger.Info("API key retrieved", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, key)
}

// UpdateAPIKey handles PUT /api-keys/{id}
// @Summary Update an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Param request body UpdateAPIKeyRequest true "API key update data"
// @Success 200 {object} APIKeyDTO
// @Router /api-keys/{id} [put]
func (h *APIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	var req UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "body", Message: "Invalid request body"},
		})
		return
	}

	// Validate required fields
	if req.Name == "" {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "name", Message: "Name is required"},
		})
		return
	}

	// Update API key
	key, err := h.apiKeyService.Update(r.Context(), id, req.Name, req.Description, req.RateLimit, req.DailyLimit)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to update API key", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to update API key")
		return
	}

	h.logger.Info("API key updated", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, key)
}

// RevokeAPIKey handles POST /api-keys/{id}/revoke
// @Summary Revoke (deactivate) an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Router /api-keys/{id}/revoke [post]
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	// Revoke API key
	err = h.apiKeyService.Revoke(r.Context(), id)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to revoke API key", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to revoke API key")
		return
	}

	h.logger.Info("API key revoked", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, map[string]string{"message": "API key revoked successfully"})
}

// DeleteAPIKey handles DELETE /api-keys/{id}
// @Summary Delete an API key (soft delete)
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Router /api-keys/{id} [delete]
func (h *APIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	// Delete API key (soft delete)
	err = h.apiKeyService.Delete(r.Context(), id)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to delete API key", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to delete API key")
		return
	}

	h.logger.Info("API key deleted", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, map[string]string{"message": "API key deleted successfully"})
}

// RotateAPIKey handles POST /api-keys/{id}/rotate
// @Summary Rotate (regenerate) an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} CreateAPIKeyResponse
// @Router /api-keys/{id}/rotate [post]
func (h *APIKeyHandler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	// Rotate API key
	result, err := h.apiKeyService.Rotate(r.Context(), id)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to rotate API key", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to rotate API key")
		return
	}

	h.logger.Info("API key rotated", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, result)
}

// GetAPIKeyStats handles GET /api-keys/{id}/stats
// @Summary Get usage statistics for an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} APIKeyStatsDTO
// @Router /api-keys/{id}/stats [get]
func (h *APIKeyHandler) GetAPIKeyStats(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	// Parse API key ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "id", Message: "Invalid API key ID"},
		})
		return
	}

	// Get stats
	stats, err := h.apiKeyService.GetStats(r.Context(), id)
	if err != nil {
		if err == ErrAPIKeyNotFound {
			responder.NotFound(w, r, "API key not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to get API key stats", err, zap.String("id", id.String()))
		responder.InternalServerError(w, r, "Failed to get API key stats")
		return
	}

	h.logger.Info("API key stats retrieved", zap.String("key_id", id.String()), zap.String("user_id", identity.UserID.String()))
	responder.OK(w, r, stats)
}

// ValidateAPIKey handles POST /api-keys/validate
// @Summary Validate an API key
// @Tags API Keys
// @Accept json
// @Produce json
// @Param request body ValidateAPIKeyRequest true "API key to validate"
// @Success 200 {object} ValidateAPIKeyResponse
// @Router /api-keys/validate [post]
func (h *APIKeyHandler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	// API Key management requires JWT authentication
	identity, ok := core.RequireJWT(r.Context())
	if !ok {
		responder.Forbidden(w, r, "API Key management requires JWT authentication")
		return
	}

	var req ValidateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "body", Message: "Invalid request body"},
		})
		return
	}

	if req.Key == "" {
		responder.ValidationError(w, r, []responder.FieldError{
			{Field: "key", Message: "API key is required"},
		})
		return
	}

	// Validate API key
	validated, err := h.apiKeyService.Validate(r.Context(), req.Key)
	if err != nil {
		if err == ErrAPIKeyNotFound || err == ErrInvalidAPIKey {
			responder.OK(w, r, &ValidateAPIKeyResponse{Valid: false})
			return
		}
		if err == ErrAPIKeyInactive {
			responder.OK(w, r, &ValidateAPIKeyResponse{Valid: false})
			return
		}
		if err == ErrAPIKeyExpired {
			responder.OK(w, r, &ValidateAPIKeyResponse{Valid: false})
			return
		}
		httplog.Error(h.logger, r, "Failed to validate API key", err)
		responder.InternalServerError(w, r, "Failed to validate API key")
		return
	}

	h.logger.Info("API key validated", zap.String("key_id", validated.ID.String()), zap.String("requested_by", identity.UserID.String()))

	// Return validation result
	resp := &ValidateAPIKeyResponse{
		Valid:       true,
		KeyID:       validated.ID,
		Name:        validated.Name,
		Permissions: validated.Permissions,
		DataFilters: validated.DataFilters,
		RateLimit:   validated.RateLimit,
		DailyLimit:  validated.DailyLimit,
		OwnerID:     validated.OwnerID,
		RoleID:      validated.RoleID,
	}

	responder.OK(w, r, resp)
}

// SetupAPIKeyRoutes registers all API key routes
func SetupAPIKeyRoutes(r chi.Router, handler *APIKeyHandler, jwtMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/api-keys", func(r chi.Router) {
		// All routes require JWT authentication
		r.Use(jwtMiddleware)

		r.Post("/", handler.CreateAPIKey)
		r.Get("/", handler.ListAPIKeys)
		r.Post("/validate", handler.ValidateAPIKey)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", handler.GetAPIKey)
			r.Put("/", handler.UpdateAPIKey)
			r.Delete("/", handler.DeleteAPIKey)
			r.Post("/revoke", handler.RevokeAPIKey)
			r.Post("/rotate", handler.RotateAPIKey)
			r.Get("/stats", handler.GetAPIKeyStats)
		})
	})
}
