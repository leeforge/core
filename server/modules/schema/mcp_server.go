package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"
)

// MCPServer wraps the schema service as an MCP server
type MCPServer struct {
	server  *mcp.Server
	service *Service
	logger  logging.Logger
}

// NewMCPServer creates a standalone MCP server for schema operations
func NewMCPServer(service *Service, logger logging.Logger) (*MCPServer, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "leeforge-schema-server",
		Version: "1.0.0",
	}, &mcp.ServerOptions{})

	mcpServer := &MCPServer{
		server:  server,
		service: service,
		logger:  logger,
	}

	if err := mcpServer.registerResources(server); err != nil {
		return nil, fmt.Errorf("failed to register resources: %w", err)
	}

	if err := mcpServer.registerTools(server); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return mcpServer, nil
}

// RegisterOn registers all schema tools and resources onto an external shared server.
func (s *MCPServer) RegisterOn(server *mcp.Server) error {
	if err := s.registerResources(server); err != nil {
		return fmt.Errorf("failed to register schema resources: %w", err)
	}
	if err := s.registerTools(server); err != nil {
		return fmt.Errorf("failed to register schema tools: %w", err)
	}
	return nil
}

// registerResources registers schema resources onto the given server
func (s *MCPServer) registerResources(server *mcp.Server) error {
	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "schema://module/{module}",
			Name:        "Schema by Module",
			Description: "Access schema definition by module name",
			MIMEType:    "application/json",
		},
		s.handleSchemaResourceByModule,
	)

	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "schema://name/{name}",
			Name:        "Schema by Name",
			Description: "Access schema definition by schema name",
			MIMEType:    "application/json",
		},
		s.handleSchemaResourceByName,
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "schema://list",
			Name:        "Schema List",
			Description: "List of all available schemas",
			MIMEType:    "application/json",
		},
		s.handleSchemaListResource,
	)

	return nil
}

// registerTools registers schema tools onto the given server
func (s *MCPServer) registerTools(server *mcp.Server) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_schemas",
		Description: "List all available schemas in the system",
	}, s.handleListSchemasTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_schema_by_module",
		Description: "Get schema definition by module name",
	}, s.handleGetSchemaByModuleTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_schema_by_name",
		Description: "Get schema definition by schema name",
	}, s.handleGetSchemaByNameTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_schema",
		Description: "Validate if a module has a valid schema.json file",
	}, s.handleValidateSchemaTool)

	return nil
}

// Resource handlers

func (s *MCPServer) handleSchemaListResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	schemas, err := s.service.ListSchemas()
	if err != nil {
		s.logger.Error("Failed to list schemas", zap.Error(err))
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	data, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schemas: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func (s *MCPServer) handleSchemaResourceByModule(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse URI to extract module name
	// URI format: schema://module/{module}
	var moduleName string
	_, err := fmt.Sscanf(req.Params.URI, "schema://module/%s", &moduleName)
	if err != nil || moduleName == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	schema, err := s.service.GetSchemaByModule(moduleName)
	if err != nil {
		s.logger.Error("Failed to get schema by module",
			zap.String("module", moduleName),
			zap.Error(err))
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func (s *MCPServer) handleSchemaResourceByName(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse URI to extract schema name
	// URI format: schema://name/{name}
	var schemaName string
	_, err := fmt.Sscanf(req.Params.URI, "schema://name/%s", &schemaName)
	if err != nil || schemaName == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	schema, err := s.service.GetSchemaByName(schemaName)
	if err != nil {
		s.logger.Error("Failed to get schema by name",
			zap.String("name", schemaName),
			zap.Error(err))
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

// Tool handlers - Input/Output types

type ListSchemasInput struct{}
type ListSchemasOutput struct {
	Schemas []SchemaInfo `json:"schemas"`
}

type GetSchemaByModuleInput struct {
	Module string `json:"module"`
}

type GetSchemaByNameInput struct {
	Name string `json:"name"`
}

type ValidateSchemaInput struct {
	Module string `json:"module"`
}

type ValidateSchemaOutput struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	Module  string `json:"module"`
}

// Tool handler functions

func (s *MCPServer) handleListSchemasTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListSchemasInput,
) (*mcp.CallToolResult, ListSchemasOutput, error) {
	schemas, err := s.service.ListSchemas()
	if err != nil {
		s.logger.Error("Failed to list schemas", zap.Error(err))
		return nil, ListSchemasOutput{}, fmt.Errorf("failed to list schemas: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Found %d schemas", len(schemas.Schemas)),
			},
		},
	}, ListSchemasOutput{Schemas: schemas.Schemas}, nil
}

func (s *MCPServer) handleGetSchemaByModuleTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetSchemaByModuleInput,
) (*mcp.CallToolResult, *SchemaDetailResponse, error) {
	if input.Module == "" {
		return nil, nil, fmt.Errorf("module name is required")
	}

	schema, err := s.service.GetSchemaByModule(input.Module)
	if err != nil {
		s.logger.Error("Failed to get schema by module",
			zap.String("module", input.Module),
			zap.Error(err))
		return nil, nil, fmt.Errorf("failed to get schema for module %s: %w", input.Module, err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Schema '%s' for module '%s'", schema.Name, input.Module),
			},
		},
	}, schema, nil
}

func (s *MCPServer) handleGetSchemaByNameTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetSchemaByNameInput,
) (*mcp.CallToolResult, *SchemaDetailResponse, error) {
	if input.Name == "" {
		return nil, nil, fmt.Errorf("schema name is required")
	}

	schema, err := s.service.GetSchemaByName(input.Name)
	if err != nil {
		s.logger.Error("Failed to get schema by name",
			zap.String("name", input.Name),
			zap.Error(err))
		return nil, nil, fmt.Errorf("failed to get schema '%s': %w", input.Name, err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Schema '%s' found", schema.Name),
			},
		},
	}, schema, nil
}

func (s *MCPServer) handleValidateSchemaTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ValidateSchemaInput,
) (*mcp.CallToolResult, ValidateSchemaOutput, error) {
	if input.Module == "" {
		return nil, ValidateSchemaOutput{}, fmt.Errorf("module name is required")
	}

	// Try to get schema to validate
	schema, err := s.service.GetSchemaByModule(input.Module)

	output := ValidateSchemaOutput{
		Module: input.Module,
	}

	if err != nil {
		output.Valid = false
		output.Message = fmt.Sprintf("Schema validation failed: %v", err)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("❌ Module '%s' does not have a valid schema", input.Module),
				},
			},
		}, output, nil
	}

	// Basic validation checks
	if schema.Name == "" {
		output.Valid = false
		output.Message = "Schema name is empty"
	} else if len(schema.Properties) == 0 {
		output.Valid = false
		output.Message = "Schema has no properties defined"
	} else {
		output.Valid = true
		output.Message = fmt.Sprintf("Schema '%s' is valid with %d properties", schema.Name, len(schema.Properties))
	}

	statusIcon := "✅"
	if !output.Valid {
		statusIcon = "⚠️"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("%s %s", statusIcon, output.Message),
			},
		},
	}, output, nil
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}

// Run starts the MCP server using stdio transport
func (s *MCPServer) Run(ctx context.Context) error {
	s.logger.Info("Starting Schema MCP server")

	transport := &mcp.StdioTransport{}
	if err := s.server.Run(ctx, transport); err != nil {
		s.logger.Error("MCP server error", zap.Error(err))
		return err
	}

	return nil
}
