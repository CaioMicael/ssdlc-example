// Package httpapi provides the HTTP handlers and routing for the API server.
package httpapi

import "net/http"

// NewHealthHandler returns a handler that responds 200 with
// {"status":"ok"} when s.Ping succeeds, and 503 with the spec error body
// when the database is unreachable.
func NewHealthHandler(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service unavailable", nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
