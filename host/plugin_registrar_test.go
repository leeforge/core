package host

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestNewStaticPluginRegistrar_RegistersUniqueNames(t *testing.T) {
	r := chi.NewRouter()
	rt := newChiRuntimeAdapter(r)

	err := NewStaticPluginRegistrar("tenant", "tenant", "  ", "ou")(rt, nil, nil)
	require.NoError(t, err)
	require.Len(t, rt.plugins, 2)
	_, ok := rt.plugins["tenant"]
	require.True(t, ok)
	_, ok = rt.plugins["ou"]
	require.True(t, ok)
}

func TestNewManifestPluginRegistrar_RegistersEnabledPlugins(t *testing.T) {
	dir := t.TempDir()
	manifest := []byte("plugins:\n  - name: tenant\n    enabled: true\n  - name: ou\n    enabled: false\n")
	path := filepath.Join(dir, "plugins.yaml")
	require.NoError(t, os.WriteFile(path, manifest, 0o600))

	rt := newChiRuntimeAdapter(chi.NewRouter())
	err := NewManifestPluginRegistrar(path)(rt, nil, nil)
	require.NoError(t, err)
	require.Len(t, rt.plugins, 1)
	_, ok := rt.plugins["tenant"]
	require.True(t, ok)
}

func TestNewManifestPluginRegistrar_MissingManifestFails(t *testing.T) {
	rt := newChiRuntimeAdapter(chi.NewRouter())
	err := NewManifestPluginRegistrar(filepath.Join(t.TempDir(), "missing.yaml"))(rt, nil, nil)
	require.Error(t, err)
	require.Empty(t, rt.plugins)
}

func TestNewManifestPluginRegistrar_InvalidManifestFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	require.NoError(t, os.WriteFile(path, []byte("plugins: ["), 0o600))

	rt := newChiRuntimeAdapter(chi.NewRouter())
	err := NewManifestPluginRegistrar(path)(rt, nil, nil)
	require.Error(t, err)
}

func TestNewStaticPluginRegistrar_NilRuntimeFails(t *testing.T) {
	err := NewStaticPluginRegistrar("tenant")(nil, nil, nil)
	require.Error(t, err)
}

func TestNewManifestPluginRegistrar_NilRuntimeFails(t *testing.T) {
	err := NewManifestPluginRegistrar("plugins.yaml")(nil, nil, nil)
	require.Error(t, err)
}

func TestNewStaticPluginRegistrar_DuplicateRegisterError(t *testing.T) {
	rt := newChiRuntimeAdapter(chi.NewRouter())
	rt.plugins["tenant"] = httptest.NewRecorder()
	err := NewStaticPluginRegistrar("tenant")(rt, nil, nil)
	require.Error(t, err)
}
