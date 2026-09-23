package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// NewRouter builds the HTTP handler for the API and static frontend.
//
// - GET /healthz reports 200 when s.Ping succeeds and 503 otherwise.
// - The task CRUD and archive/restore routes are registered against s.
// - Any other path under /api/ returns 404 (no matching API route).
// - Any other GET path serves the matching file under webDir when it exists
//   and is not a directory; otherwise it falls back to webDir/index.html
//   (SPA fallback), always with a 200 response.
func NewRouter(webDir string, s TaskStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", NewHealthHandler(s))

	registerTaskRoutes(mux, s)
	registerTimerRoutes(mux, s)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveStaticOrFallback(webDir, w, r)
	})

	return mux
}

// serveStaticOrFallback serves the requested file from webDir if it exists
// and is a regular file, otherwise serves webDir/index.html. The requested
// path is cleaned relative to a synthetic root before being joined with
// webDir, so a traversal attempt (e.g. "/../secret.txt" or an encoded
// variant) can never resolve outside webDir.
func serveStaticOrFallback(webDir string, w http.ResponseWriter, r *http.Request) {
	cleaned := path.Clean("/" + r.URL.Path)
	fullPath := filepath.Join(webDir, filepath.FromSlash(cleaned))

	info, err := os.Stat(fullPath)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, fullPath)
		return
	}

	http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
}
