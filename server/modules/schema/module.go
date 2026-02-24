package schema

import (
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// SchemaModule represents the schema module
type SchemaModule struct {
	handler *Handler
	service *Service
	logger  logging.Logger
}

// NewSchemaModule creates a new schema module
func NewSchemaModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Get the modules directory path
	// Assuming the binary is run from the project root
	modulesDir := filepath.Join("internal", "modules")

	// Create service
	service := NewService(logger, modulesDir)

	// Create handler
	handler := NewHandler(service, logger)

	return &SchemaModule{
		handler: handler,
		service: service,
		logger:  logger,
	}
}

// Name returns the module name
func (m *SchemaModule) Name() string {
	return "schema"
}

// RegisterPublicRoutes registers public schema endpoints (frontend needs to read schemas)
func (m *SchemaModule) RegisterPublicRoutes(r chi.Router) {
	r.Route("/schemas", func(r chi.Router) {
		// List all schemas
		framePerm.Get(r, "/", m.handler.ListSchemas, framePerm.Public("List schemas", "schema.read"))

		// Get schema by module name
		framePerm.Get(r, "/module/{module}", m.handler.GetSchemaByModule, framePerm.Public("Get schema by module", "schema.read"))

		// Get schema by schema name
		framePerm.Get(r, "/{name}", m.handler.GetSchemaByName, framePerm.Public("Get schema by name", "schema.read"))
	})

	m.logger.Info("Schema routes registered")
}

// RegisterPrivateRoutes - schema module has no private routes
func (m *SchemaModule) RegisterPrivateRoutes(r chi.Router) {
	// No private routes for schema module (all endpoints are public)
}
