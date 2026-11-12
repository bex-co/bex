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
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestPGDiskSnapshotRestoreCarriesDiskID pins w2/064: Render's
// disk_snapshot_restored requires the restored disk's id, so the audit row
// records it and both the feed and the by-id lookup read it back.
func TestPGDiskSnapshotRestoreCarriesDiskID(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "restore-disk-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "restored", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "fixture-owner", Verb: core.AuditVerbRestoreDiskSnapshot,
		Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID),
		Outcome: core.AuditAllowed, At: time.Now().UTC(), DiskID: "dsk-restored",
	}); err != nil {
		t.Fatal(err)
	}

	page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbRestoreDiskSnapshot}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].DiskID == nil || *page[0].DiskID != "dsk-restored" {
		t.Fatalf("feed rows = %+v, want one restore naming dsk-restored", page)
	}
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, page[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := lookup.Event.DiskID; got == nil || *got != "dsk-restored" {
		t.Fatalf("by-id DiskID = %v, want dsk-restored", got)
	}
}

// TestPGAuditChangeDetailsRoundTrip pins w3/m317: a disk resize, a
// maintenance-page change and toggle, and an auto-deploy disable keep the facts
// Render's members require, and both the feed and the by-id lookup read them
// back. A row recorded without them (any row from before 0222) reads nil.
func TestPGAuditChangeDetailsRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "audit-changes-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "changed", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	from, to := int32(10), int32(20)
	at := time.Now().UTC().Truncate(time.Second)
	record := func(minute int, verb string, set func(*core.AuditEvent)) {
		t.Helper()
		ev := core.AuditEvent{
			Caller: "fixture-owner", Verb: verb, Resource: core.WorkspaceObject(tenant.ID),
			Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed, At: at.Add(time.Duration(minute) * time.Minute),
		}
		set(&ev)
		if err := st.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	record(0, core.AuditVerbUpdateDisk, func(ev *core.AuditEvent) {
		ev.DiskID, ev.DiskSizeFromGB, ev.DiskSizeGB = "dsk-resized", &from, &to
	})
	record(1, core.AuditVerbMaintenanceModeURIUpdated, func(ev *core.AuditEvent) {
		ev.MaintenanceURIFrom, ev.MaintenanceURITo = new("https://status.example.com"), new("")
	})
	record(2, core.AuditVerbMaintenanceModeEnabled, func(ev *core.AuditEvent) { ev.MaintenanceModeTo = new(true) })
	record(3, core.AuditVerbSetAutoDeploy, func(ev *core.AuditEvent) {
		ev.AutoDeployEnabled = new(false)
		ev.AutoDeployTrigger, ev.AutoDeployReason = new("checksPass"), new(core.AutoDeployReasonRollback)
	})
	record(4, core.AuditVerbSetAutoDeploy, func(ev *core.AuditEvent) { ev.AutoDeployEnabled = new(false) })

	page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{
		core.AuditVerbUpdateDisk, core.AuditVerbMaintenanceModeURIUpdated,
		core.AuditVerbMaintenanceModeEnabled, core.AuditVerbSetAutoDeploy,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 5 {
		t.Fatalf("feed rows = %+v, want 5", page)
	}
	// Newest first: the bare disable, the rollback disable, toggle, URI, resize.
	if bare := page[0].AuditChanges; bare != (AuditChanges{}) {
		t.Errorf("a row without the facts = %+v, want none", bare)
	}
	checks := []struct {
		row  ServiceEventRow
		ok   bool
		what string
	}{
		{page[1], deref(page[1].AutoDeployReason) == "rollback" && deref(page[1].AutoDeployTrigger) == "checksPass", "auto-deploy reason rollback, trigger checksPass"},
		{page[2], page[2].MaintenanceModeTo != nil && *page[2].MaintenanceModeTo, "maintenance enabled"},
		{page[3], deref(page[3].MaintenanceURIFrom) == "https://status.example.com" && deref(page[3].MaintenanceURITo) == "", "maintenance URI from status.example.com to the default page"},
		{page[4], deref(page[4].DiskID) == "dsk-resized" && page[4].DiskSizeFromGB != nil && *page[4].DiskSizeFromGB == 10 &&
			page[4].DiskSizeGB != nil && *page[4].DiskSizeGB == 20, "disk dsk-resized from 10 to 20 GB"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("feed row %s = %+v, want %s", c.row.Verb, c.row, c.what)
		}
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, c.row.Key))
		if err != nil {
			t.Fatal(err)
		}
		if got := lookup.Event.AuditChanges; !reflect.DeepEqual(got, c.row.AuditChanges) {
			t.Errorf("by-id %s = %+v, want the feed's %+v", c.row.Verb, got, c.row.AuditChanges)
		}
	}
}

// TestPGAutoscalingTargetsRoundTrip pins w4/m335: an autoscaling change keeps
// both sides' CPU and memory targets, and the feed and the by-id lookup read
// them back.
func TestPGAutoscalingTargetsRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "autoscaling-targets-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "autoscaled", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	one, three, sixty, seventy := int32(1), int32(3), int32(60), int32(70)
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "fixture-owner", Verb: core.AuditVerbSetAutoscaling, Resource: core.WorkspaceObject(tenant.ID),
		Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed, At: time.Now().UTC(),
		AutoscalingMinFrom: &one, AutoscalingMaxFrom: &three, AutoscalingMinTo: &one, AutoscalingMaxTo: &three,
		AutoscalingCPUFrom: &seventy, AutoscalingCPUTo: &sixty, AutoscalingMemoryTo: &seventy,
	}); err != nil {
		t.Fatal(err)
	}

	page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbSetAutoscaling}})
	if err != nil {
		t.Fatal(err)
	}
	want := AuditChanges{AutoscalingCPUFrom: &seventy, AutoscalingCPUTo: &sixty, AutoscalingMemoryTo: &seventy}
	if len(page) != 1 || !reflect.DeepEqual(page[0].AuditChanges, want) {
		t.Fatalf("feed rows = %+v, want one carrying CPU 70 → 60 and memory → 70", page)
	}
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, page[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := lookup.Event.AuditChanges; !reflect.DeepEqual(got, want) {
		t.Fatalf("by-id changes = %+v, want %+v", got, want)
	}
}

// TestPGRenameAndDomainNamesRoundTrip pins w4/m337: a rename keeps both
// names and a custom-domain event its hostname, and the feed and the by-id
// lookup read them back.
func TestPGRenameAndDomainNamesRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "rename-domain-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "renamed", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	for i, ev := range []core.AuditEvent{
		{Verb: core.AuditVerbSetDisplayName, DisplayNameFrom: "renamed", DisplayNameTo: "Checkout API"},
		{Verb: core.AuditVerbAddDomain, Domain: "app.example.com"},
	} {
		ev.Caller, ev.Resource, ev.Target = "fixture-owner", core.WorkspaceObject(tenant.ID), core.ServiceTarget(app.ID)
		ev.Outcome, ev.At = core.AuditAllowed, at.Add(time.Duration(i)*time.Minute)
		if err := st.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbSetDisplayName, core.AuditVerbAddDomain}})
	if err != nil {
		t.Fatal(err)
	}
	// Newest first: the domain, then the rename.
	want := []AuditChanges{{Domain: new("app.example.com")}, {DisplayNameFrom: new("renamed"), DisplayNameTo: new("Checkout API")}}
	if len(page) != len(want) {
		t.Fatalf("feed rows = %+v, want %d", page, len(want))
	}
	for i, row := range page {
		if !reflect.DeepEqual(row.AuditChanges, want[i]) {
			t.Errorf("feed %s changes = %+v, want %+v", row.Verb, row.AuditChanges, want[i])
		}
		lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, row.Key))
		if err != nil {
			t.Fatal(err)
		}
		if got := lookup.Event.AuditChanges; !reflect.DeepEqual(got, want[i]) {
			t.Errorf("by-id %s changes = %+v, want %+v", row.Verb, got, want[i])
		}
	}
}

// TestPGAuditRecordWithDerivedIDRecordsOnce pins w2/067: Stripe redelivers
// checkout.session.completed, and a row carrying a derived id must land once
// however often it is recorded.
func TestPGAuditRecordWithDerivedIDRecordsOnce(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	workspace := ids.New(ids.Workspace)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, workspace)
	})
	base := time.Now().UTC().Truncate(time.Second)
	event := func(session, caller string, at time.Time) core.AuditEvent {
		return core.AuditEvent{
			ID: ids.Derive(ids.Audit, "checkout", session), Caller: caller, CallerMethod: "session",
			Verb: core.AuditVerbUpdatePaymentMethod, Resource: core.WorkspaceObject(workspace),
			Outcome: core.AuditAllowed, At: at,
		}
	}
	for _, ev := range []core.AuditEvent{
		event("cs_once", "usr-member", base), event("cs_once", "usr-member", base), event("cs_second", "usr-other", base.Add(time.Minute)),
	} {
		if err := st.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := st.ListAuditEvents(ctx, workspace, AuditFilter{OldestFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("audit rows = %+v, want one per session", rows)
	}
	if first := rows[0]; first.Verb != core.AuditVerbUpdatePaymentMethod || first.Caller != "usr-member" || first.CallerMethod != "session" {
		t.Fatalf("payment-method row = %+v", first)
	}
}

// TestPGEnvGroupLinkNamesItsGroup pins w4/563: an env-group link or unlink
// row names its group, and both the feed and the by-id lookup read it back.
func TestPGEnvGroupLinkNamesItsGroup(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "env-group-link-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "linked", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "fixture-owner", Verb: core.AuditVerbUnlinkService,
		Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID),
		Outcome: core.AuditAllowed, At: time.Now().UTC(), EnvGroupID: "evg-shared", EnvGroupName: "shared",
	}); err != nil {
		t.Fatal(err)
	}

	page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbUnlinkService}})
	if err != nil {
		t.Fatal(err)
	}
	named := func(c AuditChanges) bool {
		return c.EnvGroupID != nil && *c.EnvGroupID == "evg-shared" && c.EnvGroupName != nil && *c.EnvGroupName == "shared"
	}
	if len(page) != 1 || !named(page[0].AuditChanges) {
		t.Fatalf("feed rows = %+v, want one unlink naming evg-shared", page)
	}
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, page[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	if !named(lookup.Event.AuditChanges) {
		t.Fatalf("by-id changes = %+v, want the group named", lookup.Event.AuditChanges)
	}
}
