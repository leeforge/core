package echoadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/leeforge/core/host"
)

func TestRegisterAllEcho_ExposesHealth(t *testing.T) {
	e := echo.New()
	err := RegisterAllEcho(e, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterAllEcho_ForwardsOnlyAPIPrefix(t *testing.T) {
	e := echo.New()
	err := RegisterAllEcho(e, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRegisterAllEcho_ExposesSwagger(t *testing.T) {
	e := echo.New()
	err := RegisterAllEcho(e, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"swagger": "2.0"`)
}
