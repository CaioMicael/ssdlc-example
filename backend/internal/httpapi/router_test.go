package httpapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC2: any path under /api/ returns 404 because no API routes exist yet.
// webDir contains a real index.html so this test also proves the /api/
// handler does not fall through to the SPA fallback (a regression that
// rewired /api/ to serveStaticOrFallback would still 200 with the index
// body, or 404 only because the fixture happened to lack an index.html).
func TestRouter_ApiPathReturns404(t *testing.T) {
	webDir := t.TempDir()
	indexContent := "<html><body>SSDLC Example</body></html>"
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(webDir, newTestStore(t))

	req := httptest.NewRequest("GET", "/api/nao-existe", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), indexContent) {
		t.Fatalf("body leaked SPA index.html content for /api/ path: %q", rec.Body.String())
	}
}

// AC3: a GET path that isn't /api/ or /healthz and isn't a static file falls
// back to webDir/index.html with 200.
func TestRouter_UnknownPathFallsBackToIndexHTML(t *testing.T) {
	webDir := t.TempDir()
	indexContent := "<html><body>SSDLC Example</body></html>"
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(webDir, newTestStore(t))

	req := httptest.NewRequest("GET", "/rota/qualquer", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != indexContent {
		t.Fatalf("body = %q, want %q", rec.Body.String(), indexContent)
	}
}

// AC3: an existing static file under webDir is served with its own content.
func TestRouter_ExistingStaticFileIsServedWithItsContent(t *testing.T) {
	webDir := t.TempDir()
	assetContent := "body { color: red; }"
	if err := os.WriteFile(filepath.Join(webDir, "style.css"), []byte(assetContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(webDir, newTestStore(t))

	req := httptest.NewRequest("GET", "/style.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != assetContent {
		t.Fatalf("body = %q, want %q", rec.Body.String(), assetContent)
	}
}

// AC3 (safety): a path traversal attempt must never read a file placed
// outside webDir, regardless of how the ".." is encoded. This calls the
// static-serving logic directly so the request path's ".." segments reach
// it unmodified by any surrounding router redirect behavior.
func TestServeStaticOrFallback_TraversalNeverEscapesWebDir(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.Mkdir(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	indexContent := "<html>index</html>"
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}

	secretContent := "top-secret-value"
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte(secretContent), 0o644); err != nil {
		t.Fatal(err)
	}

	traversalPaths := []string{
		"/../secret.txt",
		"/../../secret.txt",
		"/%2e%2e/secret.txt",
	}

	for _, p := range traversalPaths {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()

		serveStaticOrFallback(webDir, rec, req)

		if strings.Contains(rec.Body.String(), secretContent) {
			t.Fatalf("path %q leaked outside-webDir content: %q", p, rec.Body.String())
		}
	}
}
