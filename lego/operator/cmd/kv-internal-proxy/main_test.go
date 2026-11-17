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
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresConfigurationBeforeListening(t *testing.T) {
	for _, env := range []map[string]string{
		{},
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1"},
		{"VALKEY_PASSWORD": "secret"},
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": "127.0.0.1:0", "BEX_KV_TLS_ADDR": "127.0.0.1:0"},
		// A store with internal authentication keeps the password for the check.
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "BEX_KV_INTERNAL_AUTH": "client"},
	} {
		err := run(context.Background(), func(key string) string { return env[key] }, false)
		if err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("missing config: %v", err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("configuration error exposed password")
		}
	}
}

func TestRunCheckReportsUnreachableUpstream(t *testing.T) {
	env := map[string]string{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:0", "VALKEY_PASSWORD": "secret"}
	if err := run(t.Context(), func(key string) string { return env[key] }, true); err == nil || !strings.Contains(err.Error(), "upstream connection failed") {
		t.Fatalf("check=%v", err)
	}
}

// An unknown BEX_KV_INTERNAL_AUTH fails closed, for the listeners and for the
// readiness check, rather than falling back to either mode.
func TestRunRefusesAnUnknownInternalAuthMode(t *testing.T) {
	for _, check := range []bool{false, true} {
		env := map[string]string{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": "127.0.0.1:0", "BEX_KV_INTERNAL_AUTH": "proxy"}
		if err := run(t.Context(), func(key string) string { return env[key] }, check); err == nil || !strings.Contains(err.Error(), "BEX_KV_INTERNAL_AUTH") {
			t.Fatalf("check=%v: %v", check, err)
		}
	}
}

// fakeValkey answers each connection as a password-protected Valkey does:
// AUTH default <password> then PING reads +OK then +PONG, and a PING before
// AUTH reads NOAUTH. It writes the first bytes each connection sent to seen.
func fakeValkey(t *testing.T, password string, seen chan<- string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	auth := fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$7\r\ndefault\r\n$%d\r\n%s\r\n", len(password), password)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				buf := make([]byte, 256)
				n, err := conn.Read(buf)
				if err != nil {
					return
				}
				got := string(buf[:n])
				seen <- got
				switch {
				case strings.HasPrefix(got, auth):
					_, _ = io.WriteString(conn, "+OK\r\n")
					if strings.TrimPrefix(got, auth) == "" {
						n, _ = conn.Read(buf)
						got = string(buf[:n])
					} else {
						got = strings.TrimPrefix(got, auth)
					}
					if strings.Contains(got, "PING") {
						_, _ = io.WriteString(conn, "+PONG\r\n")
					}
				default:
					_, _ = io.WriteString(conn, "-NOAUTH Authentication required.\r\n")
				}
			}()
		}
	}()
	return listener.Addr().String()
}

// BEX_KV_INTERNAL_AUTH=client (w6/m463): the plain listener forwards the
// client's bytes with no AUTH of its own, so a client without the password
// reads Valkey's NOAUTH, while the readiness check still authenticates.
func TestRunClientModeForwardsTheClientsOwnAuth(t *testing.T) {
	const password = "secret"
	seen := make(chan string, 4)
	upstream := fakeValkey(t, password, seen)
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()
	env := map[string]string{"BEX_KV_UPSTREAM_ADDR": upstream, "VALKEY_PASSWORD": password, "BEX_KV_INTERNAL_ADDR": addr, "BEX_KV_INTERNAL_AUTH": "client"}
	getenv := func(key string) string { return env[key] }

	if err := run(t.Context(), getenv, true); err != nil {
		t.Fatalf("the check in client mode: %v", err)
	}
	if got := <-seen; !strings.HasPrefix(got, "*3\r\n$4\r\nAUTH") {
		t.Fatalf("the check did not authenticate: %q", got)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx, getenv, false) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	var conn net.Conn
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if conn, err = net.Dial("tcp", addr); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 64)
	n, _ := conn.Read(reply)
	if got := <-seen; got != "PING\r\n" {
		t.Fatalf("upstream first read %q, want only the client's PING", got)
	}
	if string(reply[:n]) != "-NOAUTH Authentication required.\r\n" {
		t.Fatalf("client read %q, want Valkey's NOAUTH", reply[:n])
	}
}
