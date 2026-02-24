package host

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	framePerm "github.com/leeforge/framework/permission"
)

type SwaggerOptions struct {
	Title       string
	Description string
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
}

type openAPIDocument struct {
	Swagger  string                          `json:"swagger"`
	Info     openAPIInfo                     `json:"info"`
	Host     string                          `json:"host"`
	BasePath string                          `json:"basePath"`
	Schemes  []string                        `json:"schemes"`
	Tags     []openAPITag                    `json:"tags,omitempty"`
	Paths    map[string]map[string]openAPIOp `json:"paths"`
}

type openAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type openAPIOp struct {
	Summary   string                 `json:"summary"`
	Tags      []string               `json:"tags,omitempty"`
	Produces  []string               `json:"produces,omitempty"`
	Responses map[string]openAPIResp `json:"responses"`
}

type openAPIResp struct {
	Description string `json:"description"`
}

type openAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Leeforge Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/swagger/doc.json",
      dom_id: "#swagger-ui"
    });
  </script>
</body>
</html>`

func RegisterSwaggerChi(router chi.Router, opts *SwaggerOptions) error {
	if router == nil {
		return fmt.Errorf("router is nil")
	}

	cfg := defaultSwaggerOptions(opts)
	snapshot, err := framePerm.SnapshotFromRouter(router)
	if err != nil {
		return err
	}

	doc := buildOpenAPIDoc(filterSwaggerRoutes(snapshot.Routes), cfg)
	docJSON, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	router.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusPermanentRedirect)
	})
	router.Get("/swagger/index.html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerHTML))
	})
	router.Get("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(docJSON)
	})
	return nil
}

func defaultSwaggerOptions(opts *SwaggerOptions) SwaggerOptions {
	cfg := SwaggerOptions{
		Title:       "Leeforge API",
		Description: "Leeforge API routes",
		Version:     "1.0",
		Host:        "localhost:8080",
		BasePath:    DefaultBasePath,
		Schemes:     []string{"http"},
	}
	if opts == nil {
		return cfg
	}
	if strings.TrimSpace(opts.Title) != "" {
		cfg.Title = opts.Title
	}
	if strings.TrimSpace(opts.Description) != "" {
		cfg.Description = opts.Description
	}
	if strings.TrimSpace(opts.Version) != "" {
		cfg.Version = opts.Version
	}
	if strings.TrimSpace(opts.Host) != "" {
		cfg.Host = opts.Host
	}
	if strings.TrimSpace(opts.BasePath) != "" {
		cfg.BasePath = opts.BasePath
	}
	if len(opts.Schemes) > 0 {
		cfg.Schemes = append([]string(nil), opts.Schemes...)
	}
	return cfg
}

func filterSwaggerRoutes(routes []framePerm.RouteInfo) []framePerm.RouteInfo {
	if len(routes) == 0 {
		return nil
	}
	filtered := make([]framePerm.RouteInfo, 0, len(routes))
	for _, route := range routes {
		path := strings.TrimSpace(route.Path)
		if path == "/swagger" || strings.HasPrefix(path, "/swagger/") {
			continue
		}
		filtered = append(filtered, route)
	}
	return filtered
}

func buildOpenAPIDoc(routes []framePerm.RouteInfo, opts SwaggerOptions) openAPIDocument {
	doc := openAPIDocument{
		Swagger: "2.0",
		Info: openAPIInfo{
			Title:       opts.Title,
			Description: opts.Description,
			Version:     opts.Version,
		},
		Host:     opts.Host,
		BasePath: opts.BasePath,
		Schemes:  append([]string(nil), opts.Schemes...),
		Paths:    map[string]map[string]openAPIOp{},
	}
	tagSet := make(map[string]struct{})

	for _, route := range routes {
		if route.Method == "" || route.Path == "" {
			continue
		}
		method := strings.ToLower(route.Method)
		swaggerPath := normalizePath(route.Path, doc.BasePath)
		if _, exists := doc.Paths[swaggerPath]; !exists {
			doc.Paths[swaggerPath] = map[string]openAPIOp{}
		}
		if _, exists := doc.Paths[swaggerPath][method]; exists {
			continue
		}

		summary := strings.TrimSpace(route.Description)
		if summary == "" {
			summary = strings.ToUpper(route.Method) + " " + swaggerPath
		}
		tag := classifyRouteTag(swaggerPath)
		tagSet[tag] = struct{}{}
		doc.Paths[swaggerPath][method] = openAPIOp{
			Summary:  summary,
			Tags:     []string{tag},
			Produces: []string{"application/json"},
			Responses: map[string]openAPIResp{
				"200": {Description: "OK"},
			},
		}
	}

	doc.Tags = buildTags(tagSet)
	doc.Paths = sortPaths(doc.Paths)
	return doc
}

func normalizePath(path, basePath string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return "/"
	}
	if basePath != "" && strings.HasPrefix(normalized, basePath) {
		normalized = strings.TrimPrefix(normalized, basePath)
		if normalized == "" {
			normalized = "/"
		}
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return normalized
}

func sortPaths(paths map[string]map[string]openAPIOp) map[string]map[string]openAPIOp {
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make(map[string]map[string]openAPIOp, len(paths))
	for _, k := range keys {
		methodKeys := make([]string, 0, len(paths[k]))
		for m := range paths[k] {
			methodKeys = append(methodKeys, m)
		}
		sort.Strings(methodKeys)

		methods := make(map[string]openAPIOp, len(methodKeys))
		for _, m := range methodKeys {
			methods[m] = paths[k][m]
		}
		out[k] = methods
	}
	return out
}

func classifyRouteTag(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "General"
	}
	parts := strings.Split(trimmed, "/")
	segment := parts[0]
	switch segment {
	case "auth":
		return "Auth"
	case "users":
		return "Users"
	case "roles":
		return "Roles"
	case "permissions":
		return "Permissions"
	case "menus":
		return "Menus"
	case "dictionaries", "dictionary-details":
		return "Dictionaries"
	case "domains":
		return "Domains"
	case "media":
		return "Media"
	case "api-keys":
		return "API Keys"
	case "schemas":
		return "Schemas"
	case "mcp":
		return "MCP"
	case "logs":
		return "Logs"
	case "captcha":
		return "Captcha"
	case "init":
		return "Initialization"
	case "profile":
		return "Profile"
	case "health":
		return "System"
	default:
		return humanizeTag(segment)
	}
}

func buildTags(tagSet map[string]struct{}) []openAPITag {
	if len(tagSet) == 0 {
		return nil
	}
	names := make([]string, 0, len(tagSet))
	for name := range tagSet {
		names = append(names, name)
	}
	sort.Strings(names)
	tags := make([]openAPITag, 0, len(names))
	for _, name := range names {
		tags = append(tags, openAPITag{
			Name:        name,
			Description: name + " related APIs",
		})
	}
	return tags
}

func humanizeTag(segment string) string {
	words := strings.Fields(strings.ReplaceAll(segment, "-", " "))
	for i, word := range words {
		if word == "" {
			continue
		}
		lower := strings.ToLower(word)
		words[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, " ")
}
