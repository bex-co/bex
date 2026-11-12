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

package apps

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"k8s.io/utils/ptr"
)

// w9/091 on real Postgres: a Blueprint sync that moves a cached service from a
// paid plan to Free lands its plan_changed and system edge_cache_disabled rows
// through the store's audit sink, and the service's events feed lists both
// beside the member's earlier profile save. The sync that created the service
// is no plan change, and a re-sync that keeps the plan adds nothing.
func TestPGBlueprintSyncPlanChangeListsInTheServiceFeed(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	ws, err := st.CreateWorkspace(context.Background(), "bp-plan-"+id.New(id.Owner), store.PlanHobby, "bp-plan-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, ws.ID)
		_ = st.DeleteTenant(context.Background(), ws.ID)
	})
	bp, err := st.UpsertBlueprint(context.Background(), store.Blueprint{
		TenantID: ws.ID, Name: "replan", Repo: "https://github.com/acme/" + ws.ID, Branch: "main",
		Path: CanonicalBlueprintFilename, Status: store.BlueprintStatusInSync, Manifest: "services: []\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"bp-plan-owner": ws.ID}, Audit: st},
		Store: st, Blueprints: st, DomainTraffic: allowDomainTraffic{},
	}
	feed := &events.Service{Base: svc.Base, Store: st}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "bp-plan-owner", Method: "session"})
	sync := func(manifest string) {
		t.Helper()
		if _, err := svc.SyncBlueprint(ctx, bp.ID, ws.ID, manifest, "", nil); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}
	list := func(serviceID string) []string {
		t.Helper()
		evs, err := feed.List(ctx, serviceID, events.Filter{Types: []string{
			events.TypePlanChanged, events.TypeEdgeCacheEnabled, events.TypeEdgeCacheDisabled,
		}})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ev := range evs {
			switch d := ev.Details; ev.Type {
			case events.TypePlanChanged:
				out = append(out, "plan_changed "+ptr.Deref(d.PlanFrom, "")+"->"+ptr.Deref(d.PlanTo, ""))
			default:
				trigger := "manual"
				if d.EdgeCacheTrigger != nil && d.EdgeCacheTrigger.System {
					trigger = "system"
				}
				if d.EdgeCacheTrigger == nil || d.EdgeCacheTrigger.User == nil || d.EdgeCacheTrigger.User.ID != "bp-plan-owner" {
					trigger += " without the member"
				}
				out = append(out, ev.Type+" "+trigger)
			}
		}
		slices.Sort(out) // the pair shares its instant; order is not the contract
		return out
	}

	sync(planManifest("standard"))
	app := getTenantApp(t, svc.Client, ws.ID, "web")
	serviceID := core.AppPublicID(app)
	if got := list(serviceID); len(got) != 0 {
		t.Fatalf("the Blueprint's create of the service listed %v, want no plan change", got)
	}
	if _, err := svc.SetCacheProfile(ctx, serviceID, appv1alpha1.CacheProfileStatic); err != nil {
		t.Fatal(err)
	}
	sync(planManifest("free"))
	want := []string{"edge_cache_disabled system", "edge_cache_enabled manual", "plan_changed standard->free"}
	if got := list(serviceID); !slices.Equal(got, want) {
		t.Fatalf("feed after the sync to Free = %v, want %v", got, want)
	}
	if got := getTenantApp(t, svc.Client, ws.ID, "web").Spec; got.Tier != "free" || got.CacheProfile != appv1alpha1.CacheProfileStatic {
		t.Fatalf("spec after the sync = tier %q profile %q, want free with the profile kept", got.Tier, got.CacheProfile)
	}
	sync(planManifest("free"))
	if got := list(serviceID); !slices.Equal(got, want) {
		t.Fatalf("a re-sync that keeps the plan changed the feed to %v, want %v", got, want)
	}
}
