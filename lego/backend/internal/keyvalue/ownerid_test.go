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

package keyvalue

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ownerid_test.go covers the ownerId contract w6/m4/t002 brought to managed
// KeyValue, mirroring postgres/ownerid_test.go: CreateKeyValue stamps both
// workspace labels with the caller's real tenant id, and the ownerId
// list-filter authorizes the requested workspace and isolates cleanly between
// tenants.

type fakeChecker struct {
	allow bool
	deny  string
}

func (c *fakeChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	if c.deny != "" {
		return object != c.deny, nil
	}
	return c.allow, nil
}

// fakeWorkspace is a map-backed core.WorkspaceResolver, mirroring apps'/
// postgres' test helper: identities not in the map resolve ok=false.
type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

// IsMember: a map-backed caller belongs to exactly the one workspace it
// resolves to — the single-membership case every pre-w6/m14 test is written
// against. Multi-membership callers use a richer fake (see the m14 tests).
func (f fakeWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	tid, ok := f[id.Subject]
	return ok && tid == tenantID, nil
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func sampleKeyValue(name string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func TestListKeyValues_UnlabeledStoreReadsAsUnowned(t *testing.T) {
	svc, _ := newService(sampleKeyValue("kv1"))
	list, err := svc.ListKeyValues(ctxAs("user-a"), "")
	if err != nil || len(list) != 1 || list[0].OwnerID != "" {
		t.Fatalf("ListKeyValues = %+v, err=%v; want one unowned instance", list, err)
	}
}

func TestListKeyValues_OwnerIDFilterYieldsEmptyNotUnscoped(t *testing.T) {
	svc, _ := newService(sampleKeyValue("kv1"))
	svc.Authz = &fakeChecker{allow: true}

	list, err := svc.ListKeyValues(ctxAs("user-a"), "tea-1")
	if err != nil || len(list) != 0 {
		t.Fatalf("ListKeyValues(tea-1) = %+v, err=%v; want empty", list, err)
	}
}

func TestListKeyValues_OwnerIDFilterForbiddenWhenCallerCantAccess(t *testing.T) {
	svc, _ := newService(sampleKeyValue("kv1"))
	svc.Authz = &fakeChecker{deny: core.WorkspaceObject("tea-2")}

	if _, err := svc.ListKeyValues(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for an inaccessible ownerId, got %v", err)
	}
}

// TestCreateKeyValue_StampsBothLabels is w6/m4/t002's regression test.
func TestCreateKeyValue_StampsBothLabels(t *testing.T) {
	svc, cl := newService()
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}

	view, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "kv1"})
	if err != nil {
		t.Fatalf("CreateKeyValue: %v", err)
	}
	if view.OwnerID != "tea-a" {
		t.Fatalf("created view OwnerID = %q, want tea-a", view.OwnerID)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "kv1"}, &kv); err != nil {
		t.Fatalf("get KeyValue: %v", err)
	}
	if kv.Labels[core.LabelTenant] != "tea-a" || kv.Labels[core.LabelWorkspace] != "tea-a" {
		t.Fatalf("KeyValue labels = %+v, want both LabelTenant and LabelWorkspace = tea-a", kv.Labels)
	}
}

// TestListKeyValues_OwnerIDFilterIsolatesTenants proves the filter actually
// isolates between two labeled tenants, not just "returns something".
func TestListKeyValues_OwnerIDFilterIsolatesTenants(t *testing.T) {
	a := sampleKeyValue("kv-a")
	a.Labels = map[string]string{core.LabelTenant: "tea-a"}
	b := sampleKeyValue("kv-b")
	b.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, _ := newService(a, b)
	svc.Authz = &fakeChecker{allow: true}

	list, err := svc.ListKeyValues(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatalf("ListKeyValues(tea-a): %v", err)
	}
	if len(list) != 1 || list[0].ID != "kv-a" {
		t.Fatalf("ListKeyValues(tea-a) = %+v, want exactly kv-a", list)
	}
}

// --- w6/m14: the fetch-by-name workspace gate (same hole as postgres') ---

func ownedKeyValue(name, tenantID string) *appv1alpha1.KeyValue {
	kv := sampleKeyValue(name)
	kv.Labels = map[string]string{core.LabelTenant: tenantID}
	return kv
}

// fetchKeyValue was a bare client.Get: any authenticated caller who knew the
// name read the store — KeyValueConnectionInfo rides the same fetch, so that
// leaked another workspace's Valkey credentials.
func TestGetKeyValue_CrossTenantByNameIsForbidden(t *testing.T) {
	svc, _ := newService(ownedKeyValue("acme-kv", "tea-acme"))
	svc.Workspace = fakeWorkspace{"mallory": "tea-evil"}
	svc.Authz = &fakeChecker{allow: true}

	if _, err := svc.GetKeyValue(ctxAs("mallory"), "acme-kv"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("GetKeyValue on another workspace's store: got %v, want ErrForbidden", err)
	}
	if _, err := svc.KeyValueConnectionInfo(ctxAs("mallory"), "acme-kv"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("KeyValueConnectionInfo on another workspace's store: got %v, want ErrForbidden — this leaks credentials", err)
	}
	if err := svc.DeleteKeyValue(ctxAs("mallory"), "acme-kv"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("DeleteKeyValue on another workspace's store: got %v, want ErrForbidden", err)
	}
}
