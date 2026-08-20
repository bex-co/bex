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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

var wsSecret = []byte("web-shell-gateway-secret")

func newWSGateway(t *testing.T, st *gatewaytest.FakeStore, resolver sshgateway.TargetResolver, exec sshgateway.Executor) *Server {
	t.Helper()
	return &Server{
		Store: st, Apps: resolver, Executor: exec,
		Metrics:      sshgateway.NewMetrics(prometheus.NewRegistry()),
		TicketSecret: wsSecret,
		Nonces:       &sshgateway.NonceGuard{Store: st},
	}
}

func wsURL(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/shell"
}

func dialWS(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, *http.Response, error) {
	return dialWSHeaders(t, srv, token, nil)
}

func dialWSHeaders(t *testing.T, srv *httptest.Server, token string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	protocols := []string{wsShellSubprotocol}
	if token != "" {
		protocols = append(protocols, wsTicketPrefix+token)
	}
	dialer := websocket.Dialer{Subprotocols: protocols}
	return dialer.Dial(wsURL(t, srv), headers)
}

func mintWS(t *testing.T, claims shellticket.Claims) string {
	t.Helper()
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = time.Now().Add(time.Minute).Unix()
	}
	token, err := shellticket.Mint(wsSecret, claims)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestWebSocketShellHappyPath(t *testing.T) {
	st := &gatewaytest.FakeStore{}
	resolver := &gatewaytest.FakeResolver{}
	exec := &gatewaytest.FakeExecutor{}
	srv := httptest.NewServer(newWSGateway(t, st, resolver, exec).Handler())
	defer srv.Close()

	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// The shared FakeExecutor reads two terminal sizes before writing output; the
	// browser sends the second via a resize control frame.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":100,"rows":40}`)); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	var output string
	var exitCode = -1
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case websocket.BinaryMessage:
			output += string(data)
		case websocket.TextMessage:
			var ctrl serverControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "exit" {
				exitCode = ctrl.Code
			}
		}
	}

	if output != "inside-app\n" {
		t.Errorf("terminal output = %q, want the app stdout", output)
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if !exec.TTY || len(exec.Command) != 1 || exec.Command[0] != "/bin/sh" {
		t.Errorf("executor got tty=%v command=%v, want an interactive /bin/sh", exec.TTY, exec.Command)
	}
	if len(exec.Sizes) == 0 || exec.Sizes[len(exec.Sizes)-1].Width != 100 {
		t.Errorf("resize not delivered to the exec stream: %+v", exec.Sizes)
	}
	if resolver.Subject != "user-1" || resolver.Username != "srv-abcdeabcdeabcdeabcde" {
		t.Errorf("resolver saw subject=%q username=%q", resolver.Subject, resolver.Username)
	}
	started, ended := st.StartedSessions(), waitEndedSessions(t, st, 1)
	if len(started) != 1 || len(ended) != 1 || !strings.HasSuffix(ended[0], ":completed") {
		t.Errorf("audit rows: started=%v ended=%v", started, ended)
	}
}

func TestWebSocketAuditUsesForwardedClientOnlyFromTrustedPeer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trusted bool
		want    string
	}{
		{name: "trusted peer", trusted: true, want: "203.0.113.9"},
		{name: "untrusted peer", trusted: false, want: "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &gatewaytest.FakeStore{}
			gw := newWSGateway(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{})
			if tc.trusted {
				gw.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
			}
			srv := httptest.NewServer(gw.Handler())
			defer srv.Close()

			headers := http.Header{"X-Forwarded-For": []string{"198.51.100.7, 203.0.113.9"}}
			token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
			conn, _, err := dialWSHeaders(t, srv, token, headers)
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`))
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
			conn.Close()

			started := st.StartedSessions()
			if len(started) != 1 || !strings.HasPrefix(started[0].RemoteAddress, tc.want) {
				t.Fatalf("started sessions = %+v, want remote address %q", started, tc.want)
			}
		})
	}
}

// waitEndedSessions polls for `want` session-end audit rows. bridgeShell sends
// the WebSocket close frame before runWebSocketSession's deferred EndSSHSession
// runs, so a client that asserts the moment its read loop breaks races the
// audit write. Returns whatever it has at the deadline so the caller reports
// the real rows.
func waitEndedSessions(t *testing.T, st *gatewaytest.FakeStore, want int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ended := st.EndedSessions()
		if len(ended) >= want || time.Now().After(deadline) {
			return ended
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWebSocketPinnedInstanceUsername(t *testing.T) {
	resolver := &gatewaytest.FakeResolver{}
	srv := httptest.NewServer(newWSGateway(t, &gatewaytest.FakeStore{}, resolver, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde", InstanceID: "srv-abcdeabcdeabcdeabcde-pod01"})
	conn, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	conn.Close()
	if resolver.Username != "srv-abcdeabcdeabcdeabcde-pod01" {
		t.Errorf("pinned instance username = %q, want the compound id", resolver.Username)
	}
}

func TestWebSocketRejectsBadTickets(t *testing.T) {
	srv := httptest.NewServer(newWSGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()

	cases := map[string]string{
		"missing": "",
		"garbage": "not-a-valid-ticket",
		"wrongSign": func() string {
			tok, _ := shellticket.Mint([]byte("other-secret"), shellticket.Claims{Subject: "u", ServiceID: "srv-x", ExpiresAt: time.Now().Add(time.Minute).Unix()})
			return tok
		}(),
		"expired": func() string {
			return mintWS(t, shellticket.Claims{Subject: "u", ServiceID: "srv-x", IssuedAt: time.Now().Add(-time.Hour).Unix(), ExpiresAt: time.Now().Add(-time.Minute).Unix()})
		}(),
	}
	for name, token := range cases {
		_, resp, err := dialWS(t, srv, token)
		if err == nil {
			t.Errorf("%s: dial succeeded, want rejection", name)
			continue
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status = %v, want 401", name, resp)
		}
	}
}

func TestWebSocketRejectsReplayedTicket(t *testing.T) {
	srv := httptest.NewServer(newWSGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})

	first, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	_ = first.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`))
	for {
		if _, _, err := first.ReadMessage(); err != nil {
			break
		}
	}
	first.Close()

	_, resp, err := dialWS(t, srv, token)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("replayed ticket: status = %v, err = %v, want 401", resp, err)
	}
}

// Two Servers sharing one store model two gateway replicas behind the LB
// (w1/042 L7): a ticket redeemed on replica A must be refused on replica B —
// each replica's per-process nonce map can't see across, the shared nonce
// claim can.
func TestWebSocketRejectsReplayAcrossReplicas(t *testing.T) {
	shared := &gatewaytest.FakeStore{}
	srvA := httptest.NewServer(newWSGateway(t, shared, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srvA.Close()
	srvB := httptest.NewServer(newWSGateway(t, shared, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srvB.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})

	first, _, err := dialWS(t, srvA, token)
	if err != nil {
		t.Fatalf("dial replica A: %v", err)
	}
	_ = first.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`))
	for {
		if _, _, err := first.ReadMessage(); err != nil {
			break
		}
	}
	first.Close()

	_, resp, err := dialWS(t, srvB, token)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("cross-replica replay: status = %v, err = %v, want 401", resp, err)
	}
}

// A store outage refuses tickets (fail closed) rather than quietly degrading
// to per-replica single-use — the audit write requires the same DB anyway.
func TestWebSocketNonceStoreErrorFailsClosed(t *testing.T) {
	st := &gatewaytest.FakeStore{ClaimErr: errors.New("db down")}
	srv := httptest.NewServer(newWSGateway(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	_, resp, err := dialWS(t, srv, token)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("claim error: status = %v, err = %v, want 401", resp, err)
	}
}

// The dashboard carries the ticket in a Sec-WebSocket-Protocol entry, not the
// URL (w1/042 L8) — the gateway extracts it from the offer and selects only the
// bex.shell marker in the response, so the credential is never echoed.
func TestWebSocketTicketViaSubprotocol(t *testing.T) {
	st := &gatewaytest.FakeStore{}
	srv := httptest.NewServer(newWSGateway(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})

	dialer := websocket.Dialer{Subprotocols: []string{wsShellSubprotocol, wsTicketPrefix + token}}
	conn, resp, err := dialer.Dial(wsURL(t, srv), nil)
	if err != nil {
		t.Fatalf("subprotocol dial: %v", err)
	}
	if got := conn.Subprotocol(); got != wsShellSubprotocol {
		t.Errorf("negotiated subprotocol = %q, want %q — the ticket entry must never be selected", got, wsShellSubprotocol)
	}
	if echoed := resp.Header.Get("Sec-WebSocket-Protocol"); strings.Contains(echoed, token) {
		t.Errorf("handshake response echoes the ticket: %q", echoed)
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	conn.Close()
	if len(st.StartedSessions()) != 1 {
		t.Errorf("audit rows started = %d, want 1", len(st.StartedSessions()))
	}
}

// Query-string credentials are rejected even when the ticket itself is valid;
// accepting them would put a reusable capability in the edge's logged path.
func TestWebSocketRejectsQueryTicket(t *testing.T) {
	srv := httptest.NewServer(newWSGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})

	queryURL, err := url.Parse(wsURL(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	values := queryURL.Query()
	values.Set("ticket", token)
	queryURL.RawQuery = values.Encode()
	_, resp, err := websocket.DefaultDialer.Dial(queryURL.String(), nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("query ticket: status = %v, err = %v, want 401", resp, err)
	}
}

func TestWebSocketDisabledWithoutSecret(t *testing.T) {
	gw := &Server{Store: &gatewaytest.FakeStore{}, Apps: &gatewaytest.FakeResolver{}, Executor: &gatewaytest.FakeExecutor{}, Metrics: sshgateway.NewMetrics(prometheus.NewRegistry())}
	if gw.Enabled() {
		t.Fatal("Enabled should be false without a ticket secret")
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	_, resp, err := dialWS(t, srv, "anything")
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("disabled gateway: status = %v, err = %v, want 503", resp, err)
	}
}

func TestWebSocketPerIdentityCap(t *testing.T) {
	exec := &gatewaytest.FakeExecutor{Block: true, Started: make(chan struct{}, 1)}
	gw := newWSGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, exec)
	gw.Limits = sshgateway.NewSessionLimiter(100, 1)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	// Session 1 holds the only per-identity slot: its exec blocks until closed.
	conn1, _, err := dialWS(t, srv,
		mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"}))
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer conn1.Close()
	go func() {
		for {
			if _, _, err := conn1.ReadMessage(); err != nil {
				return
			}
		}
	}()
	select {
	case <-exec.Started:
	case <-time.After(3 * time.Second):
		t.Fatal("session 1 never reached the exec bridge")
	}

	// Session 2 (same subject, fresh ticket) must be refused by the cap with an
	// error control frame, not a second exec stream.
	conn2, _, err := dialWS(t, srv,
		mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"}))
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	defer conn2.Close()
	var refused bool
	for {
		mt, data, err := conn2.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage {
			var ctrl serverControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "error" && strings.Contains(ctrl.Message, "limit") {
				refused = true
			}
		}
	}
	if !refused {
		t.Error("second session should be refused by the per-identity session cap")
	}
}

func TestWebSocketResolveFailureSendsErrorFrame(t *testing.T) {
	st := &gatewaytest.FakeStore{}
	resolver := &gatewaytest.FakeResolver{Err: errors.New("no ready instance")}
	srv := httptest.NewServer(newWSGateway(t, st, resolver, &gatewaytest.FakeExecutor{}).Handler())
	defer srv.Close()

	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var gotError bool
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage {
			var ctrl serverControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "error" && ctrl.Message != "" {
				gotError = true
			}
		}
	}
	if !gotError {
		t.Error("expected an error control frame when the target cannot be resolved")
	}
	// A rejected target opens no session and writes no audit row.
	if started := st.StartedSessions(); len(started) != 0 {
		t.Errorf("resolve failure must not start a session: %v", started)
	}
}

// codex round-9 #6: admission-time revalidation alone leaves an admitted shell
// open until the browser disconnects or the 4h cap. The watchdog re-runs the
// target resolution (and its fresh can_operate check) on the interval and
// cancels the LIVE exec when it fails — here with the shell blocked mid-stream
// and the client sending nothing.
func TestWebSocketMidStreamRevocationEndsShell(t *testing.T) {
	st := &gatewaytest.FakeStore{}
	resolver := &gatewaytest.FakeResolver{}
	exec := &gatewaytest.FakeExecutor{
		Block:   true,
		Started: make(chan struct{}, 1),
		Stopped: make(chan error, 1),
	}
	gw := newWSGateway(t, st, resolver, exec)
	gw.RevalidateInterval = 10 * time.Millisecond
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	select {
	case <-exec.Started:
	case <-time.After(2 * time.Second):
		t.Fatal("shell exec never started")
	}

	resolver.SetFlip(errors.New("revoked"))

	select {
	case <-exec.Stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("a mid-stream revocation did not cancel the live exec")
	}
	ended := waitEndedSessions(t, st, 1)
	if len(ended) != 1 || !strings.HasSuffix(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
}
