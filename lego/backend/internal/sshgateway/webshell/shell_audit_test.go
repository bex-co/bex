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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 Safari/605.1.15"

// runShell opens one dashboard Shell session as user-1 from a browser, sends a
// window size, and waits for the session to end.
func runShell(t *testing.T, gw *Server, st *gatewaytest.FakeStore) {
	t.Helper()
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWSHeaders(t, srv, token, http.Header{"User-Agent": []string{browserUA}})
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
	waitEndedSessions(t, st, 1)
}

// A dashboard Shell session leaves Render's StartShellEvent and EndShellEvent
// rows (w9/m451): the person who opened it, the service and instance, where
// the browser connected from and its user agent, and nothing typed into it.
func TestWebShellRecordsStartAndEndShellEvents(t *testing.T) {
	st, audit := &gatewaytest.FakeStore{}, &gatewaytest.FakeAudit{}
	gw := newWSGateway(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{})
	gw.Audit = audit
	runShell(t, gw, st)

	rows := audit.Rows()
	if len(rows) != 2 || rows[0].Verb != core.AuditVerbStartShell || rows[1].Verb != core.AuditVerbEndShell {
		t.Fatalf("shell rows = %+v, want one start then one end", rows)
	}
	sessionID := st.StartedSessions()[0].ID
	if transport := st.StartedSessions()[0].Transport; transport != store.SSHTransportWebShell {
		t.Errorf("session transport = %q, want %q", transport, store.SSHTransportWebShell)
	}
	for _, row := range rows {
		if row.Caller != "user-1" || row.CallerMethod != core.MethodShell || row.Outcome != core.AuditAllowed ||
			row.Resource != core.WorkspaceObject("tea-workspace") || row.Target != core.ServiceTarget("srv-abcdeabcdeabcdeabcde") ||
			row.InstanceID != "srv-abcdeabcdeabcdeabcde-pod01" || row.SourceIP != "127.0.0.1" || row.UserAgent != browserUA {
			t.Errorf("%s row = %+v", row.Verb, row)
		}
		// Content-free (ADR035): no session output or command.
		if dump := fmt.Sprintf("%+v", row); strings.Contains(dump, "inside-app") || strings.Contains(dump, "/bin/sh") {
			t.Errorf("%s row carries terminal content: %s", row.Verb, dump)
		}
		if row.ID == "" || row.ID == sessionID {
			t.Errorf("%s row id = %q, want one derived from the session", row.Verb, row.ID)
		}
	}
	if rows[0].ID == rows[1].ID {
		t.Errorf("start and end share id %q", rows[0].ID)
	}
}

// refusingResolver refuses as apps.ResolveSSHSession does, recording its denial
// through the audit funnel (core.RecordAuditEvent), which fills where the caller
// connected from out of ctx.
type refusingResolver struct{ audit *gatewaytest.FakeAudit }

func (r refusingResolver) ResolveSSHSession(ctx context.Context, _ string) (apps.SSHInstanceTarget, error) {
	id, _ := core.IdentityFrom(ctx)
	core.RecordAuditEvent(ctx, r.audit, core.AuditEvent{
		Caller: id.Subject, CallerMethod: id.Method, Verb: "apps.ResolveSSHSession", Outcome: core.AuditDenied,
	})
	return apps.SSHInstanceTarget{}, core.ErrForbidden
}

// A shell the gateway refuses records only the resolver's denial (the audit
// log's StartShellEvent error), and that row says where the browser was.
func TestWebShellRefusalRecordsTheDenialWithItsOrigin(t *testing.T) {
	st, audit := &gatewaytest.FakeStore{}, &gatewaytest.FakeAudit{}
	gw := newWSGateway(t, st, refusingResolver{audit}, &gatewaytest.FakeExecutor{})
	gw.Audit = audit
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	token := mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"})
	conn, _, err := dialWSHeaders(t, srv, token, http.Header{"User-Agent": []string{browserUA}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	conn.Close()

	rows := audit.Rows()
	if len(rows) != 1 || rows[0].Outcome != core.AuditDenied || rows[0].SourceIP != "127.0.0.1" || rows[0].UserAgent != browserUA {
		t.Fatalf("refusal rows = %+v, want the one denial with the browser's origin", rows)
	}
}
