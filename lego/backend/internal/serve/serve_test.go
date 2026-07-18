/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package serve

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// freeAddr returns a currently-free 127.0.0.1 address by binding an ephemeral
// port and immediately releasing it. Good enough for a test that re-binds it a
// moment later.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release free port: %v", err)
	}
	return addr
}

// waitUntilServing polls url until it answers or the deadline elapses.
func waitUntilServing(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never became reachable at %s: %v", url, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// get fetches url and returns the status code and body.
func get(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s body: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// TestUntilShutdownExitsOnCancel is the w1/m30 regression guard: it proves
// that once the server is serving, cancelling the context makes UntilShutdown
// RETURN promptly (graceful drain) instead of blocking for the whole grace
// period the way the pre-fix `log.Fatal(ListenAndServe())` did (.pm/w1/019.md).
// Against the old blocking code this test times out at the 5s guard below;
// against the fix it returns in milliseconds. No drain window here — the zero
// Options value is the pre-m52 behavior.
func TestUntilShutdownExitsOnCancel(t *testing.T) {
	addr := freeAddr(t)
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		ReadHeaderTimeout: 2 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- UntilShutdown(ctx, srv, Options{}) }()

	// Wait until the server is actually accepting connections, so the cancel
	// below exercises the shutdown path (not a still-binding server).
	client := &http.Client{Timeout: time.Second}
	waitUntilServing(t, client, "http://"+addr+"/")

	// The server is up and should NOT have returned yet.
	select {
	case err := <-done:
		t.Fatalf("UntilShutdown returned before context cancel: %v", err)
	default:
	}

	// SIGTERM analogue: cancel the context and require a prompt graceful return.
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UntilShutdown did not return within 5s of context cancel — it is still serving through the grace period")
	}

	// After shutdown the listener is closed: new connections are refused.
	if resp, err := client.Get("http://" + addr + "/"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server still accepting connections after shutdown")
	}
}

// TestUntilShutdownSurfacesBindError proves the startup-error path: when
// ListenAndServe fails to bind (address already in use), UntilShutdown returns
// that error instead of hanging — this is the branch main() fails fast on
// rather than logging a phantom "listening" line.
func TestUntilShutdownSurfacesBindError(t *testing.T) {
	// Hold a port for the whole test so the server under test can't bind it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Addr: ln.Addr().String(), ReadHeaderTimeout: 2 * time.Second}
	done := make(chan error, 1)
	go func() { done <- UntilShutdown(context.Background(), srv, Options{}) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a bind error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UntilShutdown hung on a bind failure instead of returning the error")
	}
}

// TestUntilShutdownDrainWindow is the w1/m52 drain-sequence guard: on cancel,
// readiness must flip to 503 immediately while REGULAR requests keep
// succeeding through the drain window — the terminating pod stays a working
// backend for late-routed traffic — and only after the window does the
// listener close. Against a fix-less server (shutdown on cancel), the
// mid-window request fails and this test catches it.
func TestUntilShutdownDrainWindow(t *testing.T) {
	addr := freeAddr(t)
	ready := &Readiness{}
	mux := http.NewServeMux()
	mux.Handle("GET /readyz", ready.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("served"))
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 2 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const window = 500 * time.Millisecond
	done := make(chan error, 1)
	go func() {
		done <- UntilShutdown(ctx, srv, Options{Readiness: ready, DrainWindow: window, ShutdownTimeout: 2 * time.Second})
	}()

	client := &http.Client{Timeout: time.Second}
	waitUntilServing(t, client, "http://"+addr+"/")

	// Before the signal: ready and serving.
	if code, _ := get(t, client, "http://"+addr+"/readyz"); code != http.StatusOK {
		t.Fatalf("/readyz before drain = %d, want 200", code)
	}

	// SIGTERM analogue.
	cancel()

	// Mid-window: /readyz must answer 503 while a regular request still
	// succeeds — this succeeding request IS the fix (a late-routed request
	// during endpoint propagation must not hit a closed listener).
	deadline := time.Now().Add(window / 2)
	sawDraining := false
	for time.Now().Before(deadline) {
		code, body := get(t, client, "http://"+addr+"/readyz")
		if code == http.StatusServiceUnavailable && body == "draining" {
			sawDraining = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawDraining {
		t.Fatal("/readyz never flipped to 503 draining after cancel")
	}
	if code, body := get(t, client, "http://"+addr+"/"); code != http.StatusOK || body != "served" {
		t.Fatalf("regular request during drain window = %d %q, want 200 served", code, body)
	}

	// After the window the server shuts down cleanly and refuses connections.
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("drain-window shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UntilShutdown did not return after the drain window elapsed")
	}
	if resp, err := client.Get("http://" + addr + "/"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server still accepting connections after drain-window shutdown")
	}
}

// TestReadinessHandler pins the probe contract both probes are configured
// against: 200 "ok" before StartDraining, 503 "draining" after, idempotent.
func TestReadinessHandler(t *testing.T) {
	ready := &Readiness{}
	srv := &http.Server{Addr: freeAddr(t), Handler: ready.Handler(), ReadHeaderTimeout: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- UntilShutdown(ctx, srv, Options{}) }()
	client := &http.Client{Timeout: time.Second}
	url := "http://" + srv.Addr + "/"
	waitUntilServing(t, client, url)

	if code, body := get(t, client, url); code != http.StatusOK || body != "ok" {
		t.Fatalf("before draining = %d %q, want 200 ok", code, body)
	}
	if ready.Draining() {
		t.Fatal("Draining() true before StartDraining")
	}
	ready.StartDraining()
	ready.StartDraining() // idempotent
	if !ready.Draining() {
		t.Fatal("Draining() false after StartDraining")
	}
	if code, body := get(t, client, url); code != http.StatusServiceUnavailable || body != "draining" {
		t.Fatalf("after draining = %d %q, want 503 draining", code, body)
	}
	cancel()
	<-done
}
