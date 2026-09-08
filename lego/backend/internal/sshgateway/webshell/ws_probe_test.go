/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webshell

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestProbeUnauthenticatedRefusalAgainstLiveHandler is the protocol-level
// assertion for w2/m90: a real webshell.Handler with a ticket secret returns
// the deterministic 401 "missing ticket" refusal to a ticketless upgrade.
func TestProbeUnauthenticatedRefusalAgainstLiveHandler(t *testing.T) {
	srv := httptest.NewServer((&Server{TicketSecret: []byte("probe-secret")}).Handler())
	t.Cleanup(srv.Close)

	if err := ProbeUnauthenticatedRefusal(srv.URL+"/shell", 2*time.Second); err != nil {
		t.Fatalf("ProbeUnauthenticatedRefusal against a healthy refusing edge: %v", err)
	}
}

// TestProbeUnauthenticatedRefusalDetectsWrongStatus covers a peer that answers
// HTTP but not with the refusal shape — e.g. a wrong route or a silent 200.
func TestProbeUnauthenticatedRefusalDetectsWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "ok", http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	if err := ProbeUnauthenticatedRefusal(srv.URL+"/shell", 2*time.Second); err == nil {
		t.Fatal("ProbeUnauthenticatedRefusal passed on HTTP 200; it must require 401 missing ticket")
	}
}

// TestProbeUnauthenticatedRefusalDetectsDisabledTransport covers the
// unactivated shape: secret absent ⇒ 503 "web shell not configured". That is
// distinguishable from alive-but-refusing and must fail the liveness probe.
func TestProbeUnauthenticatedRefusalDetectsDisabledTransport(t *testing.T) {
	srv := httptest.NewServer((&Server{}).Handler())
	t.Cleanup(srv.Close)

	if err := ProbeUnauthenticatedRefusal(srv.URL+"/shell", 2*time.Second); err == nil {
		t.Fatal("ProbeUnauthenticatedRefusal passed against a disabled (503) transport")
	}
}

// TestProbeUnauthenticatedRefusalDetectsDeadPort covers connection refused.
func TestProbeUnauthenticatedRefusalDetectsDeadPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	if err := ProbeUnauthenticatedRefusal("http://"+addr+"/shell", 400*time.Millisecond); err == nil {
		t.Fatal("ProbeUnauthenticatedRefusal passed against a closed port")
	}
}

// TestProbeUnauthenticatedRefusalDetectsHang covers a peer that accepts TLS/TCP
// and never answers headers — the classic silent-dead edge.
func TestProbeUnauthenticatedRefusalDetectsHang(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(2 * time.Second) // hold without writing a response
	}()

	if err := ProbeUnauthenticatedRefusal("http://"+ln.Addr().String()+"/shell", 400*time.Millisecond); err == nil {
		t.Fatal("ProbeUnauthenticatedRefusal passed against a hanging peer")
	}
}

// TestPublicShellWSLiveness is the scheduled synthetic itself: pointed at a
// real endpoint via BEX_TEST_SHELL_WS_URL (wss://ssh.bex.co/shell in the guard
// workflow), it asserts the live edge returns the refusal shape. Skipped when
// unset so ordinary `go test ./...` never dials the public internet.
func TestPublicShellWSLiveness(t *testing.T) {
	rawURL := os.Getenv("BEX_TEST_SHELL_WS_URL")
	if rawURL == "" {
		t.Skip("set BEX_TEST_SHELL_WS_URL=wss://host/shell to probe a live Web Shell edge")
	}
	if err := ProbeUnauthenticatedRefusal(rawURL, 10*time.Second); err != nil {
		t.Fatalf("live Web Shell edge %s did not refuse with 401 missing ticket: %v", rawURL, err)
	}
}
