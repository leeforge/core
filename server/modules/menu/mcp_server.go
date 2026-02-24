package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"

	"github.com/leeforge/framework/logging"
)

// MenuOutput is a flat DTO for MCP tool outputs, free of ent Edge cycles.
type MenuOutput struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Component string         `json:"component,omitempty"`
	Redirect  string         `json:"redirect,omitempty"`
	Icon      string         `json:"icon,omitempty"`
	ParentID  uuid.UUID      `json:"parentId,omitempty"`
	Sort      int            `json:"sort"`
	Hidden    bool           `json:"hidden"`
	Affix     bool           `json:"affix"`
	Meta      map[string]any `json:"meta,omitempty"`
	Params    []string       `json:"params,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// MenuTreeOutput is a tree DTO for MCP tool outputs.
type MenuTreeOutput struct {
	MenuOutput
	Children []MenuTreeOutput `json:"children,omitempty"`
}

// BatchMenusResult wraps a slice for MCP output (must be object type).
type BatchMenusResult struct {
	Menus []MenuOutput `json:"menus"`
	Count int          `json:"count"`
}

// MenuTreeResult is a simple metadata output for the tree tool.
// The full tree JSON is returned via TextContent to avoid recursive type cycles.
type MenuTreeResult struct {
	RootCount  int `json:"rootCount"`
	TotalCount int `json:"totalCount"`
}

func toMenuOutput(m *ent.Menu) MenuOutput {
	return MenuOutput{
		ID:        m.ID,
		Name:      m.Name,
		Path:      m.Path,
		Component: m.Component,
		Redirect:  m.Redirect,
		Icon:      m.Icon,
		ParentID:  m.ParentID,
		Sort:      m.Sort,
		Hidden:    m.Hidden,
		Affix:     m.Affix,
		Meta:      m.Meta,
		Params:    m.Params,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toMenuTreeOutput(nodes []*MenuNode) []MenuTreeOutput {
	out := make([]MenuTreeOutput, 0, len(nodes))
	for _, n := range nodes {
		item := MenuTreeOutput{
			MenuOutput: toMenuOutput(n.Menu),
		}
		if len(n.Children) > 0 {
			item.Children = toMenuTreeOutput(n.Children)
		}
		out = append(out, item)
	}
	return out
}

// MCPServer wraps the menu service as an MCP server
type MCPServer struct {
	server  *mcp.Server
	service *MenuService
	logger  logging.Logger
}

// NewMCPServer creates a standalone MCP server for menu operations
func NewMCPServer(service *MenuService, logger logging.Logger) (*MCPServer, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "leeforge-menu-server",
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

// RegisterOn registers all menu tools and resources onto an external shared server.
func (s *MCPServer) RegisterOn(server *mcp.Server) error {
	if err := s.registerResources(server); err != nil {
		return fmt.Errorf("failed to register menu resources: %w", err)
	}
	if err := s.registerTools(server); err != nil {
		return fmt.Errorf("failed to register menu tools: %w", err)
	}
	return nil
}

// registerResources registers menu resources onto the given server
func (s *MCPServer) registerResources(server *mcp.Server) error {
	server.AddResource(
		&mcp.Resource{
			URI:         "menu://tree",
			Name:        "Menu Tree",
			Description: "Complete menu tree structure",
			MIMEType:    "application/json",
		},
		s.handleMenuTreeResource,
	)

	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "menu://item/{id}",
			Name:        "Menu Item",
			Description: "Single menu item by ID",
			MIMEType:    "application/json",
		},
		s.handleMenuItemResource,
	)

	return nil
}

// registerTools registers menu tools onto the given server
func (s *MCPServer) registerTools(server *mcp.Server) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_menu",
		Description: "Create a single menu item. Fields: name (required), path (required), component, redirect, icon, parentId (UUID), sort (int), hidden (bool), affix (bool), meta (object), params (string array).",
	}, s.handleCreateMenu)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "batch_create_menus",
		Description: "Create multiple menus at once. Supports parent-child references within the batch via tempId/tempParentId. Menus without tempParentId are created first, then menus referencing tempParentId are created with resolved real IDs.",
	}, s.handleBatchCreateMenus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_menu_tree",
		Description: "Get the complete menu tree. Returns all menus in a hierarchical structure.",
	}, s.handleGetMenuTree)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_menu",
		Description: "Delete a menu and all its sub-menus recursively.",
	}, s.handleDeleteMenu)

	return nil
}

// --- Resource Handlers ---

func (s *MCPServer) handleMenuTreeResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	menus, err := s.service.GetAllMenus(ctx)
	if err != nil {
		s.logger.Error("Failed to get menus", zap.Error(err))
		return nil, fmt.Errorf("failed to get menus: %w", err)
	}

	tree := s.service.buildTree(menus, uuid.Nil)

	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal menu tree: %w", err)
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

func (s *MCPServer) handleMenuItemResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	var idStr string
	_, err := fmt.Sscanf(req.Params.URI, "menu://item/%s", &idStr)
	if err != nil || idStr == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	m, err := s.service.GetMenuByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get menu", zap.String("id", idStr), zap.Error(err))
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal menu: %w", err)
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

// --- Tool Input/Output Types ---

type CreateMenuInput struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Component string         `json:"component,omitempty"`
	Redirect  string         `json:"redirect,omitempty"`
	Icon      string         `json:"icon,omitempty"`
	ParentID  string         `json:"parentId,omitempty"`
	Sort      int            `json:"sort"`
	Hidden    bool           `json:"hidden"`
	Affix     bool           `json:"affix"`
	Meta      map[string]any `json:"meta,omitempty"`
	Params    []string       `json:"params,omitempty"`
}

type BatchCreateMenusInput struct {
	Menus []BatchMenuInputMCP `json:"menus"`
}

type BatchMenuInputMCP struct {
	TempID       string         `json:"tempId,omitempty"`
	TempParentID string         `json:"tempParentId,omitempty"`
	Name         string         `json:"name"`
	Path         string         `json:"path"`
	Component    string         `json:"component,omitempty"`
	Redirect     string         `json:"redirect,omitempty"`
	Icon         string         `json:"icon,omitempty"`
	ParentID     string         `json:"parentId,omitempty"`
	Sort         int            `json:"sort"`
	Hidden       bool           `json:"hidden"`
	Affix        bool           `json:"affix"`
	Meta         map[string]any `json:"meta,omitempty"`
	Params       []string       `json:"params,omitempty"`
}

type DeleteMenuInput struct {
	ID string `json:"id"`
}

type GetMenuTreeInput struct{}

// --- Tool Handlers ---

func (s *MCPServer) handleCreateMenu(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateMenuInput,
) (*mcp.CallToolResult, MenuOutput, error) {
	if input.Name == "" || input.Path == "" {
		return nil, MenuOutput{}, fmt.Errorf("name and path are required")
	}

	menuInput := &ent.Menu{
		Name:      input.Name,
		Path:      input.Path,
		Component: input.Component,
		Redirect:  input.Redirect,
		Icon:      input.Icon,
		Sort:      input.Sort,
		Hidden:    input.Hidden,
		Affix:     input.Affix,
		Meta:      input.Meta,
		Params:    input.Params,
	}

	if input.ParentID != "" {
		pid, err := uuid.Parse(input.ParentID)
		if err != nil {
			return nil, MenuOutput{}, fmt.Errorf("invalid parentId: %w", err)
		}
		menuInput.ParentID = pid
	}

	m, err := s.service.CreateMenu(ctx, menuInput)
	if err != nil {
		s.logger.Error("Failed to create menu", zap.Error(err))
		return nil, MenuOutput{}, fmt.Errorf("failed to create menu: %w", err)
	}

	out := toMenuOutput(m)
	data, _ := json.MarshalIndent(out, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Menu '%s' created successfully (ID: %s)\n%s", m.Name, m.ID, string(data)),
			},
		},
	}, out, nil
}

func (s *MCPServer) handleBatchCreateMenus(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BatchCreateMenusInput,
) (*mcp.CallToolResult, BatchMenusResult, error) {
	if len(input.Menus) == 0 {
		return nil, BatchMenusResult{}, fmt.Errorf("menus array is required and must not be empty")
	}

	// Convert MCP input to service input
	var batchInputs []BatchMenuInput
	for _, m := range input.Menus {
		bi := BatchMenuInput{
			TempID:       m.TempID,
			TempParentID: m.TempParentID,
			Name:         m.Name,
			Path:         m.Path,
			Component:    m.Component,
			Redirect:     m.Redirect,
			Icon:         m.Icon,
			Sort:         m.Sort,
			Hidden:       m.Hidden,
			Affix:        m.Affix,
			Meta:         m.Meta,
			Params:       m.Params,
		}
		if m.ParentID != "" {
			pid, err := uuid.Parse(m.ParentID)
			if err != nil {
				return nil, BatchMenusResult{}, fmt.Errorf("invalid parentId for menu '%s': %w", m.Name, err)
			}
			bi.ParentID = &pid
		}
		batchInputs = append(batchInputs, bi)
	}

	results, err := s.service.BatchCreateMenus(ctx, batchInputs)
	if err != nil {
		s.logger.Error("Failed to batch create menus", zap.Error(err))
		return nil, BatchMenusResult{}, fmt.Errorf("failed to batch create menus: %w", err)
	}

	out := make([]MenuOutput, 0, len(results))
	for _, r := range results {
		out = append(out, toMenuOutput(r))
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	result := BatchMenusResult{Menus: out, Count: len(out)}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully created %d menus\n%s", len(results), string(data)),
			},
		},
	}, result, nil
}

func (s *MCPServer) handleGetMenuTree(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetMenuTreeInput,
) (*mcp.CallToolResult, MenuTreeResult, error) {
	menus, err := s.service.GetAllMenus(ctx)
	if err != nil {
		s.logger.Error("Failed to get menus", zap.Error(err))
		return nil, MenuTreeResult{}, fmt.Errorf("failed to get menus: %w", err)
	}

	tree := s.service.buildTree(menus, uuid.Nil)
	out := toMenuTreeOutput(tree)

	data, _ := json.MarshalIndent(out, "", "  ")
	result := MenuTreeResult{RootCount: len(out), TotalCount: len(menus)}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Menu tree with %d root items\n%s", len(out), string(data)),
			},
		},
	}, result, nil
}

func (s *MCPServer) handleDeleteMenu(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DeleteMenuInput,
) (*mcp.CallToolResult, map[string]string, error) {
	if input.ID == "" {
		return nil, nil, fmt.Errorf("id is required")
	}

	id, err := uuid.Parse(input.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid id: %w", err)
	}

	if err := s.service.DeleteMenu(ctx, id); err != nil {
		s.logger.Error("Failed to delete menu", zap.String("id", input.ID), zap.Error(err))
		return nil, nil, fmt.Errorf("failed to delete menu: %w", err)
	}

	result := map[string]string{"message": fmt.Sprintf("Menu %s and its sub-menus deleted successfully", input.ID)}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: result["message"],
			},
		},
	}, result, nil
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}

// Run starts the MCP server using stdio transport
func (s *MCPServer) Run(ctx context.Context) error {
	s.logger.Info("Starting Menu MCP server")

	transport := &mcp.StdioTransport{}
	if err := s.server.Run(ctx, transport); err != nil {
		s.logger.Error("MCP server error", zap.Error(err))
		return err
	}

	return nil
}
