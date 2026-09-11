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
	"encoding/json"
	"errors"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestProductInventoryState(t *testing.T) {
	for _, tc := range []struct {
		name, phase, want                   string
		status                              metav1.ConditionStatus
		suspended, stale, missing, deleting bool
	}{
		{name: "running", phase: "Running", status: metav1.ConditionTrue, want: "ready"},
		{name: "database ready", phase: "Ready", status: metav1.ConditionTrue, want: "ready"},
		{name: "false readiness", phase: "Running", status: metav1.ConditionFalse, want: "unknown"},
		{name: "unknown readiness", phase: "Ready", status: metav1.ConditionUnknown, want: "unknown"},
		{name: "stale readiness", phase: "Running", status: metav1.ConditionTrue, stale: true, want: "unknown"},
		{name: "stale failure", phase: "Failed", status: metav1.ConditionFalse, stale: true, want: "unknown"},
		{name: "missing readiness", phase: "Ready", missing: true, want: "unknown"},
		{name: "intentional suspension", phase: "Ready", missing: true, suspended: true, want: "suspended"},
		{name: "hibernated", phase: "Hibernated", missing: true, want: "suspended"},
		{name: "provisioning", phase: "Provisioning", status: metav1.ConditionFalse, want: "provisioning"},
		{name: "failure", phase: "Failed", status: metav1.ConditionFalse, want: "failed"},
		{name: "cancellation", phase: "Canceled", status: metav1.ConditionFalse, want: "canceled"},
		{name: "deleting still exists", phase: "Ready", deleting: true, suspended: true, want: "deleting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 2}}
			if tc.deleting {
				v := metav1.Now()
				obj.DeletionTimestamp = &v
			}
			conditions := []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: tc.status, ObservedGeneration: 2}}
			if tc.stale {
				conditions[0].ObservedGeneration = 1
			}
			if tc.missing {
				conditions = nil
			}
			if got := productInventoryState(obj, tc.phase, tc.suspended, conditions); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

type inventoryFailingList struct {
	client.Client
	failSecond bool
	calls      int
}

func (c *inventoryFailingList) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*appv1alpha1.DatabaseList); !ok {
		return c.Client.List(ctx, list, opts...)
	}
	c.calls++
	if c.failSecond && c.calls == 1 {
		list.(*appv1alpha1.DatabaseList).Items = []appv1alpha1.Database{{ObjectMeta: metav1.ObjectMeta{Name: "dpg-page-one", Labels: map[string]string{LabelTenant: "tea-test"}}}}
		list.SetContinue("page-two")
		return nil
	}
	return errors.New("inventory list unavailable")
}

func TestProductInventoryListsAreCompleteAndDeduplicated(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	legacy := readyDatabase(time.Now())
	legacy.Namespace = "bex-system"
	canonical := legacy.DeepCopy()
	canonical.Namespace = testWorkspace
	canonical.Spec.Suspended = true
	foreign := legacy.DeepCopy()
	foreign.Name = "dpg-foreign"
	foreign.Labels = map[string]string{LabelTenant: testWorkspace, ControlPlaneLabel: "dev-other"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(legacy, canonical, foreign).Build()
	c := &ProductInventoryCollector{Client: cl}
	items, err := c.listResources(context.Background(), "postgres")
	if err != nil || len(items) != 1 || items[0].State != "suspended" {
		t.Fatalf("dedup/ownership: %+v %v", items, err)
	}
	failing := &inventoryFailingList{Client: cl, failSecond: true}
	c.Client = failing
	items, err = c.listResources(context.Background(), "postgres")
	if err == nil || len(items) != 0 || failing.calls != 2 {
		t.Fatalf("partial list published: %+v %v calls=%d", items, err, failing.calls)
	}
}

// Decode like the API client: the final page omits, rather than clears, continue.
type inventoryPagedList struct {
	client.Client
	calls int
}

func (c *inventoryPagedList) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.calls++
	options := (&client.ListOptions{}).ApplyOptions(opts)
	if options.Limit != 500 {
		return errors.New("missing page limit")
	}
	switch c.calls {
	case 1:
		if options.Continue != "" {
			return errors.New("unexpected initial continuation")
		}
		return json.Unmarshal([]byte(`{"metadata":{"continue":"page-two"},"items":[{"metadata":{"name":"dpg-first","labels":{"bex.co/tenant":"tea-test"}}}]}`), list)
	case 2:
		if options.Continue != "page-two" {
			return errors.New("missing continuation")
		}
		return json.Unmarshal([]byte(`{"metadata":{},"items":[{"metadata":{"name":"dpg-second","labels":{"bex.co/tenant":"tea-test"}}}]}`), list)
	default:
		return errors.New("reused final continuation")
	}
}

func TestProductInventoryPaginationClearsFinalContinuation(t *testing.T) {
	cl := &inventoryPagedList{}
	items, err := (&ProductInventoryCollector{Client: cl}).listResources(context.Background(), "postgres")
	if err != nil || len(items) != 2 || cl.calls != 2 {
		t.Fatalf("items=%+v err=%v calls=%d", items, err, cl.calls)
	}
}

func TestProductInventoryProjectsAppsAndKeyValue(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	metadata := metav1.ObjectMeta{Name: "cron", Namespace: testWorkspace, Generation: 2,
		Labels: map[string]string{LabelTenant: testWorkspace, LabelAppID: "crn-test", ControlPlaneLabel: DefaultControlPlaneIdentity}}
	cron := &appv1alpha1.App{ObjectMeta: metadata}
	cron.Spec.Type = appv1alpha1.TypeCronJob
	cron.Status.Phase = appv1alpha1.PhaseRunning
	cron.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, ObservedGeneration: 2}}
	web := cron.DeepCopy()
	web.Name = "web"
	web.Labels[LabelAppID] = "srv-test"
	web.Spec.Type = ""
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-test", Namespace: testWorkspace,
		Generation: 2, Labels: map[string]string{LabelTenant: testWorkspace}}}
	kv.Status.Phase = appv1alpha1.KVPhaseReady
	kv.Status.Conditions = cron.Status.Conditions
	c := &ProductInventoryCollector{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cron, web, kv).Build()}
	for _, source := range []string{"services", "keyvalue"} {
		items, err := c.listResources(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		wantKinds := map[string]string{"crn-test": appv1alpha1.TypeCronJob, "srv-test": appv1alpha1.TypeWebService}
		if source == "keyvalue" {
			wantKinds = map[string]string{"kv-test": "keyvalue"}
		}
		if len(items) != len(wantKinds) {
			t.Fatalf("%s: %+v", source, items)
		}
		for _, item := range items {
			if item.Kind != wantKinds[item.ID] || item.State != "ready" || item.Workspace != testWorkspace {
				t.Fatalf("incorrect projection: %+v", item)
			}
		}
	}
}

func TestPGProductInventoryCompleteSnapshots(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour).Truncate(5 * time.Minute)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM product_inventory_batches WHERE bucket>=$1", at) })
	resource := ids.New(ids.Postgres)
	item := productInventoryResource{ID: resource, Workspace: tenant.ID, Kind: "postgres", State: "failed", CreatedAt: at.Add(-time.Minute)}
	record := func(source string, when time.Time, items []productInventoryResource) {
		t.Helper()
		if err := st.recordProductInventory(ctx, source, when, items); err != nil {
			t.Fatal(err)
		}
	}
	scalar := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	record("postgres", at, []productInventoryResource{item})
	record("postgres", at.Add(time.Second), []productInventoryResource{item})
	if n := scalar("SELECT sum(resources) FROM product_inventory_counts WHERE source='postgres' AND bucket=$1", at); n != 1 {
		t.Fatalf("unhealthy resource lost or duplicate: %d", n)
	}
	item.State = "ready"
	record("postgres", at.Add(5*time.Minute), []productInventoryResource{item})
	item.State = "failed"
	record("postgres", at.Add(10*time.Minute), []productInventoryResource{item})
	if n := scalar("SELECT count(*) FROM product_hosting_daily WHERE resource_id=$1 AND NOT live AND was_live AND observed_at=$2", resource, at.Add(10*time.Minute)); n != 1 {
		t.Fatal("inventory did not batch daily hosting, or lost previous readiness")
	}
	record("postgres", at.Add(time.Minute), nil)
	if n := scalar("SELECT count(*) FROM product_inventory_lifecycle WHERE resource_id=$1 AND first_ready_at=$2 AND removed_at IS NULL", resource, at.Add(5*time.Minute)); n != 1 {
		t.Fatal("old sample deleted resource or reset first readiness")
	}
	item.State = "invalid-state"
	if err := st.recordProductInventory(ctx, "postgres", at.Add(15*time.Minute), []productInventoryResource{item}); err == nil {
		t.Fatal("invalid sample accepted")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_batches WHERE source='postgres' AND bucket=$1", at.Add(15*time.Minute)); n != 0 {
		t.Fatal("failed transaction published a batch")
	}
	record("postgres", at.Add(20*time.Minute), nil)
	if n := scalar("SELECT count(*) FROM product_inventory_lifecycle WHERE resource_id=$1 AND removed_at=$2", resource, at.Add(20*time.Minute)); n != 1 {
		t.Fatal("complete empty inventory did not remove resource")
	}
	if n := scalar("SELECT count(*) FROM product_activity_events WHERE source_key=$1", "deleted:"+resource); n != 1 {
		t.Fatal("missing sampled removal")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_counts WHERE source='postgres' AND bucket=$1", at.Add(20*time.Minute)); n != 0 {
		t.Fatal("complete zero retains old counts")
	}

	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "inventory-app", Image: "nginx", Tier: "free", Type: "static_site", Port: 80, Replicas: 1})
	if err != nil {
		t.Fatal(err)
	}
	record("services", at, nil)
	if n := scalar("SELECT resources FROM product_inventory_counts WHERE source='services' AND bucket=$1 AND workspace_id=$2 AND state='unknown'", at, tenant.ID); n != 1 {
		t.Fatal("missing CR erased authoritative App inventory")
	}
	if _, err := pool.Exec(ctx, "UPDATE apps SET suspended=true WHERE id=$1", app.ID); err != nil {
		t.Fatal(err)
	}
	record("services", at.Add(5*time.Minute), []productInventoryResource{{ID: app.ID, Workspace: tenant.ID, State: "deleting"}})
	if n := scalar("SELECT resources FROM product_inventory_counts WHERE source='services' AND bucket=$1 AND workspace_id=$2 AND state='deleting'", at.Add(5*time.Minute), tenant.ID); n != 1 {
		t.Fatal("suspension overwrote deleting state")
	}
	domain, err := st.CreateDomain(ctx, app.ID, "inventory-"+tenant.ID+".test", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE domains SET claim_state='verified',verified_at=$2 WHERE id=$1", domain.ID, at); err != nil {
		t.Fatal(err)
	}
	if err = st.RecordProductDomainTLS(ctx, domain.ID, true, at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	record("domains", at, nil)
	if n := scalar("SELECT resources FROM product_inventory_counts WHERE source='domains' AND bucket=$1 AND workspace_id=$2 AND state='verified_tls_unknown'", at, tenant.ID); n != 1 {
		t.Fatal("stale certificate was treated as ready or absent")
	}

	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	// A later failed list must not turn a still-existing datastore into a deletion.
	item.State = "failed"
	record("postgres", at.Add(25*time.Minute), []productInventoryResource{item})
	c := &ProductInventoryCollector{Store: st, Client: &inventoryFailingList{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}}
	if err := c.collectSource(ctx, "postgres"); err == nil {
		t.Fatal("expected list failure")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_lifecycle WHERE resource_id=$1 AND removed_at IS NULL", resource); n != 1 {
		t.Fatal("failed list inferred deletion")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_batches WHERE source='postgres' AND NOT complete AND bucket>$1", at.Add(25*time.Minute)); n != 1 {
		t.Fatal("failed list not visible in coverage")
	}
	c.Client = fake.NewClientBuilder().WithScheme(scheme).Build()
	if err := c.collectSource(ctx, "postgres"); err != nil {
		t.Fatal(err)
	}
	failAgain := &inventoryFailingList{Client: c.Client}
	c.Client = failAgain
	if err := c.collectSource(ctx, "postgres"); err != nil || failAgain.calls != 0 {
		t.Fatalf("completed bucket performed redundant list: %v, calls=%d", err, failAgain.calls)
	}
	if _, err := st.PurgeProductAnalytics(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := scalar("SELECT count(*) FROM product_inventory_counts WHERE bucket=$1", at); n != 0 {
		t.Fatal("retention left old snapshot counts")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_lifecycle WHERE resource_id=$1", app.ID); n != 1 {
		t.Fatal("retention erased a current resource")
	}
	if err := st.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if n := scalar("SELECT count(*) FROM product_inventory_counts WHERE workspace_id=$1", tenant.ID); n != 0 {
		t.Fatal("workspace privacy left snapshots")
	}
	if n := scalar("SELECT count(*) FROM product_inventory_lifecycle WHERE workspace_id=$1", tenant.ID); n != 0 {
		t.Fatal("workspace privacy left lifecycle")
	}
}
