// Command api runs the SSDLC example HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CaioMicael/ssdlc-example/backend/internal/httpapi"
	"github.com/CaioMicael/ssdlc-example/backend/internal/store"
)

const shutdownTimeout = 10 * time.Second

// wrapHandler lets tests inject middleware (e.g. artificial latency) around
// the router without changing production behavior. Production never
// overrides it.
var wrapHandler = func(h http.Handler) http.Handler { return h }

// onStoreOpened lets tests observe the store instance run opens right after
// a successful store.Open, e.g. to close it and simulate a database outage
// while the server is running. Production never overrides it.
var onStoreOpened = func(*store.Store) {}

// run starts the HTTP server and blocks until ctx is canceled, then shuts it
// down gracefully (waiting up to shutdownTimeout for in-flight requests).
// getenv resolves PORT (default "8080") and WEB_DIR (default "./web"). If
// ready is non-nil, the listener's actual address is sent on it once the
// server is listening.
func run(ctx context.Context, getenv func(string) string, ready chan<- string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	webDir := getenv("WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}

	dbPath := getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./app.db"
	}
	logger.Info("opening database", "db_path", dbPath)
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	onStoreOpened(st)

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	handler := wrapHandler(httpapi.NewRouter(webDir, st))
	srv := &http.Server{Handler: handler}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	logger.Info("listening", "addr", ln.Addr().String())
	if ready != nil {
		ready <- ln.Addr().String()
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		<-serveErr
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, nil); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
