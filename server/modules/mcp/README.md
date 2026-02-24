# MCP Module

Model Context Protocol (MCP) module, manages the unified MCP server with **stdio** and **Streamable HTTP** transports.

## Directory Structure

```
internal/modules/mcp/
├── module.go                      # MCP module core
├── module_http_contract_test.go   # HTTP interface contract tests
├── dto.go                         # Data transfer objects
├── utils.go                       # Utility functions
└── README.md                      # This document

cmd/mcp/
└── main.go                        # Unified MCP server entry (stdio mode)
```

## Transports

| Transport | Endpoint | Use Case |
|-----------|----------|----------|
| **Streamable HTTP** | `POST/GET/DELETE /api/v1/mcp/sse` | Web apps, remote access, multi-client |
| **stdio** | `./bin/mcp` | Local CLI tools, Claude Desktop |

---

## Streamable HTTP Interface

The unified endpoint `/api/v1/mcp/sse` supports three HTTP methods per the [MCP Streamable HTTP spec](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http):

| Method | Purpose | When |
|--------|---------|------|
| **POST** | Send JSON-RPC messages (initialize, tools/list, tools/call, notifications) | All client-to-server communication |
| **GET** | Open SSE stream for server-to-client messages | After initialize, for server-initiated messages |
| **DELETE** | Close a session | When done |

### Required Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type: application/json` | POST | JSON-RPC payload |
| `Accept: application/json, text/event-stream` | POST/GET | Required by spec |
| `Mcp-Session-Id` | After initialize | Session ID from initialize response |
| `Mcp-Protocol-Version` | After initialize | e.g. `2025-06-18` |

### Protocol Flow

```
1. POST  /api/v1/mcp/sse  {"method":"initialize","params":{"protocolVersion":"2025-06-18"}}
   <- 200 OK + Mcp-Session-Id header + InitializeResult JSON

2. POST  /api/v1/mcp/sse  {"method":"notifications/initialized"}
   (with Mcp-Session-Id + Mcp-Protocol-Version headers)
   <- 202 Accepted

3. POST  /api/v1/mcp/sse  {"method":"tools/list"}
   (with Mcp-Session-Id + Mcp-Protocol-Version headers)
   <- 200 OK + tools catalog JSON

4. POST  /api/v1/mcp/sse  {"method":"tools/call","params":{"name":"list_schemas"}}
   <- 200 OK + tool result JSON

5. DELETE /api/v1/mcp/sse
   (with Mcp-Session-Id header)
   <- 204 No Content
```

### Quick Test

```bash
# Start the server
cd backend && make dev

# Initialize a session
curl -i -X POST http://localhost:8080/api/v1/mcp/sse \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}'
```

---

## Discovery & Health

```bash
# List available servers
curl http://localhost:8080/api/v1/mcp/servers | jq

# Health check
curl http://localhost:8080/api/v1/mcp/health | jq
```

---

## Client Configuration

### Claude Desktop (Streamable HTTP)

```json
{
  "mcpServers": {
    "leeforge": {
      "url": "http://localhost:8080/api/v1/mcp/sse"
    }
  }
}
```

### Claude Desktop (stdio)

```json
{
  "mcpServers": {
    "leeforge": {
      "command": "/path/to/backend/bin/mcp",
      "args": [],
      "env": {}
    }
  }
}
```

### Go SDK Client

```go
client := mcp.NewClient(&mcp.Implementation{
    Name:    "my-client",
    Version: "1.0.0",
}, nil)

transport := &mcp.StreamableClientTransport{
    Endpoint: "http://localhost:8080/api/v1/mcp/sse",
}

session, err := client.Connect(context.Background(), transport, nil)
```

---

## Available Tools

All tools are exposed through the unified server:

| Tool | Module | Description |
|------|--------|-------------|
| `list_schemas` | schema | List all entity schemas |
| `get_schema_by_module` | schema | Get schemas by module name |
| `get_schema_by_name` | schema | Get schema by entity name |
| `validate_schema` | schema | Validate a schema definition |
| `create_menu` | menu | Create a menu item |
| `batch_create_menus` | menu | Create multiple menu items |
| `get_menu_tree` | menu | Get the menu tree |
| `delete_menu` | menu | Delete a menu item |

---

**Version**: 3.0.0
**Last Updated**: 2026-02-15
**Transport**: Streamable HTTP (MCP spec 2025-06-18)
