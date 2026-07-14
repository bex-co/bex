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

package main

import (
	"context"
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

// TestServeUntilShutdownExitsOnCancel is the w1/m30 regression guard: it proves
// that once the server is serving, cancelling the context makes
// serveUntilShutdown RETURN promptly (graceful drain) instead of blocking for
// the whole grace period the way the pre-fix `log.Fatal(ListenAndServe())` did
// (.pm/w1/019.md). Against the old blocking code this test times out at the
// 5s guard below; against the fix it returns in milliseconds.
func TestServeUntilShutdownExitsOnCancel(t *testing.T) {
	addr := freeAddr(t)
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		ReadHeaderTimeout: 2 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- serveUntilShutdown(ctx, srv) }()

	// Wait until the server is actually accepting connections, so the cancel
	// below exercises the shutdown path (not a still-binding server).
	client := &http.Client{Timeout: time.Second}
	waitUntilServing(t, client, "http://"+addr+"/")

	// The server is up and should NOT have returned yet.
	select {
	case err := <-done:
		t.Fatalf("serveUntilShutdown returned before context cancel: %v", err)
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
		t.Fatal("serveUntilShutdown did not return within 5s of context cancel — it is still serving through the grace period")
	}

	// After shutdown the listener is closed: new connections are refused.
	if resp, err := client.Get("http://" + addr + "/"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server still accepting connections after shutdown")
	}
}

// TestServeUntilShutdownSurfacesBindError proves the startup-error path: when
// ListenAndServe fails to bind (address already in use), serveUntilShutdown
// returns that error instead of hanging — this is the branch main() fails fast
// on rather than logging a phantom "listening" line.
func TestServeUntilShutdownSurfacesBindError(t *testing.T) {
	// Hold a port for the whole test so the server under test can't bind it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Addr: ln.Addr().String(), ReadHeaderTimeout: 2 * time.Second}
	done := make(chan error, 1)
	go func() { done <- serveUntilShutdown(context.Background(), srv) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a bind error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilShutdown hung on a bind failure instead of returning the error")
	}
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
