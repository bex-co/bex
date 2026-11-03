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
