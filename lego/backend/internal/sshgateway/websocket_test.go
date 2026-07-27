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

package sshgateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/shellticket"
)

var wsSecret = []byte("web-shell-gateway-secret")

func newWSGateway(t *testing.T, st Store, resolver TargetResolver, exec Executor) *Server {
	t.Helper()
	return &Server{
		Store: st, Apps: resolver, Executor: exec,
		Metrics:      NewMetrics(prometheus.NewRegistry()),
		TicketSecret: wsSecret,
	}
}

func wsURL(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/shell"
}

func dialWS(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	protocols := []string{wsShellSubprotocol}
	if token != "" {
		protocols = append(protocols, wsTicketPrefix+token)
	}
	dialer := websocket.Dialer{Subprotocols: protocols}
	return dialer.Dial(wsURL(t, srv), nil)
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
	st := &fakeGatewayStore{}
	resolver := &fakeResolver{}
	exec := &fakeExecutor{}
	srv := httptest.NewServer(newWSGateway(t, st, resolver, exec).WebSocketHandler())
	defer srv.Close()

	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWS(t, srv, token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// The shared fakeExecutor reads two terminal sizes before writing output; the
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
	if !exec.tty || len(exec.command) != 1 || exec.command[0] != "/bin/sh" {
		t.Errorf("executor got tty=%v command=%v, want an interactive /bin/sh", exec.tty, exec.command)
	}
	if len(exec.sizes) == 0 || exec.sizes[len(exec.sizes)-1].Width != 100 {
		t.Errorf("resize not delivered to the exec stream: %+v", exec.sizes)
	}
	if resolver.subject != "user-1" || resolver.username != "srv-abcdeabcdeabcdeabcde" {
		t.Errorf("resolver saw subject=%q username=%q", resolver.subject, resolver.username)
	}
	if len(st.started) != 1 || len(st.ended) != 1 || !strings.HasSuffix(st.ended[0], ":completed") {
		t.Errorf("audit rows: started=%v ended=%v", st.started, st.ended)
	}
}

func TestWebSocketPinnedInstanceUsername(t *testing.T) {
	resolver := &fakeResolver{}
	srv := httptest.NewServer(newWSGateway(t, &fakeGatewayStore{}, resolver, &fakeExecutor{}).WebSocketHandler())
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
	if resolver.username != "srv-abcdeabcdeabcdeabcde-pod01" {
		t.Errorf("pinned instance username = %q, want the compound id", resolver.username)
	}
}

func TestWebSocketRejectsBadTickets(t *testing.T) {
	srv := httptest.NewServer(newWSGateway(t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
	srv := httptest.NewServer(newWSGateway(t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
// the per-process map can't see across, the shared nonce claim can.
func TestWebSocketRejectsReplayAcrossReplicas(t *testing.T) {
	shared := &fakeGatewayStore{}
	srvA := httptest.NewServer(newWSGateway(t, shared, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
	defer srvA.Close()
	srvB := httptest.NewServer(newWSGateway(t, shared, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
	st := &fakeGatewayStore{claimErr: errors.New("db down")}
	srv := httptest.NewServer(newWSGateway(t, st, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
	st := &fakeGatewayStore{}
	srv := httptest.NewServer(newWSGateway(t, st, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
	if len(st.started) != 1 {
		t.Errorf("audit rows started = %d, want 1", len(st.started))
	}
}

// Query-string credentials are rejected even when the ticket itself is valid;
// accepting them would put a reusable capability in the edge's logged path.
func TestWebSocketRejectsQueryTicket(t *testing.T) {
	srv := httptest.NewServer(newWSGateway(t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}).WebSocketHandler())
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
	gw := &Server{Store: &fakeGatewayStore{}, Apps: &fakeResolver{}, Executor: &fakeExecutor{}, Metrics: NewMetrics(prometheus.NewRegistry())}
	if gw.WebSocketEnabled() {
		t.Fatal("WebSocketEnabled should be false without a ticket secret")
	}
	srv := httptest.NewServer(gw.WebSocketHandler())
	defer srv.Close()
	_, resp, err := dialWS(t, srv, "anything")
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("disabled gateway: status = %v, err = %v, want 503", resp, err)
	}
}

func TestWebSocketPerIdentityCap(t *testing.T) {
	exec := &fakeExecutor{block: true, started: make(chan struct{}, 1)}
	gw := newWSGateway(t, &fakeGatewayStore{}, &fakeResolver{}, exec)
	gw.MaxPerIdentity = 1
	srv := httptest.NewServer(gw.WebSocketHandler())
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
	case <-exec.started:
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
	st := &fakeGatewayStore{}
	resolver := &fakeResolver{err: errors.New("no ready instance")}
	srv := httptest.NewServer(newWSGateway(t, st, resolver, &fakeExecutor{}).WebSocketHandler())
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
	if len(st.started) != 0 {
		t.Errorf("resolve failure must not start a session: %v", st.started)
	}
}
