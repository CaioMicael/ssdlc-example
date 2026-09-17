package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func getenvFunc(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

// AC4: PORT empty defaults to listening on port 8080.
func TestRun_DefaultPort_ListensOn8080(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, getenvFunc(map[string]string{"WEB_DIR": t.TempDir()}), ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("could not parse addr %q: %v", addr, err)
	}
	if port != "8080" {
		t.Fatalf("port = %q, want %q", port, "8080")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

// AC4: a custom PORT value is used instead of the default.
func TestRun_CustomPort_UsesGivenPort(t *testing.T) {
	wantPort := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, getenvFunc(map[string]string{"PORT": wantPort, "WEB_DIR": t.TempDir()}), ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("could not parse addr %q: %v", addr, err)
	}
	if port != wantPort {
		t.Fatalf("port = %q, want %q", port, wantPort)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

// AC5: canceling ctx while a request is in flight lets that request finish
// with 200, and run returns well within the 10s shutdown budget.
func TestRun_GracefulShutdown_WaitsForInFlightRequest(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	originalWrap := wrapHandler
	wrapHandler = func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			time.Sleep(300 * time.Millisecond)
			h.ServeHTTP(w, r)
		})
	}
	defer func() { wrapHandler = originalWrap }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, getenvFunc(map[string]string{"PORT": "0", "WEB_DIR": webDir}), ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}
	addr = strings.Replace(addr, "[::]", "127.0.0.1", 1)
	addr = strings.Replace(addr, "0.0.0.0", "127.0.0.1", 1)

	type result struct {
		status int
		err    error
	}
	reqResult := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			reqResult <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.ReadAll(resp.Body)
		reqResult <- result{status: resp.StatusCode}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request never started")
	}

	shutdownStart := time.Now()
	cancel()

	var runErr error
	select {
	case runErr = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return within 10s")
	}
	elapsed := time.Since(shutdownStart)
	if elapsed >= 10*time.Second {
		t.Fatalf("shutdown took %v, want < 10s", elapsed)
	}
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}

	res := <-reqResult
	if res.err != nil {
		t.Fatalf("in-flight request failed: %v", res.err)
	}
	if res.status != http.StatusOK {
		t.Fatalf("in-flight request status = %d, want 200", res.status)
	}
}
