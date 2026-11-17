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

package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestPGEnvVarSaveReadsAsRendersRowsPG pins w9/m449 on real Postgres: one env
// var save is a feed_only secrets.PatchEnvironment row and Render's per-kind
// rows. The workspace audit log reads the per-kind rows alone, with their
// keys; the service's feed still reads the save's own row.
func TestPGEnvVarSaveReadsAsRendersRowsPG(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "env-var-rows-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "envs", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	row := func(verb string, keys []string, feedOnly bool) core.AuditEvent {
		return core.AuditEvent{Caller: "fixture-owner", CallerMethod: "session", Verb: verb,
			Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID),
			Outcome: core.AuditAllowed, At: time.Now().UTC(), EnvVarKeys: keys, FeedOnly: feedOnly}
	}
	if err := st.RecordAll(ctx, []core.AuditEvent{
		row(core.AuditVerbPatchEnvironment, nil, true),
		row(core.AuditVerbCreateEnvVars, []string{"K1"}, false),
		row(core.AuditVerbUpdateEnvVars, []string{"K2", "K3"}, false),
	}); err != nil {
		t.Fatal(err)
	}

	logged, err := st.ListAuditEvents(ctx, tenant.ID, AuditFilter{OldestFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range logged {
		got = append(got, r.Verb)
		if want := map[string][]string{core.AuditVerbCreateEnvVars: {"K1"}, core.AuditVerbUpdateEnvVars: {"K2", "K3"}}[r.Verb]; !slices.Equal(r.EnvVarKeys, want) {
			t.Errorf("%s keys = %v, want %v", r.Verb, r.EnvVarKeys, want)
		}
	}
	if !slices.Equal(got, []string{core.AuditVerbCreateEnvVars, core.AuditVerbUpdateEnvVars}) {
		t.Fatalf("audit log = %v, want only Render's two rows", got)
	}

	feed, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbPatchEnvironment}})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 {
		t.Fatalf("feed rows = %+v, want the save's own row", feed)
	}
}
