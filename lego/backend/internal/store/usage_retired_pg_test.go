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
)

// w4/m173 t007: a resource deleted before its hour is rolled up is still owed
// that hour. Every App deletion path removes its row, so the trigger retires
// it; datastores are recorded by their delete verbs. Asserted against real
// Postgres because the trigger, the cascade and the rollback are the contract.
func TestRetiredUsageResourcesPG(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	retired := func(t *testing.T) map[string]RetiredUsageResource {
		t.Helper()
		got, err := st.ListRetiredUsageResources(ctx, time.Now().Add(-48*time.Hour))
		if err != nil {
			t.Fatalf("ListRetiredUsageResources: %v", err)
		}
		out := map[string]RetiredUsageResource{}
		for _, r := range got {
			if r.WorkspaceID == tenant.ID {
				out[r.ResourceKind+"/"+r.ResourceID] = r
			}
		}
		return out
	}

	// Deleting an App row retires it with everything its meters need.
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "short-lived", Type: "web_service", Image: "nginx:1", Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	before := time.Now().Add(-time.Second)
	if err := st.DeleteApp(ctx, app.ID); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
	got, ok := retired(t)[ResourceKindService+"/"+app.ID]
	if !ok || got.Name != "short-lived" || got.Tier != "starter" || !got.CreatedAt.Equal(app.CreatedAt) || got.RetiredAt.Before(before) {
		t.Fatalf("retired App = %+v (found %v), want name short-lived, tier starter, created %v, retired after %v", got, ok, app.CreatedAt, before)
	}

	// A create rolled back never served anything: its record goes with it.
	rolled, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "never-ran", Type: "web_service", Image: "nginx:1", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if err := st.RollbackAppCreation(ctx, rolled.ID); err != nil {
		t.Fatalf("RollbackAppCreation: %v", err)
	}
	if _, ok := retired(t)[ResourceKindService+"/"+rolled.ID]; ok {
		t.Fatal("a rolled-back create was retired for metering")
	}

	// A datastore has no row; its delete verb records it, and a retried delete
	// replaces the record (here after a plan change).
	kv := RetiredUsageResource{ResourceKind: ResourceKindKeyValue, ResourceID: "red-short-" + tenant.ID, WorkspaceID: tenant.ID,
		Name: "red-short", Namespace: "tenant-ns", Display: "cache", Public: true, Tier: "free",
		CreatedAt: time.Date(2026, 10, 5, 7, 21, 27, 0, time.UTC)}
	for _, tier := range []string{"free", "starter"} {
		kv.Tier = tier
		if err := st.RecordRetiredUsageResource(ctx, kv); err != nil {
			t.Fatalf("RecordRetiredUsageResource: %v", err)
		}
	}
	got = retired(t)[ResourceKindKeyValue+"/"+kv.ResourceID]
	got.RetiredAt = time.Time{}
	if got != kv {
		t.Fatalf("retired Key Value = %+v, want %+v", got, kv)
	}

	// A deleted resource's coverage ends at its deletion hour, even after a
	// health write reopened it.
	window := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	if err := st.RecordUsageSourceHealth(ctx, []UsageSourceRecord{{WorkspaceID: tenant.ID, ResourceKind: ResourceKindKeyValue, ServiceID: kv.ResourceID,
		Kind: UsageKindInstanceSeconds, WindowStart: window, UsageSourceObservation: UsageSourceObservation{Source: UsageSourceInstance, State: UsageSourceHealthy}}}); err != nil {
		t.Fatalf("RecordUsageSourceHealth: %v", err)
	}
	if err := st.CloseUsageSourceStreams(ctx, UsageResourceRef{ResourceKind: ResourceKindKeyValue, ServiceID: kv.ResourceID}, window.Add(time.Hour)); err != nil {
		t.Fatalf("CloseUsageSourceStreams: %v", err)
	}
	var through time.Time
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = $1`, kv.ResourceID).Scan(&through); err != nil || !through.Equal(window.Add(time.Hour)) {
		t.Fatalf("stream expected_through = %v (%v), want %v", through, err, window.Add(time.Hour))
	}

	// Records past the catch-up window are pruned.
	if _, err := pool.Exec(ctx, `UPDATE usage_retired_resources SET retired_at = now() - interval '3 days' WHERE resource_id = $1`, kv.ResourceID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PruneRetiredUsageResources(ctx, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatalf("PruneRetiredUsageResources: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_retired_resources WHERE resource_id = $1`, kv.ResourceID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("a record retired three days ago survived the prune (%d, %v)", left, err)
	}

	// Deleting the workspace cascades through its Apps; the trigger must not
	// fail that delete, and nothing of the workspace is listed or kept.
	if _, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "cascaded", Type: "web_service", Image: "nginx:1", Replicas: 1}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if err := st.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatalf("DeleteTenant with Apps: %v", err)
	}
	if left := retired(t); len(left) != 0 {
		t.Fatalf("after the workspace was deleted, retired = %+v, want none listed", left)
	}
	if _, err := st.PruneRetiredUsageResources(ctx, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatalf("PruneRetiredUsageResources: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_retired_resources WHERE workspace_id = $1`, tenant.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("the deleted workspace's records survived the prune (%d, %v)", left, err)
	}
}
