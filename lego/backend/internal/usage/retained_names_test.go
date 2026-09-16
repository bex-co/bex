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

package usage

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w2/m96: a billing line outlives the resource it bills for. Before the
// retained-name record a deleted service's charges collapsed to a bare `srv-…`
// id and a sandbox, which has no name at all, was always a bare UUID.

// meteredStore seeds one instance-seconds row per (kind, id) so each becomes a
// ServiceUsage the resolver must name.
func meteredStore(t *testing.T, tenant string, refs []store.ResourceDisplayName, apps ...store.App) *memUsageStore {
	t.Helper()
	st := newMemUsageStore(apps...)
	for _, ref := range refs {
		if err := st.UpsertUsageHourly(context.Background(), store.HourlyRow{
			WorkspaceID: tenant, ServiceID: ref.ID, ResourceKind: ref.Kind,
			Kind: store.UsageKindInstanceSeconds, Tier: "starter",
			WindowStart: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC),
			Quantity:    3600,
		}); err != nil {
			t.Fatalf("seed %s/%s: %v", ref.Kind, ref.ID, err)
		}
	}
	return st
}

func namedByID(t *testing.T, summary Summary) map[string]ServiceUsage {
	t.Helper()
	out := make(map[string]ServiceUsage, len(summary.Services))
	for _, svc := range summary.Services {
		out[svc.ServiceID] = svc
	}
	return out
}

func monthToDate(t *testing.T, svc *Service) Summary {
	t.Helper()
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})
	summary, err := svc.MonthToDate(ctx, "")
	if err != nil {
		t.Fatalf("MonthToDate: %v", err)
	}
	return summary
}

// The headline case from w1/088: a deleted service keeps the name its charges
// accrued under, and says it is gone.
func TestDeletedResourceKeepsItsRetainedName(t *testing.T) {
	const tenant = "tea-001"
	live := store.App{ID: "srv-live", TenantID: tenant, Name: "still-here", Tier: "starter"}
	st := meteredStore(t, tenant, []store.ResourceDisplayName{
		{Kind: store.ResourceKindService, ID: "srv-live"},
		{Kind: store.ResourceKindService, ID: "srv-gone"},
		{Kind: store.ResourceKindPostgres, ID: "dpg-gone"},
		{Kind: store.ResourceKindService, ID: "srv-never-named"},
	}, live)
	st.retained = map[string]map[string]string{tenant: {
		store.ResourceDisplayNameKey(store.ResourceKindService, "srv-gone"):  "checkout-api",
		store.ResourceDisplayNameKey(store.ResourceKindPostgres, "dpg-gone"): "orders-db",
	}}

	got := namedByID(t, monthToDate(t, svcWithTenant(st, tenant)))

	for _, tc := range []struct {
		id      string
		name    string
		deleted bool
	}{
		{"srv-live", "still-here", false},
		{"srv-gone", "checkout-api", true},
		{"dpg-gone", "orders-db", true},
		{"srv-never-named", "", false},
	} {
		svc, ok := got[tc.id]
		if !ok {
			t.Fatalf("%s missing from the summary", tc.id)
		}
		if svc.ServiceName != tc.name || svc.Deleted != tc.deleted {
			t.Errorf("%s = %q deleted=%v, want %q deleted=%v",
				tc.id, svc.ServiceName, svc.Deleted, tc.name, tc.deleted)
		}
	}
}

// A sandbox has no name, so it is labelled from the agent session that owns (or
// owned) it — which is what names the UUIDs no live sandbox query lists.
func TestSandboxRowsAreLabelledNotBareUUIDs(t *testing.T) {
	const tenant = "tea-001"
	st := meteredStore(t, tenant, []store.ResourceDisplayName{
		{Kind: store.ResourceKindSandbox, ID: "271ec9ce-a32b-4128-bb43-02a9e57b01b6"},
		{Kind: store.ResourceKindSandbox, ID: "reaped-with-retained-name"},
		{Kind: store.ResourceKindSandbox, ID: "unknowable"},
	})
	st.sandboxLabels = map[string]string{
		"271ec9ce-a32b-4128-bb43-02a9e57b01b6": "bex-co/bex (main)",
	}
	st.retained = map[string]map[string]string{tenant: {
		store.ResourceDisplayNameKey(store.ResourceKindSandbox, "reaped-with-retained-name"): "acme/site (fix-nav)",
	}}

	got := namedByID(t, monthToDate(t, svcWithTenant(st, tenant)))

	if svc := got["271ec9ce-a32b-4128-bb43-02a9e57b01b6"]; svc.ServiceName != "bex-co/bex (main)" || svc.Deleted {
		t.Errorf("session-owned sandbox = %q deleted=%v, want the repo label and deleted=false", svc.ServiceName, svc.Deleted)
	}
	if svc := got["reaped-with-retained-name"]; svc.ServiceName != "acme/site (fix-nav)" || !svc.Deleted {
		t.Errorf("reaped sandbox = %q deleted=%v, want the retained label and deleted=true", svc.ServiceName, svc.Deleted)
	}
	if svc := got["unknowable"]; svc.ServiceName != "" || svc.Deleted {
		t.Errorf("sandbox bex never knew = %q deleted=%v, want empty and deleted=false", svc.ServiceName, svc.Deleted)
	}
}

// The usage read is the capture point: it enumerates every metered resource, so
// a name recorded here cannot miss a create path.
func TestUsageReadRetainsLiveNamesAndSkipsUnchangedOnes(t *testing.T) {
	const tenant = "tea-001"
	renamed := store.App{ID: "srv-renamed", TenantID: tenant, Name: "new-name", Tier: "starter"}
	same := store.App{ID: "srv-same", TenantID: tenant, Name: "unchanged", Tier: "starter"}
	st := meteredStore(t, tenant, []store.ResourceDisplayName{
		{Kind: store.ResourceKindService, ID: "srv-renamed"},
		{Kind: store.ResourceKindService, ID: "srv-same"},
	}, renamed, same)
	st.retained = map[string]map[string]string{tenant: {
		store.ResourceDisplayNameKey(store.ResourceKindService, "srv-renamed"): "old-name",
		store.ResourceDisplayNameKey(store.ResourceKindService, "srv-same"):    "unchanged",
	}}

	got := namedByID(t, monthToDate(t, svcWithTenant(st, tenant)))

	// The live name wins over the stale retained one.
	if svc := got["srv-renamed"]; svc.ServiceName != "new-name" || svc.Deleted {
		t.Errorf("renamed service = %q deleted=%v, want new-name and deleted=false", svc.ServiceName, svc.Deleted)
	}
	if len(st.recorded) != 1 {
		t.Fatalf("recorded %+v, want exactly the changed name", st.recorded)
	}
	if st.recorded[0].ID != "srv-renamed" || st.recorded[0].Name != "new-name" {
		t.Errorf("recorded = %+v, want srv-renamed -> new-name", st.recorded[0])
	}
	if want := "new-name"; st.retained[tenant][store.ResourceDisplayNameKey(store.ResourceKindService, "srv-renamed")] != want {
		t.Errorf("retained record was not moved forward to %q", want)
	}
}

// Names travel from the summary onto the per-resource estimates the Charges
// card actually renders, deleted flag included.
func TestResourceEstimatesCarryTheNameAndDeletedFlag(t *testing.T) {
	const tenant = "tea-001"
	st := meteredStore(t, tenant, []store.ResourceDisplayName{
		{Kind: store.ResourceKindService, ID: "srv-gone"},
	})
	st.retained = map[string]map[string]string{tenant: {
		store.ResourceDisplayNameKey(store.ResourceKindService, "srv-gone"): "checkout-api",
	}}

	summary := monthToDate(t, svcWithTenant(st, tenant))
	var found bool
	for _, r := range summary.EstimatedCost.Resources {
		if r.ServiceID != "srv-gone" {
			continue
		}
		found = true
		if r.ServiceName != "checkout-api" || !r.Deleted {
			t.Errorf("estimate = %q deleted=%v, want checkout-api and deleted=true", r.ServiceName, r.Deleted)
		}
	}
	if !found {
		t.Fatalf("srv-gone missing from %d resource estimates", len(summary.EstimatedCost.Resources))
	}
}
