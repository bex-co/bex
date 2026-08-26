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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestPGWebhookFeedReportsDisplayNameAfterRename is w6/m101's regression: a
// renamed service's webhook deliveries carried the immutable creation-time
// apps.name because every apps-joined arm of webhookEventsQuery selected it
// raw, while the dashboard/REST/GraphQL all read through displayName. Each of
// the four arms is exercised with the same app before and after a rename, so a
// re-introduction on any single arm fails here rather than only in production
// webhook bodies. The fifth (datastore) arm has no apps join and no rename
// feature; it is asserted unchanged alongside.
func TestPGWebhookFeedReportsDisplayNameAfterRename(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "rename-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web-" + stamp, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	// CreateApp opens the app's first deploy in the same transaction; closing it
	// lights up both deploy arms (created_at feeds started, finished_at ended).
	deploys, err := st.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 {
		t.Fatalf("list deploys = %d rows (err %v), want the create deploy", len(deploys), err)
	}
	if closed, err := st.CloseDeploy(ctx, deploys[0].ID, DeployLive, "traefik/whoami"); err != nil || !closed {
		t.Fatalf("close deploy = (%v, %v)", closed, err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "alice", Verb: "apps.Restart", Resource: core.WorkspaceObject(tenant.ID),
		Target: core.ServiceTarget(core.CRName(tenant.Name, app.Name)), Outcome: core.AuditAllowed, At: at,
	}); err != nil {
		t.Fatalf("record app audit: %v", err)
	}
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "alice", Verb: core.AuditVerbPostgresCreated, Resource: core.WorkspaceObject(tenant.ID),
		Target: core.DatabaseTarget("dpg-" + stamp), TargetName: "orders", Outcome: core.AuditAllowed, At: at,
	}); err != nil {
		t.Fatalf("record datastore audit: %v", err)
	}
	if inserted, err := st.InsertServiceEventFact(ctx, ServiceEventFact{
		SourceKey: "observed:" + stamp, AppID: app.ID,
		Type: EventFactServerAvailable, At: at,
	}); err != nil || !inserted {
		t.Fatalf("insert fact = (%v, %v)", inserted, err)
	}

	verbs := []string{"apps.Restart", core.AuditVerbPostgresCreated}
	// names returns the label each arm reports, keyed by the arm it came from,
	// so a miss names the arm instead of just the row.
	names := func() map[string]string {
		t.Helper()
		rows, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().UTC().Add(time.Hour), verbs, []string{tenant.ID}, 100)
		if err != nil {
			t.Fatalf("list webhook events: %v", err)
		}
		out := map[string]string{}
		for _, r := range rows {
			arm := r.Source
			switch {
			case r.Source == EventSourceDeploy:
				arm += ":" + r.Phase
			case r.Source == EventSourceAudit && r.AppID == "":
				arm = "datastore_audit"
			}
			if prev, dup := out[arm]; dup && prev != r.ServiceName {
				t.Fatalf("arm %s reported both %q and %q", arm, prev, r.ServiceName)
			}
			out[arm] = r.ServiceName
		}
		return out
	}
	appArms := []string{EventSourceDeploy + ":" + EventPhaseStarted, EventSourceDeploy + ":" + EventPhaseEnded, EventSourceAudit, EventSourceFact}

	// Never renamed: display_name defaults to empty and every arm falls back to
	// the immutable name.
	before := names()
	for _, arm := range appArms {
		if before[arm] != app.Name {
			t.Errorf("before rename, arm %s reported %q, want the immutable name %q", arm, before[arm], app.Name)
		}
	}
	if before["datastore_audit"] != "orders" {
		t.Errorf("datastore arm reported %q, want the audit row's own target name", before["datastore_audit"])
	}

	// The public service id is derived from the immutable name and must not move
	// with the label — receivers call the API back with it.
	rows, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().UTC().Add(time.Hour), verbs, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	for _, r := range rows {
		if r.AppID != "" && r.ServiceID != core.CRName(tenant.Name, app.Name) {
			t.Errorf("%s row service id = %q, want the immutable %q", r.Source, r.ServiceID, core.CRName(tenant.Name, app.Name))
		}
	}

	if err := st.SetAppDisplayName(ctx, app.ID, "renamed-label"); err != nil {
		t.Fatalf("set display name: %v", err)
	}
	after := names()
	for _, arm := range appArms {
		if after[arm] != "renamed-label" {
			t.Errorf("after rename, arm %s reported %q, want the display label %q", arm, after[arm], "renamed-label")
		}
	}
	if after["datastore_audit"] != "orders" {
		t.Errorf("datastore arm followed the app rename: %q", after["datastore_audit"])
	}

	// Clearing the label restores the fallback — SetDisplayName("") is how a
	// service goes back to being shown under its immutable name.
	if err := st.SetAppDisplayName(ctx, app.ID, ""); err != nil {
		t.Fatalf("clear display name: %v", err)
	}
	cleared := names()
	for _, arm := range appArms {
		if cleared[arm] != app.Name {
			t.Errorf("after clearing, arm %s reported %q, want the immutable name %q", arm, cleared[arm], app.Name)
		}
	}
}

func TestPGSetAppDisplayNameMissingApp(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	if err := st.SetAppDisplayName(ctx, "srv-doesnotexist0000", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("set display name on a missing app = %v, want ErrNotFound", err)
	}
}
