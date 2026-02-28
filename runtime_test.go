package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	corecore "github.com/leeforge/core/core"
	"github.com/leeforge/core/server/config"
	frameplugin "github.com/leeforge/framework/plugin"
	frameLogging "github.com/leeforge/framework/logging"
	frameworkruntime "github.com/leeforge/framework/runtime"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type staticResourceProvider struct {
	resources *RuntimeResources
	err       error
}

func (p staticResourceProvider) Build(_ context.Context, _ ResourceInput) (*RuntimeResources, error) {
	return p.resources, p.err
}

func TestBuildRuntime_NoPluginRegistrarDoesNotFail(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeConfigFiles(t, dir)

	rt, err := BuildRuntime(context.Background(), RuntimeOptions{
		ConfigPath:       dir,
		ResourceProvider: staticResourceProvider{resources: &RuntimeResources{}},
		SkipMigrate:      true,
	})

	require.NoError(t, err)
	require.NotNil(t, rt)
	require.NotNil(t, rt.Handler())
}

func TestBuildRuntime_PluginRegistrarRegistersSwaggerRoute(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeConfigFiles(t, dir)

	rt, err := BuildRuntime(context.Background(), RuntimeOptions{
		ConfigPath:       dir,
		ResourceProvider: staticResourceProvider{resources: &RuntimeResources{}},
		SkipMigrate:      true,
		PluginRegistrar: func(rt *frameworkruntime.Runtime, _ *frameplugin.ServiceRegistry, _ *zap.Logger) error {
			return rt.Register(&dummyPlugin{})
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	rt.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"/tenants"`)
}

func TestDefaultResourceProvider_PostgresDriverRegistered(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host: "localhost",
			Port: "5432",
			Name: "leeforge",
		},
	}

	res, err := defaultResourceProvider{}.Build(context.Background(), ResourceInput{
		Config: cfg,
		Logger: zap.NewNop(),
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.CoreClient)
	if res != nil && res.CoreClient != nil {
		require.NoError(t, res.CoreClient.Close())
	}
}

func writeRuntimeConfigFiles(t *testing.T, dir string) {
	t.Helper()
	writeConfigFile(t, dir, "server", "server:\n  port: \"8080\"\n")
	writeConfigFile(t, dir, "database", "database:\n  host: \"localhost\"\n  port: \"5432\"\n  name: \"leeforge\"\n  auto_migrate: false\n")
	writeConfigFile(t, dir, "cache", "cache: {}\n")
	writeConfigFile(t, dir, "log", "log: {}\n")
	writeConfigFile(t, dir, "tracing", "tracing: {}\n")
	writeConfigFile(t, dir, "metrics", "metrics: {}\n")
	writeConfigFile(t, dir, "security", "security: {}\n")
	writeConfigFile(t, dir, "access_control", "access_control: {}\n")
	writeConfigFile(t, dir, "frontend", "frontend: {}\n")
	writeConfigFile(t, dir, "captcha", "captcha: {}\n")
	writeConfigFile(t, dir, "init", "init: {}\n")
}

func writeConfigFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}

type dummyPlugin struct{}

func (d *dummyPlugin) Name() string           { return "dummy" }
func (d *dummyPlugin) Version() string        { return "0.1.0" }
func (d *dummyPlugin) Dependencies() []string { return nil }
func (d *dummyPlugin) Enable(context.Context, *frameplugin.AppContext) error {
	return nil
}

func (d *dummyPlugin) RegisterRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/tenants", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// dummyModule is a minimal Module implementation for testing ModuleFactories.
type dummyModule struct {
	name string
}

func (m *dummyModule) Name() string                          { return m.name }
func (m *dummyModule) RegisterPublicRoutes(router chi.Router) {}
func (m *dummyModule) RegisterPrivateRoutes(router chi.Router) {
	router.Get("/"+m.name, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(m.name))
	})
}

func TestBuildRuntime_ModuleFactoriesRegistersExternalModule(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeConfigFiles(t, dir)

	rt, err := BuildRuntime(context.Background(), RuntimeOptions{
		ConfigPath:       dir,
		ResourceProvider: staticResourceProvider{resources: &RuntimeResources{}},
		SkipMigrate:      true,
		ModuleFactories: []corecore.ModuleFactory{
			func(_ frameLogging.Logger, _ *corecore.Dependencies) corecore.Module {
				return &dummyModule{name: "external-test"}
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, rt)
	require.NotNil(t, rt.Handler())
}
