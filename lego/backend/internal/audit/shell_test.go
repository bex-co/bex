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

package audit

import (
	"context"
	"maps"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// shellRows are one SSH session's two rows, one web shell start and a refused
// shell's denial, as the store returns them (w9/m451).
var shellRows = []store.AuditRow{
	{
		ID: "aud-ssh-start", WorkspaceID: "tea-owner", Caller: "user-1", CallerMethod: core.MethodSSH,
		Verb: core.AuditVerbStartShell, Target: core.ServiceTarget("srv-abc"), Outcome: string(core.AuditAllowed),
		InstanceID: "srv-abc-pod01", SourceIP: "198.51.100.7",
	},
	{
		ID: "aud-ssh-end", WorkspaceID: "tea-owner", Caller: "user-1", CallerMethod: core.MethodSSH,
		Verb: core.AuditVerbEndShell, Target: core.ServiceTarget("srv-abc"), Outcome: string(core.AuditAllowed),
		InstanceID: "srv-abc-pod01", SourceIP: "198.51.100.7",
	},
	{
		ID: "aud-web-start", WorkspaceID: "tea-owner", Caller: "user-1", CallerMethod: core.MethodShell,
		Verb: core.AuditVerbStartShell, Target: core.ServiceTarget("srv-abc"), Outcome: string(core.AuditAllowed),
		InstanceID: "srv-abc-pod02", SourceIP: "198.51.100.7", UserAgent: "Mozilla/5.0",
	},
	{
		ID: "aud-refused", WorkspaceID: "tea-owner", Caller: "user-2", CallerMethod: core.MethodSSH,
		Verb: "apps.ResolveSSHSession", Target: core.ServiceTarget("srv-abc"), Outcome: string(core.AuditDenied),
		Relation: core.RelCanViewSensitive, SourceIP: "203.0.113.9",
	},
}

// A shell session reads as Render's StartShellEvent and EndShellEvent on REST,
// by a user actor, with Render's shell metadata keys service, instance,
// source_ip and (the web shell's) user_agent; a refused shell is
// StartShellEvent with status error.
func TestRESTRendersShellEventsInRendersShape(t *testing.T) {
	want := []struct {
		event, status string
		metadata      map[string]string
	}{
		{"StartShellEvent", "success", map[string]string{"service": "srv-abc", "instance": "srv-abc-pod01", "source_ip": "198.51.100.7"}},
		{"EndShellEvent", "success", map[string]string{"service": "srv-abc", "instance": "srv-abc-pod01", "source_ip": "198.51.100.7"}},
		{"StartShellEvent", "success", map[string]string{"service": "srv-abc", "instance": "srv-abc-pod02", "source_ip": "198.51.100.7", "user_agent": "Mozilla/5.0"}},
		{"StartShellEvent", "error", map[string]string{"service": "srv-abc", "source_ip": "203.0.113.9", "outcome": "denied", "relation": core.RelCanViewSensitive}},
	}
	for i, row := range shellRows {
		got := toRenderAuditLog(view(row))
		if got.Event != want[i].event || got.Status != want[i].status || got.Actor.Type != "user" {
			t.Errorf("%s: event %q status %q actor %+v, want %s %s user", row.ID, got.Event, got.Status, got.Actor, want[i].event, want[i].status)
		}
		if !maps.Equal(got.Metadata, want[i].metadata) {
			t.Errorf("%s: metadata = %v, want %v", row.ID, got.Metadata, want[i].metadata)
		}
	}
}

// GraphQL, which the dashboard's table and CSV export read, carries the same
// rows: the event, the instance and where the session came from.
func TestGraphQLCarriesTheShellEvents(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: &fakeStore{listRows: shellRows}}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ auditLogs(ownerId:"tea-owner") { id action status resource instance sourceIp userAgent } }`,
		Context:       core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: core.MethodSession}),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	rows := res.Data.(map[string]any)["auditLogs"].([]any)
	want := []map[string]any{
		{"id": "aud-ssh-start", "action": "StartShellEvent", "status": "success", "resource": "service:srv-abc", "instance": "srv-abc-pod01", "sourceIp": "198.51.100.7", "userAgent": nil},
		{"id": "aud-ssh-end", "action": "EndShellEvent", "status": "success", "resource": "service:srv-abc", "instance": "srv-abc-pod01", "sourceIp": "198.51.100.7", "userAgent": nil},
		{"id": "aud-web-start", "action": "StartShellEvent", "status": "success", "resource": "service:srv-abc", "instance": "srv-abc-pod02", "sourceIp": "198.51.100.7", "userAgent": "Mozilla/5.0"},
		{"id": "aud-refused", "action": "StartShellEvent", "status": "denied", "resource": "service:srv-abc", "instance": nil, "sourceIp": "203.0.113.9", "userAgent": nil},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v", rows)
	}
	for i, row := range rows {
		if !maps.Equal(row.(map[string]any), want[i]) {
			t.Errorf("row %d = %v, want %v", i, row, want[i])
		}
	}
}
