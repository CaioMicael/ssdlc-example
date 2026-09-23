package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// AC1: GET /healthz responds 200, Content-Type: application/json, body {"status":"ok"}
// when the store is reachable.
func TestHealthHandler_StoreHealthy_ReturnsOKStatusJSON(t *testing.T) {
	s := newTestStore(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler(s)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want %q", body, `{"status":"ok"}`)
	}
}

// Edge Case (spec): if the database is unavailable, /healthz responds 503
// with the spec error body {"error":{"code":"SERVICE_UNAVAILABLE",...}}.
func TestHealthHandler_StoreUnavailable_Returns503(t *testing.T) {
	s := newTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler(s)(rec, req)

	if rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"SERVICE_UNAVAILABLE"`) {
		t.Fatalf("body = %q, want it to contain SERVICE_UNAVAILABLE code", rec.Body.String())
	}
}
