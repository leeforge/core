package host

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestRegisterSwaggerChi_ExposesSwaggerRoutes(t *testing.T) {
	r := chi.NewRouter()
	err := RegisterAllChi(r, CoreOptions{})
	require.NoError(t, err)
	err = RegisterSwaggerChi(r, &SwaggerOptions{
		Title:       "Leeforge Test API",
		Description: "Test docs",
		Version:     "1.0",
		BasePath:    "/api/v1",
	})
	require.NoError(t, err)

	docReq := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	docW := httptest.NewRecorder()
	r.ServeHTTP(docW, docReq)
	require.Equal(t, http.StatusOK, docW.Code)
	require.Contains(t, docW.Body.String(), `"swagger": "2.0"`)
	require.Contains(t, docW.Body.String(), `"/health"`)
	var doc struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Paths map[string]map[string]struct {
			Tags []string `json:"tags"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(docW.Body.Bytes(), &doc))
	require.NotEmpty(t, doc.Tags)
	require.Equal(t, "System", doc.Tags[0].Name)
	require.Equal(t, []string{"System"}, doc.Paths["/health"]["get"].Tags)

	uiReq := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	uiW := httptest.NewRecorder()
	r.ServeHTTP(uiW, uiReq)
	require.Equal(t, http.StatusOK, uiW.Code)
	require.Contains(t, uiW.Body.String(), "SwaggerUIBundle")
}
