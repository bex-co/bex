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
	// A later close never stretches a stream that already ended earlier (a
	// disk deleted before its App), which would expect hours no meter writes
	// (w6/m334).
	if err := st.CloseUsageSourceStreams(ctx, UsageResourceRef{ResourceKind: ResourceKindKeyValue, ServiceID: kv.ResourceID}, window.Add(3*time.Hour)); err != nil {
		t.Fatalf("CloseUsageSourceStreams later: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = $1`, kv.ResourceID).Scan(&through); err != nil || !through.Equal(window.Add(time.Hour)) {
		t.Fatalf("a later close moved expected_through to %v (%v), want it kept at %v", through, err, window.Add(time.Hour))
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

// w6/m333: the egress identity the live meter records for an App outlives the
// App's row, so the retired pass can meter its deletion hour's egress. An App
// whose identity was never resolved retires with none; one with no routers
// keeps "known, empty". The identity goes when its retired record is pruned.
func TestRetiredAppCarriesItsEgressIdentityPG(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	create := func(name string) App {
		t.Helper()
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Type: "web_service", Image: "nginx:1", Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatalf("CreateApp %s: %v", name, err)
		}
		return app
	}
	web, unresolved, private := create("egress-web"), create("egress-unresolved"), create("egress-private")
	identity := EgressIdentity{Routers: []string{"default-a-web@kubernetes", "web-default-a-web@kubernetes"}, Direct: true}
	if err := st.RecordAppEgressIdentity(ctx, web.ID, identity); err != nil {
		t.Fatalf("RecordAppEgressIdentity: %v", err)
	}
	// An unchanged identity is not rewritten every pass.
	version := func() string {
		t.Helper()
		var xmin string
		if err := pool.QueryRow(ctx, `SELECT xmin::text FROM usage_egress_identities WHERE service_id = $1`, web.ID).Scan(&xmin); err != nil {
			t.Fatal(err)
		}
		return xmin
	}
	before := version()
	if err := st.RecordAppEgressIdentity(ctx, web.ID, identity); err != nil {
		t.Fatalf("RecordAppEgressIdentity again: %v", err)
	}
	if after := version(); after != before {
		t.Fatalf("an unchanged identity rewrote its row (xmin %s -> %s)", before, after)
	}
	if err := st.RecordAppEgressIdentity(ctx, private.ID, EgressIdentity{Direct: true}); err != nil {
		t.Fatalf("RecordAppEgressIdentity private: %v", err)
	}
	for _, app := range []App{web, unresolved, private} {
		if err := st.DeleteApp(ctx, app.ID); err != nil {
			t.Fatalf("DeleteApp %s: %v", app.Name, err)
		}
	}
	list, err := st.ListRetiredUsageResources(ctx, time.Now().Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("ListRetiredUsageResources: %v", err)
	}
	got := map[string]*EgressIdentity{}
	for _, r := range list {
		got[r.ResourceID] = r.Egress
	}
	if e := got[web.ID]; e == nil || !e.Direct || !slices.Equal(e.Routers, identity.Routers) {
		t.Fatalf("retired web service egress = %+v, want %+v", e, identity)
	}
	if e, ok := got[unresolved.ID]; !ok || e != nil {
		t.Fatalf("never-resolved service egress = %+v (listed %v), want nil", e, ok)
	}
	if e := got[private.ID]; e == nil || len(e.Routers) != 0 || !e.Direct {
		t.Fatalf("no-router service egress = %+v, want known with no routers and direct egress", e)
	}

	// Pruning the retired records takes their identities with them.
	if _, err := pool.Exec(ctx, `UPDATE usage_retired_resources SET retired_at = now() - interval '3 days' WHERE resource_id = ANY($1)`, []string{web.ID, private.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PruneRetiredUsageResources(ctx, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatalf("PruneRetiredUsageResources: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_egress_identities WHERE service_id = ANY($1)`, []string{web.ID, private.ID}).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d identities left after their records were pruned (%v), want 0", left, err)
	}
}

// w6/m333 t004: once a deleted service's last hour of egress is metered and its
// streams close, the month's coverage names no egress source degraded.
func TestDeletedServiceEgressCoverageReadsHealthyPG(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	service := "srv-gone-" + tenant.ID
	hour := time.Now().UTC().Truncate(time.Hour).Add(-72 * time.Hour)
	if err := st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: service,
		Kind: UsageKindEgressBytes, WindowStart: hour, Quantity: 42,
		SourceHealth: []UsageSourceObservation{
			{Source: UsageSourceHTTP, State: UsageSourceHealthy, ExpectedFrom: hour},
			{Source: UsageSourceWebSocket, State: UsageSourceHealthy, ExpectedFrom: hour},
			{Source: UsageSourceDirect, State: UsageSourceHealthy, ExpectedFrom: hour},
		},
	}); err != nil {
		t.Fatalf("UpsertUsageHourly: %v", err)
	}
	if err := st.CloseUsageSourceStreams(ctx, UsageResourceRef{ResourceKind: ResourceKindService, ServiceID: service}, hour.Add(time.Hour)); err != nil {
		t.Fatalf("CloseUsageSourceStreams: %v", err)
	}
	coverage, err := st.CurrentUsageCoverage(ctx, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("CurrentUsageCoverage: %v", err)
	}
	for _, source := range []string{UsageSourceHTTP, UsageSourceWebSocket, UsageSourceDirect} {
		if slices.Contains(coverage.DegradedSources, source) {
			t.Errorf("coverage %+v names %s degraded after the deleted service's last hour was metered", coverage, source)
		}
	}
}

// w4/637: a deleted App that never recorded an egress identity is not owed its
// egress, so no meter will fill an egress hole it left inside the catch-up
// window. Coverage passes that hole at once, naming the source degraded,
// instead of holding the workspace's `through` back until it settles 48 h
// later; a hole the App is still owed keeps holding it.
func TestCoveragePassesADeletedAppsUnownedEgressHolePG(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "never-served", Type: "web_service", Image: "nginx:1", Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	now := time.Date(2001, 1, 15, 12, 30, 0, 0, time.UTC)
	latest := now.Truncate(time.Hour)
	from, deleted := latest.Add(-6*time.Hour), latest.Add(-2*time.Hour)
	hole := latest.Add(-4 * time.Hour)
	record := func(kind, source string) {
		t.Helper()
		for w := from; !w.After(deleted); w = w.Add(time.Hour) {
			if w.Equal(hole) {
				continue
			}
			if err := st.RecordUsageSourceHealth(ctx, []UsageSourceRecord{{WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: app.ID,
				Kind: kind, WindowStart: w, UsageSourceObservation: UsageSourceObservation{Source: source, State: UsageSourceHealthy, ExpectedFrom: from}}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	ref := UsageResourceRef{ResourceKind: ResourceKindService, ServiceID: app.ID}
	closeAtDeletion := func() {
		t.Helper()
		if err := st.CloseUsageSourceStreams(ctx, ref, deleted.Add(time.Hour)); err != nil {
			t.Fatalf("CloseUsageSourceStreams: %v", err)
		}
	}
	record(UsageKindEgressBytes, UsageSourceHTTP)
	if err := st.DeleteApp(ctx, app.ID); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE usage_retired_resources SET retired_at = $2 WHERE resource_id = $1`, app.ID, deleted.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	closeAtDeletion()

	got, err := st.CurrentUsageCoverage(ctx, tenant.ID, now)
	if err != nil {
		t.Fatalf("CurrentUsageCoverage: %v", err)
	}
	if !got.Through.Equal(latest) || got.Complete || !slices.Equal(got.DegradedSources, []string{UsageSourceHTTP}) {
		t.Fatalf("an unowed egress hole: %+v, want through %v with http degraded", got, latest)
	}

	// Its instance hours are owed (the retired pass meters them), so the same
	// hole there holds `through` until a meter fills it or it settles.
	record(UsageKindInstanceSeconds, UsageSourceInstance)
	closeAtDeletion()
	if got, err = st.CurrentUsageCoverage(ctx, tenant.ID, now); err != nil {
		t.Fatalf("CurrentUsageCoverage: %v", err)
	}
	if !got.Through.Equal(hole) {
		t.Fatalf("an owed instance hole: through %v, want %v", got.Through, hole)
	}
}
