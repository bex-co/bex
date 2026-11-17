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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

func waitShellRows(t *testing.T, audit *gatewaytest.FakeAudit, want int) []core.AuditEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(audit.Rows()) < want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return audit.Rows()
}

// An SSH session into a service instance leaves Render's StartShellEvent and
// EndShellEvent rows (w9/m451): who, which service and instance, and from
// where; no user agent (native SSH has none) and no command or output.
func TestSSHSessionRecordsStartAndEndShellEvents(t *testing.T) {
	clientSigner := signer(t)
	st, audit := &gatewaytest.FakeStore{}, &gatewaytest.FakeAudit{}
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner,
		func(s *Server) { s.Audit = audit })
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = session.CombinedOutput("printenv SECRET_TOKEN")
	_ = client.Close()
	waitEnded(t, st)

	rows := waitShellRows(t, audit, 2)
	if len(rows) != 2 || rows[0].Verb != core.AuditVerbStartShell || rows[1].Verb != core.AuditVerbEndShell {
		t.Fatalf("shell rows = %+v, want one start then one end", rows)
	}
	for _, row := range rows {
		if row.Caller != "user-1" || row.CallerMethod != core.MethodSSH || row.Outcome != core.AuditAllowed ||
			row.Resource != core.WorkspaceObject("tea-workspace") || row.Target != core.ServiceTarget("srv-abcdeabcdeabcdeabcde") ||
			row.InstanceID != "srv-abcdeabcdeabcdeabcde-pod01" || row.SourceIP != "127.0.0.1" || row.UserAgent != "" {
			t.Errorf("%s row = %+v", row.Verb, row)
		}
		if dump := fmt.Sprintf("%+v", row); strings.Contains(dump, "SECRET_TOKEN") || strings.Contains(dump, "inside-app") {
			t.Errorf("%s row carries terminal content: %s", row.Verb, dump)
		}
	}
	if rows[0].ID == "" || rows[0].ID == rows[1].ID {
		t.Errorf("row ids = %q, %q, want two distinct ids derived from the session", rows[0].ID, rows[1].ID)
	}
}

// Agent-session SSH (ags-…, ADR054) is a cloud-agent surface whose reference
// product is Devin, not a Render service shell: it records neither event.
func TestAgentSessionSSHRecordsNoShellEvents(t *testing.T) {
	clientSigner := signer(t)
	st, audit := &gatewaytest.FakeStore{}, &gatewaytest.FakeAudit{}
	resolver := &gatewaytest.FakeResolver{Target: apps.SSHInstanceTarget{
		ID: "ags-abcdeabcdeabcdeabcde", ServiceID: "ags-abcdeabcdeabcdeabcde", OwnerID: "tea-workspace",
		Namespace: "tea-workspace-sandbox", PodName: "sbx-0", Container: "sandbox", Sandbox: true,
	}}
	addr, stop := startGatewayConfigured(t, st, resolver, &gatewaytest.FakeExecutor{}, clientSigner,
		func(s *Server) { s.Audit = audit })
	defer stop()
	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = session.CombinedOutput("true")
	_ = client.Close()
	waitEnded(t, st)
	if rows := audit.Rows(); len(rows) != 0 {
		t.Fatalf("agent-session SSH recorded %+v, want no shell rows", rows)
	}
}

// refusingResolver refuses as apps.ResolveSSHSession does, recording its denial
// through the audit funnel, which fills the caller's origin from ctx.
type refusingResolver struct{ audit *gatewaytest.FakeAudit }

func (r refusingResolver) ResolveSSHSession(ctx context.Context, _ string) (apps.SSHInstanceTarget, error) {
	id, _ := core.IdentityFrom(ctx)
	core.RecordAuditEvent(ctx, r.audit, core.AuditEvent{
		Caller: id.Subject, CallerMethod: id.Method, Verb: "apps.ResolveSSHSession", Outcome: core.AuditDenied,
	})
	return apps.SSHInstanceTarget{}, core.ErrForbidden
}

// A refused identified caller leaves only the resolver's denial (the audit
// log's StartShellEvent error), carrying where they connected from; an
// unknown key records nothing at all.
func TestRefusedSSHRecordsOnlyTheDenialWithItsOrigin(t *testing.T) {
	clientSigner := signer(t)
	audit := &gatewaytest.FakeAudit{}
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, refusingResolver{audit}, &gatewaytest.FakeExecutor{}, clientSigner,
		func(s *Server) { s.Audit = audit })
	defer stop()
	if client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
		client.Close()
		t.Fatal("authentication unexpectedly succeeded")
	}
	rows := audit.Rows()
	if len(rows) == 0 {
		t.Fatal("refusal recorded nothing")
	}
	for _, row := range rows {
		if row.Outcome != core.AuditDenied || row.Caller != "user-1" || row.SourceIP != "127.0.0.1" || row.Verb == core.AuditVerbStartShell {
			t.Errorf("refusal row = %+v, want only the resolver's denial with the client address", row)
		}
	}

	unknown := &gatewaytest.FakeAudit{}
	addr, stop = startGatewayConfigured(t, &gatewaytest.FakeStore{Missing: true}, refusingResolver{unknown}, &gatewaytest.FakeExecutor{}, clientSigner,
		func(s *Server) { s.Audit = unknown })
	defer stop()
	if client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
		client.Close()
		t.Fatal("unknown key authenticated")
	}
	if rows := unknown.Rows(); len(rows) != 0 {
		t.Fatalf("unidentified connection recorded %+v", rows)
	}
}
