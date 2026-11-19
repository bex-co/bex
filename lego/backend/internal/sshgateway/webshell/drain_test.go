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

package webshell

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w6/m482: a Shell session open when its gateway starts draining (an
// ssh-gateway release) keeps working until the user ends it, its access ends,
// or the drain bound passes; at the bound it ends with a reason the Shell
// page names.

// drainingShell serves the gateway the way cmd/ssh-gateway's web shell
// listener does: every request runs under the session context, and Inflight
// counts the hijacked sessions http.Server.Shutdown does not track.
type drainingShell struct {
	srv      *httptest.Server
	sessions *sshgateway.Inflight
	end      context.CancelCauseFunc
	st       *gatewaytest.FakeStore
}

func startDrainingShell(t *testing.T, resolver sshgateway.TargetResolver, configure func(*Server)) (*drainingShell, *websocket.Conn) {
	t.Helper()
	st := &gatewaytest.FakeStore{}
	exec := &gatewaytest.EchoExecutor{Started: make(chan struct{}, 1)}
	gw := newWSGateway(t, st, resolver, exec)
	gw.RevalidateInterval = -1
	if configure != nil {
		configure(gw)
	}
	sessionCtx, end := context.WithCancelCause(context.Background())
	sessions := &sshgateway.Inflight{Name: "web shell"}
	srv := httptest.NewUnstartedServer(sessions.Handler(gw.Handler()))
	srv.Config.BaseContext = func(net.Listener) context.Context { return sessionCtx }
	srv.Start()
	t.Cleanup(func() {
		end(nil)
		srv.Close()
	})
	conn, _, err := dialWS(t, srv, mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"}))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	select {
	case <-exec.Started:
	case <-time.After(2 * time.Second):
		t.Fatal("shell exec never started")
	}
	return &drainingShell{srv: srv, sessions: sessions, end: end, st: st}, conn
}

// stopAccepting is the listener half of the drain: it returns once the
// listener is closed, leaving the hijacked shell alone.
func (d *drainingShell) stopAccepting(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.srv.Config.Shutdown(ctx); err != nil {
		t.Fatalf("http Shutdown = %v, want nil (a hijacked shell is not a tracked connection)", err)
	}
}

// typeLine sends stdin and fails unless the shell echoes it.
func typeLine(t *testing.T, conn *websocket.Conn, line string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte(line)); err != nil {
		t.Fatalf("write %q: %v", line, err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, data, err := conn.ReadMessage()
	if err != nil || mt != websocket.BinaryMessage || string(data) != line {
		t.Fatalf("echo = (%d, %q, %v), want %q", mt, data, err, line)
	}
}

// A shell open at the drain start keeps exchanging data while new shells are
// refused; at the bound it ends as a time limit, which the Shell page names.
func TestWebSocketShellSurvivesTheDrainAndEndsAtTheBound(t *testing.T) {
	d, conn := startDrainingShell(t, &gatewaytest.FakeResolver{}, nil)
	typeLine(t, conn, "before the drain")

	d.stopAccepting(t)
	if _, _, err := dialWS(t, d.srv, mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})); err == nil {
		t.Fatal("a draining gateway accepted a new shell")
	}
	typeLine(t, conn, "during the drain")
	time.Sleep(50 * time.Millisecond)
	typeLine(t, conn, "still there")

	bound, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := d.sessions.Wait(bound); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait = %v, want the bound's deadline while a shell is open", err)
	}

	d.end(sshgateway.ErrGatewayShutdown)
	wantEnd(t, readErrorFrame(t, conn), endTimeout)
	if ended := waitEndedSessions(t, d.st, 1); len(ended) != 1 || !strings.HasSuffix(ended[0], ":failed") {
		t.Fatalf("session audit result = %v, want one entry ending :failed", ended)
	}
	if err := d.sessions.Wait(context.Background()); err != nil {
		t.Fatalf("Wait after the bound = %v, want nil", err)
	}
}

// The user closing the shell ends the drain's wait: the process need not
// stay up for the rest of the hour.
func TestWebSocketShellClosedDuringTheDrainEndsTheWait(t *testing.T) {
	d, conn := startDrainingShell(t, &gatewaytest.FakeResolver{}, nil)
	d.stopAccepting(t)
	waited := make(chan error, 1)
	go func() { waited <- d.sessions.Wait(context.Background()) }()
	typeLine(t, conn, "during the drain")

	_ = conn.Close()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("Wait = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the drain kept waiting after the only shell closed")
	}
}

// A revocation during the drain still ends the shell, named as one.
func TestWebSocketRevocationStillEndsADrainingShell(t *testing.T) {
	resolver := &gatewaytest.FakeResolver{}
	d, conn := startDrainingShell(t, resolver, func(s *Server) { s.RevalidateInterval = 10 * time.Millisecond })
	d.stopAccepting(t)
	typeLine(t, conn, "during the drain")

	resolver.SetFlip(core.ErrForbidden)
	wantEnd(t, readErrorFrame(t, conn), endRevoked)
	if ended := waitEndedSessions(t, d.st, 1); len(ended) != 1 || !strings.HasSuffix(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
}
