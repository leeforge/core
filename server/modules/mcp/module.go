package mcp

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/modules/menu"
	"github.com/leeforge/core/server/modules/schema"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// MCPModule represents the MCP module that manages all MCP servers
type MCPModule struct {
	logger logging.Logger

	// Unified server aggregates all tools/resources from every sub-module.
	// Clients only need to connect to this single server.
	unifiedServer      *mcp.Server
	unifiedHTTPHandler http.Handler

	// Individual servers kept for standalone stdio use (./bin/mcp schema)
	schemaMCP *schema.MCPServer
	menuMCP   *menu.MCPServer
}

// NewMCPModule creates a new MCP module
func NewMCPModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	module := &MCPModule{
		logger: logger,
	}

	// ── Create unified MCP server ──
	unified := mcp.NewServer(&mcp.Implementation{
		Name:    "leeforge-mcp-server",
		Version: "1.0.0",
	}, &mcp.ServerOptions{})
	module.unifiedServer = unified

	// ── Schema ──
	schemaService := schema.NewService(logger, "internal/modules")

	// Standalone server (for ./bin/mcp schema)
	schemaMCP, err := schema.NewMCPServer(schemaService, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Schema MCP server", zap.Error(err))
	}
	module.schemaMCP = schemaMCP

	// Register schema tools/resources onto the unified server
	if err := schemaMCP.RegisterOn(unified); err != nil {
		logger.Fatal("Failed to register schema on unified server", zap.Error(err))
	}

	// ── Menu ──
	menuService := menu.NewMenuService(deps.Client, logger)

	// Standalone server (for ./bin/mcp menu)
	menuMCP, err := menu.NewMCPServer(menuService, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Menu MCP server", zap.Error(err))
	}
	module.menuMCP = menuMCP

	// Register menu tools/resources onto the unified server
	if err := menuMCP.RegisterOn(unified); err != nil {
		logger.Fatal("Failed to register menu on unified server", zap.Error(err))
	}

	// ── Unified HTTP handler (streamable) ──
	module.unifiedHTTPHandler = mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return unified
		},
		&mcp.StreamableHTTPOptions{
			JSONResponse:   true,
			EventStore:     mcp.NewMemoryEventStore(nil),
			SessionTimeout: 30 * time.Minute,
		},
	)

	return module
}

// Name returns the module name
func (m *MCPModule) Name() string {
	return "mcp"
}

// RegisterPublicRoutes - MCP module has no public routes
func (m *MCPModule) RegisterPublicRoutes(r chi.Router) {}

// RegisterPrivateRoutes registers MCP endpoints (requires JWT or API-key authentication)
func (m *MCPModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/mcp", func(r chi.Router) {
		// Discovery: list available tools
		framePerm.Get(r, "/servers", m.listServers, framePerm.Private("List MCP servers", "mcp.read"))
		framePerm.Get(r, "/servers/{name}", m.getServerInfo, framePerm.Private("Get MCP server info", "mcp.read"))
		framePerm.Get(r, "/health", m.healthCheck, framePerm.Private("MCP health check", "mcp.read"))

		// ── Unified Streamable HTTP endpoint ──
		// Clients use POST for JSON-RPC messages, GET for SSE stream, DELETE to close session.
		framePerm.Get(r, "/sse", m.streamableEndpointGET, framePerm.Private("Unified MCP Streamable HTTP (GET)", "mcp.write"))
		framePerm.Post(r, "/sse", m.streamableEndpointPOST, framePerm.Private("Unified MCP Streamable HTTP (POST)", "mcp.write"))
		framePerm.Delete(r, "/sse", m.streamableEndpointDELETE, framePerm.Private("Unified MCP Streamable HTTP (DELETE)", "mcp.write"))
	})

	m.logger.Info("MCP routes registered",
		zap.Strings("endpoints", []string{
			"/api/v1/mcp/sse          (unified - all tools)",
			"/api/v1/mcp/servers",
			"/api/v1/mcp/servers/{name}",
			"/api/v1/mcp/health",
		}))
}

// ── HTTP Handlers ──

// streamableEndpointGET handles GET requests to the unified MCP streamable endpoint.
// @Summary MCP Streamable HTTP (GET)
// @Description Open an SSE stream on an active MCP session. Requires a valid Mcp-Session-Id header obtained from a prior initialize request.
// @Tags MCP
// @Produce text/event-stream
// @Param Mcp-Session-Id header string true "Session ID from initialize response"
// @Param Mcp-Protocol-Version header string false "Protocol version (default: 2025-03-26)"
// @Success 200 {string} string "SSE event stream"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {string} string "Session not found"
// @Router /mcp/sse [get]
func (m *MCPModule) streamableEndpointGET(w http.ResponseWriter, r *http.Request) {
	m.unifiedHTTPHandler.ServeHTTP(w, r)
}

// streamableEndpointPOST handles POST requests to the unified MCP streamable endpoint.
// @Summary MCP Streamable HTTP (POST)
// @Description Send a JSON-RPC message (initialize, notifications/initialized, tools/list, tools/call, etc.) to the MCP server. The first POST must be an initialize request; subsequent POSTs require the Mcp-Session-Id header.
// @Tags MCP
// @Accept json
// @Produce json
// @Param Mcp-Session-Id header string false "Session ID (required after initialize)"
// @Param Mcp-Protocol-Version header string false "Protocol version (default: 2025-03-26)"
// @Param body body object true "JSON-RPC 2.0 request"
// @Success 200 {object} object "JSON-RPC response"
// @Success 202 {string} string "Accepted (for notifications)"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {object} ErrorResponse
// @Router /mcp/sse [post]
func (m *MCPModule) streamableEndpointPOST(w http.ResponseWriter, r *http.Request) {
	m.unifiedHTTPHandler.ServeHTTP(w, r)
}

// streamableEndpointDELETE handles DELETE requests to close an MCP session.
// @Summary MCP Streamable HTTP (DELETE)
// @Description Close an active MCP session. Requires a valid Mcp-Session-Id header.
// @Tags MCP
// @Param Mcp-Session-Id header string true "Session ID to close"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request (missing session ID)"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {string} string "Session not found"
// @Router /mcp/sse [delete]
func (m *MCPModule) streamableEndpointDELETE(w http.ResponseWriter, r *http.Request) {
	m.unifiedHTTPHandler.ServeHTTP(w, r)
}

// listServers returns a list of available MCP servers.
// @Summary List MCP servers
// @Description Returns all available MCP servers with their capabilities, tools, and resources
// @Tags MCP
// @Accept json
// @Produce json
// @Success 200 {object} ServersResponse
// @Failure 401 {object} ErrorResponse
// @Router /mcp/servers [get]
func (m *MCPModule) listServers(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	servers := []MCPServerInfo{
		{
			Name:         "unified",
			Description:  "Unified MCP server — streamable HTTP + stdio, exposes all tools from every module",
			Version:      "1.0.0",
			Status:       "running",
			Transports:   []string{"stdio", "http-streamable"},
			HTTPEndpoint: baseURL + "/api/v1/mcp/sse",
			StdioCommand: "./bin/mcp",
			Capabilities: MCPCapabilities{
				Resources: []string{
					"schema://list",
					"schema://module/{module}",
					"schema://name/{name}",
					"menu://tree",
					"menu://item/{id}",
				},
				Tools: []string{
					"list_schemas",
					"get_schema_by_module",
					"get_schema_by_name",
					"validate_schema",
					"create_menu",
					"batch_create_menus",
					"get_menu_tree",
					"delete_menu",
				},
			},
		},
	}

	respondJSON(w, http.StatusOK, ServersResponse{
		Servers: servers,
		Count:   len(servers),
	})
}

// getServerInfo returns detailed information about a specific MCP server.
// @Summary Get MCP server info
// @Description Returns detailed information about a specific MCP server including usage examples
// @Tags MCP
// @Accept json
// @Produce json
// @Param name path string true "Server name (use 'unified')"
// @Success 200 {object} MCPServerInfo
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /mcp/servers/{name} [get]
func (m *MCPModule) getServerInfo(w http.ResponseWriter, r *http.Request) {
	serverName := chi.URLParam(r, "name")

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	var info *MCPServerInfo

	switch serverName {
	case "unified":
		info = &MCPServerInfo{
			Name:         "unified",
			Description:  "Unified MCP server — streamable HTTP + stdio, exposes all tools from every module",
			Version:      "1.0.0",
			Status:       "running",
			Transports:   []string{"stdio", "http-streamable"},
			HTTPEndpoint: baseURL + "/api/v1/mcp/sse",
			StdioCommand: "./bin/mcp",
			Capabilities: MCPCapabilities{
				Resources: []string{
					"schema://list",
					"schema://module/{module}",
					"schema://name/{name}",
					"menu://tree",
					"menu://item/{id}",
				},
				Tools: []string{
					"list_schemas",
					"get_schema_by_module",
					"get_schema_by_name",
					"validate_schema",
					"create_menu",
					"batch_create_menus",
					"get_menu_tree",
					"delete_menu",
				},
			},
			Examples: &MCPExamples{
				StdioUsage: []string{
					"./bin/mcp",
				},
				HTTPUsage: []string{
					"Configure MCP client with endpoint: " + baseURL + "/api/v1/mcp/sse",
				},
				ClaudeDesktopConfig: map[string]any{
					"stdio": map[string]any{
						"command": "./bin/mcp",
					},
					"http": map[string]any{
						"url": baseURL + "/api/v1/mcp/sse",
					},
				},
			},
		}
	default:
		respondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "Server not found",
			Message: "Use 'unified' to access the combined MCP server",
		})
		return
	}

	respondJSON(w, http.StatusOK, info)
}

// healthCheck returns the health status of all MCP servers.
// @Summary MCP health check
// @Description Returns the health status of all MCP servers and their transport readiness
// @Tags MCP
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 401 {object} ErrorResponse
// @Router /mcp/health [get]
func (m *MCPModule) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := HealthResponse{
		Status: "healthy",
		Servers: map[string]ServerHealth{
			"unified": {
				Status: "available",
				Transports: map[string]string{
					"stdio":           "ready",
					"http-streamable": "ready",
				},
			},
		},
	}

	respondJSON(w, http.StatusOK, health)
}

// ── Public API ──

// GetUnifiedServer returns the unified MCP server
func (m *MCPModule) GetUnifiedServer() *mcp.Server {
	return m.unifiedServer
}

// RunMCPServer runs a specific MCP server by name (for command-line use)
func (m *MCPModule) RunMCPServer(ctx context.Context, serverName string) error {
	m.logger.Info("Starting MCP server", zap.String("server", serverName))

	switch serverName {
	case "unified", "":
		// Run unified server over stdio
		m.logger.Info("Starting unified MCP server (all tools)")
		transport := &mcp.StdioTransport{}
		return m.unifiedServer.Run(ctx, transport)
	case "schema":
		return m.schemaMCP.Run(ctx)
	case "menu":
		return m.menuMCP.Run(ctx)
	default:
		return ErrServerNotFound
	}
}
