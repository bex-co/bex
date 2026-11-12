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
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"k8s.io/utils/ptr"
)

// w9/092 on real Postgres: Blueprint syncs that change an existing service's
// instance count, turn autoscaling on and then off again, and add a disk land
// their instance_count_changed, autoscaling_config_changed and disk_created
// rows through the store's audit sink, by the member who synced, and the
// service's events feed lists each with Render's details. The sync that
// created the service lists none, and a re-sync that changes nothing adds
// nothing.
func TestPGBlueprintSyncSettingChangesListInTheServiceFeed(t *testing.T) {
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
	ws, err := st.CreateWorkspace(context.Background(), "bp-settings-"+id.New(id.Owner), store.PlanHobby, "bp-settings-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, ws.ID)
		_ = st.DeleteTenant(context.Background(), ws.ID)
	})
	bp, err := st.UpsertBlueprint(context.Background(), store.Blueprint{
		TenantID: ws.ID, Name: "rescale", Repo: "https://github.com/acme/" + ws.ID, Branch: "main",
		Path: CanonicalBlueprintFilename, Status: store.BlueprintStatusInSync, Manifest: "services: []\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"bp-settings-owner": ws.ID}, Audit: st},
		Store: st, Blueprints: st, DomainTraffic: allowDomainTraffic{},
	}
	feed := &events.Service{Base: svc.Base, Store: st}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "bp-settings-owner", Method: "session"})
	sync := func(manifest string) {
		t.Helper()
		if _, err := svc.SyncBlueprint(ctx, bp.ID, ws.ID, manifest, "", nil); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}
	var serviceID, appID string
	list := func() []string {
		t.Helper()
		evs, err := feed.List(ctx, serviceID, events.Filter{Types: []string{
			events.TypeInstanceCountChanged, events.TypeAutoscalingConfigChanged, events.TypeDiskCreated,
		}})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ev := range evs {
			d := ev.Details
			switch ev.Type {
			case events.TypeInstanceCountChanged:
				out = append(out, fmt.Sprintf("instance_count_changed %d->%d", ptr.Deref(d.InstanceCountFrom, -1), ptr.Deref(d.InstanceCountTo, -1)))
			case events.TypeAutoscalingConfigChanged:
				from, to := "none", "off"
				if d.AutoscalingMinFrom != nil {
					from = fmt.Sprintf("%d..%d", *d.AutoscalingMinFrom, ptr.Deref(d.AutoscalingMaxFrom, -1))
				}
				if d.AutoscalingMinTo != nil {
					to = fmt.Sprintf("%d..%d", *d.AutoscalingMinTo, ptr.Deref(d.AutoscalingMaxTo, -1))
				}
				out = append(out, "autoscaling_config_changed "+from+"->"+to)
			case events.TypeDiskCreated:
				disks, err := st.ListDisks(ctx, ws.ID, appID)
				if err != nil || len(disks) != 1 || d.DiskID != disks[0].ID {
					t.Fatalf("disk_created names disk %q, want the service's one disk %+v (err %v)", d.DiskID, disks, err)
				}
				out = append(out, fmt.Sprintf("disk_created %dGB", ptr.Deref(d.DiskSizeGB, -1)))
			}
		}
		slices.Sort(out) // a sync's rows share its instant; order is not the contract
		return out
	}

	sync(imageService + "    numInstances: 1\n")
	app := getTenantApp(t, svc.Client, ws.ID, "web")
	serviceID, appID = core.AppPublicID(app), managedAppID(app)
	if appID == "" {
		t.Fatal("want a managed App")
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("the Blueprint's create of the service listed %v, want no setting change", got)
	}
	var want []string
	for _, step := range []struct {
		manifest string
		adds     []string
	}{
		{imageService + "    numInstances: 3\n", []string{"instance_count_changed 1->3"}},
		{imageService + threeToFive, []string{"autoscaling_config_changed none->1..3"}},
		{imageService + "    numInstances: 2\n", []string{"autoscaling_config_changed 1..3->off", "instance_count_changed 3->2"}},
		{imageService + "    numInstances: 1\n    disk: {name: data, mountPath: /var/data, sizeGB: 10}\n",
			[]string{"disk_created 10GB", "instance_count_changed 2->1"}},
		{imageService + "    numInstances: 1\n    disk: {name: data, mountPath: /var/data, sizeGB: 10}\n", nil},
	} {
		sync(step.manifest)
		want = append(want, step.adds...)
		slices.Sort(want)
		if got := list(); !slices.Equal(got, want) {
			t.Fatalf("feed after syncing\n%s= %v, want %v", step.manifest, got, want)
		}
	}

	rows, err := pool.Query(ctx, `SELECT verb, caller FROM audit_events WHERE workspace_id = $1 AND verb = ANY($2)`, ws.ID,
		[]string{core.AuditVerbScale, core.AuditVerbSetAutoscaling, core.AuditVerbDeleteAutoscaling, core.AuditVerbAddDisk})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var verb, caller string
		if err := rows.Scan(&verb, &caller); err != nil {
			t.Fatal(err)
		}
		if caller != "bp-settings-owner" {
			t.Fatalf("%s row names caller %q, want the member who synced", verb, caller)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != len(want) {
		t.Fatalf("audit rows = %d, want one per listed event (%d)", n, len(want))
	}
}
