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

package workspaces

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- ResourceLimits: terminating vs. active reconciliation (w6/m129) --------
//
// The defect m129 fixes: the counter counted every App/Database/KeyValue CR
// (11) while the resource list dropped the ones finishing deletion (6), and no
// surface bridged the two. These tests set a real DeletionTimestamp on a CR —
// the fixture shape no earlier test had — and pin the reconciliation identity
// Used - Terminating == what the list shows, so the two can never silently
// diverge again.

func limitsScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return scheme
}

// limitsSvc builds a Service with an allow-all checker, the given store, and a
// live fake k8s client (so ResourceLimits actually counts CRs). Seed CRs into
// the returned client with seedActive / seedTerminating.
func limitsSvc(t *testing.T, st WorkspaceStore) (*Service, client.Client) {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(limitsScheme(t)).Build()
	return &Service{
		Base: &core.Base{
			Authz:     &fakeChecker{allow: true},
			Workspace: &fakeResolver{store: st},
			Client:    cl,
		},
		Store:   st,
		Granter: &fakeGranter{},
	}, cl
}

func tenantMeta(name, tenantID string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: tenantID,
		Labels:    map[string]string{core.LabelTenant: tenantID},
	}
}

func appObj(name, tenantID string) *appv1alpha1.App {
	return &appv1alpha1.App{ObjectMeta: tenantMeta(name, tenantID)}
}

func dbObj(name, tenantID string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: tenantMeta(name, tenantID)}
}

func kvObj(name, tenantID string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{ObjectMeta: tenantMeta(name, tenantID)}
}

func seedActive(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	if err := cl.Create(context.Background(), obj); err != nil {
		t.Fatalf("seed active %T %s: %v", obj, obj.GetName(), err)
	}
}

// seedTerminating creates a CR with a finalizer and then deletes it: the fake
// client stamps a DeletionTimestamp and — because a finalizer holds it — retains
// the object in exactly the mid-teardown state a stuck delete leaves behind.
func seedTerminating(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	obj.SetFinalizers([]string{"app.bex.co/finalizer"})
	if err := cl.Create(context.Background(), obj); err != nil {
		t.Fatalf("seed terminating (create) %T %s: %v", obj, obj.GetName(), err)
	}
	if err := cl.Delete(context.Background(), obj); err != nil {
		t.Fatalf("seed terminating (delete) %T %s: %v", obj, obj.GetName(), err)
	}
}

// seedFinalized creates a CR without a finalizer and deletes it: the fake client
// removes it outright, the fully-finalized state that must leave BOTH surfaces.
func seedFinalized(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	if err := cl.Create(context.Background(), obj); err != nil {
		t.Fatalf("seed finalized (create) %T %s: %v", obj, obj.GetName(), err)
	}
	if err := cl.Delete(context.Background(), obj); err != nil {
		t.Fatalf("seed finalized (delete) %T %s: %v", obj, obj.GetName(), err)
	}
}

// assertReconciles pins m129's core invariant for one resource kind: Used counts
// every present CR (== what the ResourceQuota gates creates on), Terminating is
// the deleting subset, and Used - Terminating is the count the resource list
// shows. wantListed is that reconciled count (active + suspended, never
// terminating).
func assertReconciles(t *testing.T, kind string, cap ResourceCapView, wantUsed, wantTerminating, wantListed int) {
	t.Helper()
	if cap.Used != wantUsed {
		t.Errorf("%s Used = %d, want %d", kind, cap.Used, wantUsed)
	}
	if cap.Terminating != wantTerminating {
		t.Errorf("%s Terminating = %d, want %d", kind, cap.Terminating, wantTerminating)
	}
	if got := cap.Used - cap.Terminating; got != wantListed {
		t.Errorf("%s Used-Terminating = %d, want %d (the count the resource list shows)", kind, got, wantListed)
	}
}

func TestResourceLimits_TerminatingHeldInUsedButReportedSeparately(t *testing.T) {
	st := newFakeStore()
	svc, cl := limitsSvc(t, st)
	w, err := svc.Create(ctxAs("user-a"), "acme", "pro")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Services: 2 running + 1 suspended (still counts, still listed) + 1
	// terminating + 1 fully finalized (gone from both surfaces).
	seedActive(t, cl, appObj("web", w.ID))
	seedActive(t, cl, appObj("api", w.ID))
	suspended := appObj("paused", w.ID)
	suspended.Spec.Suspended = true
	seedActive(t, cl, suspended)
	seedTerminating(t, cl, appObj("gone", w.ID))
	seedFinalized(t, cl, appObj("erased", w.ID))

	// Postgres + Key Value: 1 active + 1 terminating each — the Hobby-cap-of-1
	// blast-radius case, forced rather than waited for.
	seedActive(t, cl, dbObj("pg", w.ID))
	seedTerminating(t, cl, dbObj("pg-gone", w.ID))
	seedActive(t, cl, kvObj("kv", w.ID))
	seedTerminating(t, cl, kvObj("kv-gone", w.ID))

	limits, err := svc.ResourceLimits(ctxAs("user-a"), w.ID)
	if err != nil {
		t.Fatalf("ResourceLimits: %v", err)
	}

	// services: web, api, paused, gone present (erased finalized away) → Used 4,
	// Terminating 1 (gone), listed 3 (web, api, paused).
	assertReconciles(t, "services", limits.Services, 4, 1, 3)
	assertReconciles(t, "postgres", limits.Postgres, 2, 1, 1)
	assertReconciles(t, "keyValues", limits.KeyValues, 2, 1, 1)

	// The paid ceiling is unchanged by any of this.
	if limits.Services.Limit != 100 || limits.Postgres.Limit != 25 || limits.KeyValues.Limit != 25 {
		t.Errorf("limits drifted: %+v", limits)
	}
}

// TestResourceLimits_HobbyDatastoreFullyConsumedByOneTerminating pins the
// highest-stakes case from the milestone: on Hobby the Postgres/Key Value cap is
// 1, so a single lingering datastore CR consumes the entire quota — and the
// tenant must be able to see that it is the terminating one holding it.
func TestResourceLimits_HobbyDatastoreFullyConsumedByOneTerminating(t *testing.T) {
	st := newFakeStore()
	svc, cl := limitsSvc(t, st)
	w, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	seedTerminating(t, cl, dbObj("pg-gone", w.ID))
	seedTerminating(t, cl, kvObj("kv-gone", w.ID))

	limits, err := svc.ResourceLimits(ctxAs("user-a"), w.ID)
	if err != nil {
		t.Fatalf("ResourceLimits: %v", err)
	}

	// 1 of 1 used, that 1 is terminating, 0 visible in the list — the empty
	// database list beside a refusal-to-create the milestone calls out.
	assertReconciles(t, "postgres", limits.Postgres, 1, 1, 0)
	assertReconciles(t, "keyValues", limits.KeyValues, 1, 1, 0)
	if limits.Postgres.Limit != 1 || limits.KeyValues.Limit != 1 {
		t.Errorf("hobby datastore cap = %d/%d, want 1/1", limits.Postgres.Limit, limits.KeyValues.Limit)
	}
}
