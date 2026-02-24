package mcp

import "errors"

// MCPServerInfo contains information about an MCP server
type MCPServerInfo struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      string          `json:"version"`
	Status       string          `json:"status"` // running, stopped, error
	Transports   []string        `json:"transports"`
	HTTPEndpoint string          `json:"httpEndpoint,omitempty"`
	StdioCommand string          `json:"stdioCommand,omitempty"`
	Capabilities MCPCapabilities `json:"capabilities"`
	Examples     *MCPExamples    `json:"examples,omitempty"`
}

// MCPCapabilities describes what an MCP server can do
type MCPCapabilities struct {
	Resources []string `json:"resources"`
	Tools     []string `json:"tools"`
}

// MCPExamples provides usage examples
type MCPExamples struct {
	StdioUsage          []string       `json:"stdioUsage,omitempty"`
	HTTPUsage           []string       `json:"httpUsage,omitempty"`
	ClaudeDesktopConfig map[string]any `json:"claudeDesktopConfig,omitempty"`
}

// ServersResponse is the response for listing servers
type ServersResponse struct {
	Servers []MCPServerInfo `json:"servers"`
	Count   int             `json:"count"`
}

// ServerHealth contains health information for a server
type ServerHealth struct {
	Status     string            `json:"status"`
	Transports map[string]string `json:"transports,omitempty"`
}

// HealthResponse is the response for health check
type HealthResponse struct {
	Status  string                  `json:"status"`
	Servers map[string]ServerHealth `json:"servers"`
}

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Errors
var (
	ErrServerNotFound = errors.New("MCP server not found")
)
