package mcp

import (
	"encoding/json"
	"net/http"
)

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
