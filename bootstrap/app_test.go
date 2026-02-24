package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBootstrapAllModules_RegistersHealth(t *testing.T) {
	r, err := NewChiRouter(nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
