package ginadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/leeforge/core/host"
)

func TestRegisterAllGin_ExposesHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	err := RegisterAllGin(engine, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterAllGin_ForwardsOnlyAPIPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	err := RegisterAllGin(engine, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRegisterAllGin_ExposesSwagger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	err := RegisterAllGin(engine, host.CoreOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"swagger": "2.0"`)
}
