// Package httpapi provides the HTTP handlers and routing for the API server.
package httpapi

import "net/http"

// HealthHandler responds with a static JSON payload indicating the service is up.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
