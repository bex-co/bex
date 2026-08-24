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
	// user-a IS a member of tea-2 (so the request isn't refused on membership
	// alone) but OpenFGA denies can_view there.
	svc.Workspace = fakeWorkspace{"user-a": "tea-2"}
	svc.Authz = &fakeChecker{deny: core.WorkspaceObject("tea-2")}

	if _, err := svc.ListPostgres(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for an inaccessible ownerId, got %v", err)
	}
}

// TestListPostgres_OwnerIDFilterForbiddenWhenCallerIsNotAMember is w6/m17/t003's
// own regression: before folding ListPostgres(ownerID) onto core.WithWorkspace,
// the ownerID gate was OpenFGA-only (AuthorizeOn) with NO IsMember check, so an
// allow-all authorizer would let a caller list a workspace it does not even
// belong to. Here OpenFGA allows everything; only membership can refuse.
func TestListPostgres_OwnerIDFilterForbiddenWhenCallerIsNotAMember(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	// user-a's only membership is tea-1 — not tea-2, the one they're asking for.
	svc.Workspace = fakeWorkspace{"user-a": "tea-1"}
	svc.Authz = &fakeChecker{allow: true}

	if _, err := svc.ListPostgres(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for a workspace the caller does not belong to (even with OpenFGA wide open), got %v", err)
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
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: view.ID}, &d); err != nil {
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

func TestListPostgres_OmittedOwnerUsesDefaultWorkspace(t *testing.T) {
	a := ownedDatabase("db-a", "tea-a")
	b := ownedDatabase("db-b", "tea-b")
	svc, _ := newService(a, b)
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Authz = &fakeChecker{allow: true}

	list, err := svc.ListPostgres(ctxAs("user-a"), "")
	if err != nil {
		t.Fatalf("ListPostgres(default): %v", err)
	}
	if len(list) != 1 || list[0].ID != "db-a" {
		t.Fatalf("ListPostgres(default) = %+v, want only db-a", list)
	}
}

// --- w6/m14: the fetch-by-name workspace gate ---

// twoWorkspaces is a multi-workspace caller: subject -> workspaces, oldest first.
type twoWorkspaces map[string][]string

func (w twoWorkspaces) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws := w[id.Subject]
	if len(ws) == 0 {
		return "", false
	}
	return ws[0], true
}

func (w twoWorkspaces) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, t := range w[id.Subject] {
		if t == tenantID {
			return true, nil
		}
	}
	return false, nil
}

func ownedDatabase(name, tenantID string) *appv1alpha1.Database {
	d := sampleDatabase(name)
	d.Labels = map[string]string{core.LabelTenant: tenantID}
	return d
}

// TestGetPostgres_CrossTenantByNameIsForbidden closes a real hole (found by the
// w6/m14 review): fetchDatabase was a bare client.Get, so ANY authenticated
// caller who knew a Database's name could read it — and PostgresConnectionInfo
// rides the same fetch, so that included another workspace's credentials. Lists
// were scoped; get-by-name was not.
func TestGetPostgres_CrossTenantByNameIsForbidden(t *testing.T) {
	svc, _ := newService(ownedDatabase("acme-db", "tea-acme"))
	svc.Workspace = fakeWorkspace{"mallory": "tea-evil"}
	svc.Authz = &fakeChecker{allow: true} // a normal member of her OWN workspace

	if _, err := svc.GetPostgres(ctxAs("mallory"), "acme-db"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("GetPostgres on another workspace's database: got %v, want ErrForbidden", err)
	}
	if _, err := svc.PostgresConnectionInfo(ctxAs("mallory"), "acme-db"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("PostgresConnectionInfo on another workspace's database: got %v, want ErrForbidden — this leaks credentials", err)
	}
	if err := svc.DeletePostgres(ctxAs("mallory"), "acme-db"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("DeletePostgres on another workspace's database: got %v, want ErrForbidden", err)
	}
	newName := "stolen-name"
	if _, err := svc.UpdatePostgres(ctxAs("mallory"), "acme-db", PostgresPatch{Name: &newName}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("UpdatePostgres rename on another workspace's database: got %v, want ErrForbidden", err)
	}
}

func TestCreatePostgresAllowsSameDisplayNameInDifferentWorkspaces(t *testing.T) {
	workspace := fakeWorkspace{"alice": "tea-a", "bob": "tea-b"}
	svc, _ := newTenantService(workspace)
	a, err := svc.CreatePostgres(ctxAs("alice"), CreatePostgresRequest{Name: "shared-name", Plan: "free"})
	if err != nil {
		t.Fatalf("create in tea-a: %v", err)
	}
	b, err := svc.CreatePostgres(ctxAs("bob"), CreatePostgresRequest{Name: "shared-name", Plan: "free"})
	if err != nil {
		t.Fatalf("same display name in tea-b: %v", err)
	}
	if a.ID == b.ID || a.Name != b.Name || a.OwnerID == b.OwnerID {
		t.Fatalf("workspace-scoped names = a:%+v b:%+v", a, b)
	}
	_, err = svc.CreatePostgres(ctxAs("alice"), CreatePostgresRequest{Name: "shared-name", Plan: "free"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate display name in tea-a: got %v, want ErrConflict", err)
	}
	// w6/m49: a stable code, not just message text, so a dashboard hook can
	// detect the conflict without matching backend copy.
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "CONFLICT" {
		t.Fatalf("duplicate display name in tea-a: got %v, want *core.CodedError{Code: CONFLICT}", err)
	}
}

// The owner still reaches their own database — including one in their SECOND
// workspace, which implicit resolution does not pick (the m11 shape, for CRs
// other than Apps).
func TestGetPostgres_OwnersOtherWorkspaceIsReachable(t *testing.T) {
	svc, _ := newService(ownedDatabase("db-b", "tea-2"))
	svc.Workspace = twoWorkspaces{"dana": {"tea-1", "tea-2"}} // default = tea-1
	svc.Authz = &fakeChecker{allow: true}

	v, err := svc.GetPostgres(ctxAs("dana"), "db-b")
	if err != nil {
		t.Fatalf("GetPostgres on her own database in her second workspace: %v", err)
	}
	if v.OwnerID != "tea-2" {
		t.Errorf("ownerId = %q, want tea-2", v.OwnerID)
	}
}

// ...but a role does not leak across workspaces: a caller who lacks can_create
// in the owning workspace cannot delete its database, however privileged they
// are in their own.
func TestDeletePostgres_RoleDoesNotLeakAcrossWorkspaces(t *testing.T) {
	svc, _ := newService(ownedDatabase("db-b", "tea-2"))
	svc.Workspace = twoWorkspaces{"dana": {"tea-1", "tea-2"}}
	svc.Authz = &fakeChecker{deny: core.WorkspaceObject("tea-2")} // not allowed to act in tea-2

	if err := svc.DeletePostgres(ctxAs("dana"), "db-b"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("DeletePostgres in a workspace where she lacks the relation: got %v, want ErrForbidden", err)
	}
}
