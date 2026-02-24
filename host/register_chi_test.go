package host

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegisterAllChi_RegistersHealthRoute(t *testing.T) {
	r := chi.NewRouter()
	err := RegisterAllChi(r, CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterAllChi_RouteConflictFails(t *testing.T) {
	r := chi.NewRouter()
	err := RegisterAllChi(r, CoreOptions{
		ModuleBootstrapper: func(router chi.Router, _ any, _ *zap.Logger) error {
			router.Get("/dupe", func(http.ResponseWriter, *http.Request) {})
			router.Get("/dupe", func(http.ResponseWriter, *http.Request) {})
			return nil
		},
	})
	require.Error(t, err)

	var conflict *ErrRouteConflict
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, http.MethodGet, conflict.Method)
	require.Equal(t, "/dupe", conflict.Path)
	require.NotEmpty(t, conflict.OwnerModule)
	require.NotEmpty(t, conflict.ConflictModule)
}
