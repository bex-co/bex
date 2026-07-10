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

package postgres

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ownerid_test.go covers the ownerId contract for managed Postgres: a
// hand-applied (unlabeled) Database CR reads as unowned (w6/m2/t004's original
// behavior, still correct for that case), CreatePostgres stamps both workspace
// labels with the caller's real tenant id (w6/m4/t001's fix), and the ownerId
// list-filter authorizes the requested workspace and isolates cleanly between
// tenants — it never fabricates scoped results out of unlabeled data, and it
// never leaks another tenant's rows into a scoped request.

// fakeChecker mirrors apps' test helper: allow every object except one denied.
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

// fakeWorkspace is a map-backed core.WorkspaceResolver, mirroring apps' test
// helper: identities not in the map resolve ok=false.
type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func sampleDatabase(name string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func TestListPostgres_UnlabeledDatabaseReadsAsUnowned(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	list, err := svc.ListPostgres(ctxAs("user-a"), "")
	if err != nil || len(list) != 1 || list[0].OwnerID != "" {
		t.Fatalf("ListPostgres = %+v, err=%v; want one unowned instance", list, err)
	}
}

func TestListPostgres_OwnerIDFilterYieldsEmptyNotUnscoped(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	svc.Authz = &fakeChecker{allow: true}

	// Authorized ownerId, but db1 carries no label at all: empty, not db1
	// (never silently return unscoped data for a scoped request).
	list, err := svc.ListPostgres(ctxAs("user-a"), "tea-1")
	if err != nil || len(list) != 0 {
		t.Fatalf("ListPostgres(tea-1) = %+v, err=%v; want empty", list, err)
	}
}

func TestListPostgres_OwnerIDFilterForbiddenWhenCallerCantAccess(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	svc.Authz = &fakeChecker{deny: core.WorkspaceObject("tea-2")}

	if _, err := svc.ListPostgres(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for an inaccessible ownerId, got %v", err)
	}
}

// TestCreatePostgres_StampsBothLabels is w6/m4/t001's regression test: before
// the fix, CreatePostgres stamped only LabelWorkspace, leaving OwnerID
// (which reads LabelTenant) permanently empty for every workspace-created
// Database.
func TestCreatePostgres_StampsBothLabels(t *testing.T) {
	svc, cl := newService()
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}

	view, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "db1"})
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	if view.OwnerID != "tea-a" {
		t.Fatalf("created view OwnerID = %q, want tea-a", view.OwnerID)
	}
	var d appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "db1"}, &d); err != nil {
		t.Fatalf("get Database: %v", err)
	}
	if d.Labels[core.LabelTenant] != "tea-a" || d.Labels[core.LabelWorkspace] != "tea-a" {
		t.Fatalf("Database labels = %+v, want both LabelTenant and LabelWorkspace = tea-a", d.Labels)
	}
}

// TestListPostgres_OwnerIDFilterIsolatesTenants proves the filter actually
// isolates — not just "returns something" — between two labeled tenants.
func TestListPostgres_OwnerIDFilterIsolatesTenants(t *testing.T) {
	a := sampleDatabase("db-a")
	a.Labels = map[string]string{core.LabelTenant: "tea-a"}
	b := sampleDatabase("db-b")
	b.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, _ := newService(a, b)
	svc.Authz = &fakeChecker{allow: true}

	list, err := svc.ListPostgres(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatalf("ListPostgres(tea-a): %v", err)
	}
	if len(list) != 1 || list[0].ID != "db-a" {
		t.Fatalf("ListPostgres(tea-a) = %+v, want exactly db-a", list)
	}
}
