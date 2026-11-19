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

package nativessh

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m482: a terminating gateway (an ssh-gateway release) stops accepting and
// keeps the sessions it holds until they end on their own, their access ends,
// or the drain bound passes, as Render gives open SSH connections an hour
// before its own maintenance closes them.

// drainingGateway is one gateway whose sessions run under end's context, the
// way cmd/ssh-gateway runs them, so a test can drain it and then pass the
// bound.
type drainingGateway struct {
	addr   string
	server *Server
	end    context.CancelCauseFunc
	served chan error
}

func startDrainingGateway(t *testing.T, st *gatewaytest.FakeStore, resolver *gatewaytest.FakeResolver, clientSigner ssh.Signer, configure func(*Server)) *drainingGateway {
	t.Helper()
	st.Key = store.SSHKey{Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey())}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sessionCtx, end := context.WithCancelCause(context.Background())
	t.Cleanup(func() { end(nil) })
	server := &Server{Store: st, Apps: resolver, Executor: &gatewaytest.EchoExecutor{}, Signer: signer(t), SessionTimeout: time.Minute, RevalidateInterval: -1}
	if configure != nil {
		configure(server)
	}
	gw := &drainingGateway{addr: listener.Addr().String(), server: server, end: end, served: make(chan error, 1)}
	go func() { gw.served <- server.Serve(sessionCtx, listener) }()
	return gw
}

// shell is one open interactive session the test types into.
type shell struct {
	client *ssh.Client
	stdin  io.WriteCloser
	lines  *bufio.Reader
	done   chan error
}

func openShell(t *testing.T, addr string, clientSigner ssh.Signer) *shell {
	t.Helper()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	sh := &shell{client: client, stdin: stdin, lines: bufio.NewReader(stdout), done: make(chan error, 1)}
	go func() { sh.done <- session.Wait() }()
	return sh
}

// echo types line and fails unless the shell answers with it.
func (sh *shell) echo(t *testing.T, line string) {
	t.Helper()
	if _, err := io.WriteString(sh.stdin, line+"\n"); err != nil {
		t.Fatalf("write %q: %v", line, err)
	}
	got := make(chan string, 1)
	go func() {
		text, _ := sh.lines.ReadString('\n')
		got <- text
	}()
	select {
	case text := <-got:
		if strings.TrimSpace(text) != line {
			t.Fatalf("echo = %q, want %q", text, line)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no echo of %q", line)
	}
}

// ended waits for the session to end, server side, and returns its error.
func (sh *shell) ended(t *testing.T) error {
	t.Helper()
	select {
	case err := <-sh.done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("the session did not end")
		return nil
	}
}

func shutdownAsync(server *Server, ctx context.Context) chan error {
	result := make(chan error, 1)
	go func() { result <- server.Shutdown(ctx) }()
	return result
}

// waitRefused waits until the gateway refuses a new connection: its listener
// is closed (a listener left open but unserved times out instead).
func waitRefused(t *testing.T, addr string) {
	t.Helper()
	waitFor(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
		}
		return errors.Is(err, syscall.ECONNREFUSED)
	})
}

// A session open when the drain starts keeps exchanging data while new
// connections are refused, and Shutdown returns once its user ends it.
func TestShutdownKeepsAnOpenSessionAndRefusesNewConnections(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	gw := startDrainingGateway(t, st, &gatewaytest.FakeResolver{}, clientSigner, nil)
	sh := openShell(t, gw.addr, clientSigner)
	sh.echo(t, "before the drain")

	shutdown := shutdownAsync(gw.server, context.Background())
	waitRefused(t, gw.addr)
	if _, err := dialGateway(gw.addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
		t.Fatal("a draining gateway accepted a new SSH connection")
	}
	select {
	case err := <-gw.served:
		if err != nil {
			t.Fatalf("Serve = %v, want nil once draining", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve kept accepting after Shutdown")
	}

	sh.echo(t, "during the drain")
	time.Sleep(50 * time.Millisecond)
	sh.echo(t, "still there")
	select {
	case err := <-shutdown:
		t.Fatalf("Shutdown returned %v while a session was open", err)
	default:
	}

	_ = sh.stdin.Close() // the user exits the shell
	if err := sh.ended(t); err != nil {
		t.Fatalf("the session ended with %v, want a clean exit", err)
	}
	_ = sh.client.Close()
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatalf("Shutdown = %v, want nil once the last session ended", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return after the last session ended")
	}
	if ended := st.EndedSessions(); len(ended) != 1 || !endsWith(ended[0], ":completed") {
		t.Fatalf("session audit result = %v, want one entry ending :completed", ended)
	}
}

// The bound: Shutdown gives up waiting with the context's error, the session
// is still open, and ending the session context with ErrGatewayShutdown
// closes it.
func TestDrainBoundEndsTheRemainingSessions(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	gw := startDrainingGateway(t, st, &gatewaytest.FakeResolver{}, clientSigner, nil)
	sh := openShell(t, gw.addr, clientSigner)

	bound, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := gw.server.Shutdown(bound); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want the bound's deadline while a session is open", err)
	}
	sh.echo(t, "at the bound")

	gw.end(sshgateway.ErrGatewayShutdown)
	if err := sh.ended(t); err == nil {
		t.Fatal("the session ended cleanly, want it cut at the bound")
	}
	if err := gw.server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown after the bound = %v, want nil", err)
	}
	waitFor(t, func() bool { return len(st.EndedSessions()) == 1 })
	if ended := st.EndedSessions(); !endsWith(ended[0], ":failed") {
		t.Fatalf("session audit result = %v, want one entry ending :failed", ended)
	}
}

// Draining changes nothing about what else ends a session: a revocation and
// the session cap still end it, and Shutdown returns with it.
func TestDrainingSessionStillEndsOnRevocationAndTheSessionCap(t *testing.T) {
	for name, tc := range map[string]struct {
		configure func(*Server)
		trigger   func(*gatewaytest.FakeResolver)
		result    string
	}{
		"revocation": {
			configure: func(s *Server) { s.RevalidateInterval = 10 * time.Millisecond },
			trigger:   func(r *gatewaytest.FakeResolver) { r.SetFlip(errors.New("revoked")) },
			result:    ":revoked",
		},
		"session cap": {
			configure: func(s *Server) { s.SessionTimeout = 300 * time.Millisecond },
			trigger:   func(*gatewaytest.FakeResolver) {},
			result:    ":failed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			clientSigner := signer(t)
			st := &gatewaytest.FakeStore{}
			resolver := &gatewaytest.FakeResolver{}
			gw := startDrainingGateway(t, st, resolver, clientSigner, tc.configure)
			sh := openShell(t, gw.addr, clientSigner)
			sh.echo(t, "before the drain")

			shutdown := shutdownAsync(gw.server, context.Background())
			waitRefused(t, gw.addr)
			tc.trigger(resolver)
			if err := sh.ended(t); err == nil {
				t.Fatal("the session ended cleanly, want it ended by the gateway")
			}
			select {
			case err := <-shutdown:
				if err != nil {
					t.Fatalf("Shutdown = %v, want nil", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Shutdown did not return after the session ended")
			}
			if ended := st.EndedSessions(); len(ended) != 1 || !endsWith(ended[0], tc.result) {
				t.Fatalf("session audit result = %v, want one entry ending %s", ended, tc.result)
			}
		})
	}
}

// Shutdown before Serve has started still wins: Serve then accepts nothing.
func TestShutdownBeforeServeAcceptsNothing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Store: &gatewaytest.FakeStore{}, Apps: &gatewaytest.FakeResolver{}, Executor: &gatewaytest.EchoExecutor{}, Signer: signer(t)}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown = %v", err)
	}
	if err := server.Serve(context.Background(), listener); err != nil {
		t.Fatalf("Serve after Shutdown = %v, want nil", err)
	}
	if conn, err := net.DialTimeout("tcp", listener.Addr().String(), 200*time.Millisecond); err == nil {
		_ = conn.Close()
		t.Fatal("Serve after Shutdown left its listener open")
	}
}
