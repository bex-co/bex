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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func seedLifecycleDB(t *testing.T, cl client.Client, name string, suspended bool) *appv1alpha1.Database {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "default",
			Generation: 3,
			Labels: map[string]string{
				core.LabelTenant:    "tea-a",
				core.LabelWorkspace: "tea-a",
			},
		},
		Spec: appv1alpha1.DatabaseSpec{
			Name:      "orders",
			Plan:      "free",
			Suspended: suspended,
		},
	}
	if err := cl.Create(context.Background(), db); err != nil {
		t.Fatalf("seed database %s: %v", name, err)
	}
	return db
}

func newPostgresMCPClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatalf("mcp server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("mcp client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func newPostgresMutationSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("graphql schema: %v", err)
	}
	return schema
}

// TestRestartSuspendedRefusedAcrossSurfaces pins w32/111: restarting a
// suspended Postgres database refuses with 409 Conflict across service, REST,
// GraphQL, and MCP without stamping spec.restartedAt, bumping updatedAt, or
// emitting a DatabaseRestarted audit event.
func TestRestartSuspendedRefusedAcrossSurfaces(t *testing.T) {
	now := time.Date(2026, 11, 24, 6, 30, 0, 0, time.UTC)

	assertUnchanged := func(t *testing.T, cl client.Client, sink *webhookAuditSink, name string) {
		t.Helper()
		var after appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &after); err != nil {
			t.Fatalf("get db: %v", err)
		}
		if after.Spec.RestartedAt != "" {
			t.Errorf("spec.restartedAt = %q, want empty", after.Spec.RestartedAt)
		}
		if after.Generation != 3 {
			t.Errorf("generation = %d, want unchanged 3", after.Generation)
		}
		if got := after.Annotations[resourcemeta.UpdatedAtAnnotation]; got != "" {
			t.Errorf("updatedAt annotation = %q, want untouched", got)
		}
		for _, ev := range sink.events {
			if ev.Verb == core.AuditVerbPostgresRestarted || ev.Outcome == core.AuditAllowed {
				t.Errorf("unexpected audit event on refused restart: %+v", ev)
			}
		}
	}

	t.Run("service", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		svc.Audit = sink
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-susp-svc", true)

		_, err := svc.Restart(context.Background(), "dpg-susp-svc")
		if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), "is suspended") {
			t.Fatalf("Restart(suspended) = %v, want ErrConflict mentioning is suspended", err)
		}
		assertUnchanged(t, cl, sink, "dpg-susp-svc")
	})

	t.Run("REST", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		svc.Audit = sink
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-susp-rest", true)

		rec := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-susp-rest/restart", "")
		if rec.Code != http.StatusConflict {
			t.Fatalf("POST /v1/postgres/dpg-susp-rest/restart => %d body=%s, want 409", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "is suspended") {
			t.Errorf("409 body = %s, want message naming suspended state", rec.Body.String())
		}
		assertUnchanged(t, cl, sink, "dpg-susp-rest")
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		svc.Audit = sink
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-susp-gql", true)

		schema := newPostgresMutationSchema(t, svc)
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: `mutation { restartDatabase(id: "dpg-susp-gql") { id } }`,
			Context:       context.Background(),
		})
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "is suspended") {
			t.Fatalf("restartDatabase errors = %+v, want single conflict error mentioning is suspended", res.Errors)
		}
		assertUnchanged(t, cl, sink, "dpg-susp-gql")
	})

	t.Run("MCP", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		svc.Audit = sink
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-susp-mcp", true)

		cs := newPostgresMCPClient(t, svc)
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "restart_postgres",
			Arguments: map[string]any{"postgresId": "dpg-susp-mcp"},
		})
		if err != nil || res == nil || !res.IsError || len(res.Content) == 0 {
			t.Fatalf("restart_postgres result=%+v err=%v, want tool error", res, err)
		}
		text, ok := res.Content[0].(*mcp.TextContent)
		if !ok || !strings.Contains(text.Text, "is suspended") {
			t.Fatalf("restart_postgres error text = %+v, want message mentioning is suspended", res.Content)
		}
		assertUnchanged(t, cl, sink, "dpg-susp-mcp")
	})
}

// TestRestartEnforcesBillingAndAllowsRunningControl verifies billing enforcement
// on Restart (with suspended winning before billing, matching ActionCapabilities)
// and that a running Postgres still restarts with 202 + DatabaseRestarted.
func TestRestartEnforcesBillingAndAllowsRunningControl(t *testing.T) {
	now := time.Date(2026, 11, 24, 6, 30, 0, 0, time.UTC)

	t.Run("billing enforced refuses running restart", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		gate := &refusingBillingGate{}
		svc.Audit = sink
		svc.Billing = gate
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-bill-run", false)

		_, err := svc.Restart(context.Background(), "dpg-bill-run")
		if !errors.Is(err, core.ErrBillingEnforced) {
			t.Fatalf("Restart(billing enforced) = %v, want ErrBillingEnforced", err)
		}
		if len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("billing gate calls = %v, want [tea-a]", gate.calls)
		}
		var after appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-bill-run"}, &after); err != nil {
			t.Fatal(err)
		}
		if after.Spec.RestartedAt != "" || len(sink.events) != 0 {
			t.Fatalf("refused restart mutated db or audited: restartedAt=%q events=%+v", after.Spec.RestartedAt, sink.events)
		}
	})

	t.Run("suspended wins over billing gate", func(t *testing.T) {
		svc, cl := newService()
		gate := &refusingBillingGate{}
		svc.Billing = gate
		seedLifecycleDB(t, cl, "dpg-bill-susp", true)

		_, err := svc.Restart(context.Background(), "dpg-bill-susp")
		if !errors.Is(err, core.ErrConflict) {
			t.Fatalf("Restart(suspended + billing enforced) = %v, want ErrConflict", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("billing gate calls = %v, want 0 when already refused as suspended", gate.calls)
		}
	})

	t.Run("running control still returns 202 and records DatabaseRestarted", func(t *testing.T) {
		svc, cl := newService()
		sink := &webhookAuditSink{}
		svc.Audit = sink
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-run-ok", false)

		rec := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-run-ok/restart", "")
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Fatalf("running restart => %d body=%q, want 202 empty", rec.Code, rec.Body.String())
		}
		var after appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-run-ok"}, &after); err != nil {
			t.Fatal(err)
		}
		if after.Spec.RestartedAt != now.Format(time.RFC3339) {
			t.Errorf("spec.restartedAt = %q, want %q", after.Spec.RestartedAt, now.Format(time.RFC3339))
		}
		found := false
		for _, ev := range sink.events {
			if ev.Verb == core.AuditVerbPostgresRestarted {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %s audit event, got %+v", core.AuditVerbPostgresRestarted, sink.events)
		}
	})
}

// TestFailoverSuspendedRefusedAcrossSurfaces pins w32/111's Failover parity:
// failing over a suspended Postgres refuses with 409 Conflict across service,
// REST, GraphQL, and MCP without stamping spec.failoverAt, while a running
// database still accepts failover.
func TestFailoverSuspendedRefusedAcrossSurfaces(t *testing.T) {
	now := time.Date(2026, 11, 24, 6, 30, 0, 0, time.UTC)

	assertUnchanged := func(t *testing.T, cl client.Client, name string) {
		t.Helper()
		var after appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &after); err != nil {
			t.Fatalf("get db: %v", err)
		}
		if after.Spec.FailoverAt != "" {
			t.Errorf("spec.failoverAt = %q, want empty", after.Spec.FailoverAt)
		}
		if after.Generation != 3 {
			t.Errorf("generation = %d, want unchanged 3", after.Generation)
		}
	}

	t.Run("service", func(t *testing.T) {
		svc, cl := newService()
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-fo-svc", true)

		err := svc.Failover(context.Background(), "dpg-fo-svc")
		if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), "is suspended") {
			t.Fatalf("Failover(suspended) = %v, want ErrConflict mentioning is suspended", err)
		}
		assertUnchanged(t, cl, "dpg-fo-svc")
	})

	t.Run("REST", func(t *testing.T) {
		svc, cl := newService()
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-fo-rest", true)

		rec := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-fo-rest/failover", "")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "is suspended") {
			t.Fatalf("POST /v1/postgres/dpg-fo-rest/failover => %d body=%s, want 409 mentioning is suspended", rec.Code, rec.Body.String())
		}
		assertUnchanged(t, cl, "dpg-fo-rest")
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, cl := newService()
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-fo-gql", true)

		schema := newPostgresMutationSchema(t, svc)
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: `mutation { failoverDatabase(id: "dpg-fo-gql") }`,
			Context:       context.Background(),
		})
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "is suspended") {
			t.Fatalf("failoverDatabase errors = %+v, want single conflict error mentioning is suspended", res.Errors)
		}
		assertUnchanged(t, cl, "dpg-fo-gql")
	})

	t.Run("MCP", func(t *testing.T) {
		svc, cl := newService()
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-fo-mcp", true)

		cs := newPostgresMCPClient(t, svc)
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "failover_postgres",
			Arguments: map[string]any{"postgresId": "dpg-fo-mcp"},
		})
		if err != nil || res == nil || !res.IsError || len(res.Content) == 0 {
			t.Fatalf("failover_postgres result=%+v err=%v, want tool error", res, err)
		}
		text, ok := res.Content[0].(*mcp.TextContent)
		if !ok || !strings.Contains(text.Text, "is suspended") {
			t.Fatalf("failover_postgres error text = %+v, want message mentioning is suspended", res.Content)
		}
		assertUnchanged(t, cl, "dpg-fo-mcp")
	})

	t.Run("running control still stamps failoverAt", func(t *testing.T) {
		svc, cl := newService()
		svc.Clock = func() time.Time { return now }
		seedLifecycleDB(t, cl, "dpg-fo-ok", false)

		rec := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-fo-ok/failover", "")
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Fatalf("running failover => %d body=%q, want 202 empty", rec.Code, rec.Body.String())
		}
		var after appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-fo-ok"}, &after); err != nil {
			t.Fatal(err)
		}
		if after.Spec.FailoverAt != now.Format(time.RFC3339) {
			t.Errorf("spec.failoverAt = %q, want %q", after.Spec.FailoverAt, now.Format(time.RFC3339))
		}
	})
}

// TestIdempotentSuspendResumePreservesViewMetadata pins w32/111's setSuspended
// fix: idempotent Suspend/Resume (already in the target state) must return
// s.view(d) rather than pgView(d) so Region and DashboardURL stay populated on
// service, GraphQL, and MCP responses.
func TestIdempotentSuspendResumePreservesViewMetadata(t *testing.T) {
	svc, cl := newService()
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}
	seedLifecycleDB(t, cl, "dpg-idem-susp", true)
	seedLifecycleDB(t, cl, "dpg-idem-run", false)

	wantRegion := "frankfurt"
	wantSuspURL := "https://dashboard.bex.co/d/dpg-idem-susp"
	wantRunURL := "https://dashboard.bex.co/d/dpg-idem-run"

	vSusp, err := svc.Suspend(context.Background(), "dpg-idem-susp")
	if err != nil {
		t.Fatalf("idempotent Suspend: %v", err)
	}
	if vSusp.Region != wantRegion || vSusp.DashboardURL != wantSuspURL {
		t.Errorf("idempotent Suspend metadata = (%q, %q), want (%q, %q)", vSusp.Region, vSusp.DashboardURL, wantRegion, wantSuspURL)
	}

	vRun, err := svc.Resume(context.Background(), "dpg-idem-run")
	if err != nil {
		t.Fatalf("idempotent Resume: %v", err)
	}
	if vRun.Region != wantRegion || vRun.DashboardURL != wantRunURL {
		t.Errorf("idempotent Resume metadata = (%q, %q), want (%q, %q)", vRun.Region, vRun.DashboardURL, wantRegion, wantRunURL)
	}

	// GraphQL suspendDatabase / resumeDatabase idempotent no-op
	schema := newPostgresMutationSchema(t, svc)
	for _, tc := range []struct {
		query   string
		field   string
		wantURL string
	}{
		{`mutation { suspendDatabase(id: "dpg-idem-susp") { id region dashboardUrl } }`, "suspendDatabase", wantSuspURL},
		{`mutation { resumeDatabase(id: "dpg-idem-run") { id region dashboardUrl } }`, "resumeDatabase", wantRunURL},
	} {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: tc.query, Context: context.Background()})
		if len(res.Errors) != 0 {
			t.Fatalf("%s errors: %+v", tc.field, res.Errors)
		}
		data, _ := res.Data.(map[string]any)
		row, _ := data[tc.field].(map[string]any)
		if row["region"] != wantRegion || row["dashboardUrl"] != tc.wantURL {
			t.Errorf("GraphQL %s = %+v, want region=%q dashboardUrl=%q", tc.field, row, wantRegion, tc.wantURL)
		}
	}

	// MCP suspend_postgres / resume_postgres idempotent no-op
	cs := newPostgresMCPClient(t, svc)
	for _, tc := range []struct {
		tool    string
		id      string
		wantURL string
	}{
		{"suspend_postgres", "dpg-idem-susp", wantSuspURL},
		{"resume_postgres", "dpg-idem-run", wantRunURL},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      tc.tool,
			Arguments: map[string]any{"postgresId": tc.id},
		})
		if err != nil || res == nil || res.IsError || len(res.Content) == 0 {
			t.Fatalf("MCP %s failed: res=%+v err=%v", tc.tool, res, err)
		}
		text, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("MCP %s non-text content: %+v", tc.tool, res.Content)
		}
		var view PostgresView
		if err := json.Unmarshal([]byte(text.Text), &view); err != nil {
			t.Fatalf("MCP %s unmarshal %q: %v", tc.tool, text.Text, err)
		}
		if view.Region != wantRegion || view.DashboardURL != tc.wantURL {
			t.Errorf("MCP %s metadata = (%q, %q), want (%q, %q)", tc.tool, view.Region, view.DashboardURL, wantRegion, tc.wantURL)
		}
	}
}
