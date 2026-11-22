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
	"os"
	"os/exec"
	"strings"
	"syscall"
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
		// The allow-list refusal answers inside TLS, so it needs the certificate.
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": "127.0.0.1:0",
			"BEX_KV_ALLOWLIST_REFUSAL_ADDR": "127.0.0.1:0", "BEX_KV_TLS_CERT_FILE": "/tls/tls.crt"},
	} {
		err := run(context.Background(), func(key string) string { return env[key] }, false, nil)
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
	if err := run(t.Context(), func(key string) string { return env[key] }, true, nil); err == nil || !strings.Contains(err.Error(), "upstream connection failed") {
		t.Fatalf("check=%v", err)
	}
}

// An unknown BEX_KV_INTERNAL_AUTH fails closed, for the listeners and for the
// readiness check, rather than falling back to either mode.
func TestRunRefusesAnUnknownInternalAuthMode(t *testing.T) {
	for _, check := range []bool{false, true} {
		env := map[string]string{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": "127.0.0.1:0", "BEX_KV_INTERNAL_AUTH": "proxy"}
		if err := run(t.Context(), func(key string) string { return env[key] }, check, nil); err == nil || !strings.Contains(err.Error(), "BEX_KV_INTERNAL_AUTH") {
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

	if err := run(t.Context(), getenv, true, nil); err != nil {
		t.Fatalf("the check in client mode: %v", err)
	}
	if got := <-seen; !strings.HasPrefix(got, "*3\r\n$4\r\nAUTH") {
		t.Fatalf("the check did not authenticate: %q", got)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx, getenv, false, nil) }()
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

// pingUpstream answers the injected AUTH, then each PING with PONG and INFO
// clients with one client, until the client closes.
func pingUpstream(t *testing.T, password string) string {
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
				got := make([]byte, len(auth))
				if _, err := io.ReadFull(conn, got); err != nil || string(got) != auth {
					return
				}
				if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
					return
				}
				buf := make([]byte, 64)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					reply := "+PONG\r\n"
					if strings.Contains(string(buf[:n]), "INFO") {
						reply = "$20\r\nconnected_clients:1\r\n\r\n"
					}
					if _, err := io.WriteString(conn, reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func freeAddr(t *testing.T) string {
	t.Helper()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = probe.Close() }()
	return probe.Addr().String()
}

// dialHeld opens a client through addr once it listens and checks it is served.
func dialHeld(t *testing.T, addr string) net.Conn {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			t.Cleanup(func() { _ = conn.Close() })
			ping(t, conn)
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
	}
}

func ping(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 7)
	if _, err := io.ReadFull(conn, reply); err != nil || string(reply) != "+PONG\r\n" {
		t.Fatalf("read %q (%v), want +PONG", reply, err)
	}
}

// awaitClosed waits for addr to refuse a new connection.
func awaitClosed(t *testing.T, addr string) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("the listener still accepts")
		}
	}
}

func awaitEOF(t *testing.T, conn net.Conn, within time.Duration) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(within))
	var b [1]byte
	if n, err := conn.Read(b[:]); n != 0 || err != io.EOF {
		t.Fatalf("the held client was not closed within %v: n=%d err=%v", within, n, err)
	}
}

// With BEX_KV_DRAIN_SECONDS the drain trigger closes both listeners, while a
// held stream keeps being served until the deadline, when run returns.
func TestRunDrainsUntilTheDeadline(t *testing.T) {
	const password = "secret"
	upstream := pingUpstream(t, password)
	addr, tlsAddr, refusalAddr := freeAddr(t), freeAddr(t), freeAddr(t)
	certFile, keyFile, _ := writeCertificate(t)
	env := map[string]string{
		"BEX_KV_UPSTREAM_ADDR": upstream, "VALKEY_PASSWORD": password, "BEX_KV_INTERNAL_ADDR": addr,
		"BEX_KV_TLS_ADDR": tlsAddr, "BEX_KV_TLS_UPSTREAM_ADDR": "127.0.0.1:1", "BEX_KV_DRAIN_SECONDS": "1",
		"BEX_KV_ALLOWLIST_REFUSAL_ADDR": refusalAddr, "BEX_KV_TLS_CERT_FILE": certFile, "BEX_KV_TLS_KEY_FILE": keyFile,
	}
	drain := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- run(t.Context(), func(key string) string { return env[key] }, false, drain) }()
	held := dialHeld(t, addr)
	began := time.Now()
	close(drain)
	awaitClosed(t, addr)
	awaitClosed(t, tlsAddr)
	awaitClosed(t, refusalAddr)
	ping(t, held)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return at the drain's deadline")
	}
	if elapsed := time.Since(began); elapsed < time.Second {
		t.Fatalf("run returned %v after the drain began, before its 1s deadline", elapsed)
	}
	awaitEOF(t, held, time.Second)
}

// Unset, the drain trigger closes every stream at once, as SIGTERM did before
// w6/m512.
func TestRunWithoutDrainSecondsClosesAtOnce(t *testing.T) {
	const password = "secret"
	upstream := pingUpstream(t, password)
	addr := freeAddr(t)
	env := map[string]string{"BEX_KV_UPSTREAM_ADDR": upstream, "VALKEY_PASSWORD": password, "BEX_KV_INTERNAL_ADDR": addr}
	drain := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- run(t.Context(), func(key string) string { return env[key] }, false, drain) }()
	held := dialHeld(t, addr)
	close(drain)
	awaitEOF(t, held, 500*time.Millisecond)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not return")
	}
}

func TestRunRefusesAnInvalidDrainBeforeListening(t *testing.T) {
	for _, value := range []string{"-1", "30s", "x"} {
		addr := freeAddr(t)
		env := map[string]string{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": addr, "BEX_KV_DRAIN_SECONDS": value}
		// Bounded, so a value that is wrongly accepted ends the serve and fails.
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		err := run(ctx, func(key string) string { return env[key] }, false, nil)
		cancel()
		if err == nil || !strings.Contains(err.Error(), "BEX_KV_DRAIN_SECONDS") {
			t.Fatalf("%q: %v", value, err)
		}
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("%q: run left its listener open: %v", value, err)
		}
		_ = listener.Close()
	}
}

// The binary itself: the first SIGTERM drains and the second closes at once.
func TestSIGTERMDrainsAndASecondClosesAtOnce(t *testing.T) {
	if os.Getenv("BEX_KV_PROXY_CHILD") == "1" {
		os.Args = []string{"kv-internal-proxy"}
		main()
		return
	}
	const password = "secret"
	upstream := pingUpstream(t, password)
	addr := freeAddr(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSIGTERMDrainsAndASecondClosesAtOnce$")
	cmd.Env = os.Environ()
	for key, value := range map[string]string{
		"BEX_KV_PROXY_CHILD": "1", "BEX_KV_UPSTREAM_ADDR": upstream, "VALKEY_PASSWORD": password,
		"BEX_KV_INTERNAL_ADDR": addr, "BEX_KV_DRAIN_SECONDS": "60",
	} {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	held := dialHeld(t, addr)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	awaitClosed(t, addr)
	ping(t, held)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	awaitEOF(t, held, 2*time.Second)
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("the proxy exited with %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the proxy did not exit on the second SIGTERM")
	}
}
