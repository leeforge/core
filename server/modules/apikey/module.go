package apikey

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type APIKeyModule struct {
	logger  logging.Logger
	handler *APIKeyHandler
}

func NewAPIKeyModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize Service
	svc := NewAPIKeyService(deps.Client, logger)

	// Initialize Handler
	handler := NewAPIKeyHandler(svc, logger)

	return &APIKeyModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *APIKeyModule) Name() string {
	return "apikey"
}

// ValidateAPIKey validates an API key and returns validation info
func (m *APIKeyModule) ValidateAPIKey(ctx context.Context, apiKey string) (*ValidatedAPIKey, error) {
	return m.handler.apiKeyService.Validate(ctx, apiKey)
}

// IncrementUsage increments the usage counter for an API key
func (m *APIKeyModule) IncrementUsage(ctx context.Context, keyID uuid.UUID, ipAddress string) {
	m.handler.apiKeyService.IncrementUsage(ctx, keyID, ipAddress)
}

// RegisterPublicRoutes - apikey module has no public routes
func (m *APIKeyModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for apikey module
}

// RegisterPrivateRoutes registers protected API key endpoints
func (m *APIKeyModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/api-keys", func(r chi.Router) {
		framePerm.Post(r, "/", m.handler.CreateAPIKey, framePerm.Private("Create API key", "apikey.write"))
		framePerm.Get(r, "/", m.handler.ListAPIKeys, framePerm.Private("List API keys", "apikey.read"))
		framePerm.Post(r, "/validate", m.handler.ValidateAPIKey, framePerm.Private("Validate API key", "apikey.validate"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Get(r, "/", m.handler.GetAPIKey, framePerm.Private("Get API key", "apikey.read"))
			framePerm.Put(r, "/", m.handler.UpdateAPIKey, framePerm.Private("Update API key", "apikey.write"))
			framePerm.Delete(r, "/", m.handler.DeleteAPIKey, framePerm.Private("Delete API key", "apikey.delete"))
			framePerm.Post(r, "/revoke", m.handler.RevokeAPIKey, framePerm.Private("Revoke API key", "apikey.write"))
			framePerm.Post(r, "/rotate", m.handler.RotateAPIKey, framePerm.Private("Rotate API key", "apikey.write"))
			framePerm.Get(r, "/stats", m.handler.GetAPIKeyStats, framePerm.Private("Get API key stats", "apikey.read"))
		})
	})
}
