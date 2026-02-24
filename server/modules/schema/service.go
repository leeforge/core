package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"
)

// Service provides schema-related business logic
type Service struct {
	logger     logging.Logger
	modulesDir string
}

// NewService creates a new schema service
func NewService(logger logging.Logger, modulesDir string) *Service {
	return &Service{
		logger:     logger,
		modulesDir: modulesDir,
	}
}

// ListSchemas returns a list of all available schemas
func (s *Service) ListSchemas() (*SchemaListResponse, error) {
	entries, err := os.ReadDir(s.modulesDir)
	if err != nil {
		s.logger.Error("Failed to read modules directory", zap.Error(err))
		return nil, fmt.Errorf("failed to read modules directory: %w", err)
	}

	var schemas []SchemaInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		schemaPath := filepath.Join(s.modulesDir, moduleName, "schema.json")

		// Check if schema.json exists
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			s.logger.Debug("Schema file not found for module",
				zap.String("module", moduleName),
				zap.String("path", schemaPath))
			continue
		}

		// Read basic info from schema
		data, err := os.ReadFile(schemaPath)
		if err != nil {
			s.logger.Warn("Failed to read schema file",
				zap.String("module", moduleName),
				zap.Error(err))
			continue
		}

		var schemaDetail SchemaDetailResponse
		if err := json.Unmarshal(data, &schemaDetail); err != nil {
			s.logger.Warn("Failed to parse schema file",
				zap.String("module", moduleName),
				zap.Error(err))
			continue
		}

		schemas = append(schemas, SchemaInfo{
			Name:        schemaDetail.Name,
			Description: schemaDetail.Description,
			Module:      moduleName,
		})
	}

	return &SchemaListResponse{
		Schemas: schemas,
	}, nil
}

// GetSchemaByModule returns the schema details for a specific module
func (s *Service) GetSchemaByModule(moduleName string) (*SchemaDetailResponse, error) {
	// Sanitize module name to prevent path traversal
	moduleName = strings.TrimSpace(moduleName)
	if strings.Contains(moduleName, "..") || strings.Contains(moduleName, "/") {
		return nil, fmt.Errorf("invalid module name: %s", moduleName)
	}

	schemaPath := filepath.Join(s.modulesDir, moduleName, "schema.json")

	// Check if schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		s.logger.Warn("Schema file not found",
			zap.String("module", moduleName),
			zap.String("path", schemaPath))
		return nil, fmt.Errorf("schema not found for module: %s", moduleName)
	}

	// Read schema file
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		s.logger.Error("Failed to read schema file",
			zap.String("module", moduleName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse schema
	var schema SchemaDetailResponse
	if err := json.Unmarshal(data, &schema); err != nil {
		s.logger.Error("Failed to parse schema file",
			zap.String("module", moduleName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to parse schema file: %w", err)
	}

	return &schema, nil
}

// GetSchemaByName returns the schema details by schema name
func (s *Service) GetSchemaByName(schemaName string) (*SchemaDetailResponse, error) {
	// First, list all schemas to find the matching one
	listResp, err := s.ListSchemas()
	if err != nil {
		return nil, err
	}

	for _, schemaInfo := range listResp.Schemas {
		if strings.EqualFold(schemaInfo.Name, schemaName) {
			return s.GetSchemaByModule(schemaInfo.Module)
		}
	}

	return nil, fmt.Errorf("schema not found: %s", schemaName)
}
