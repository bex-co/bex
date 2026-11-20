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

package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// workspace_delete_retry_e2e_test.go is w9/121's proof against REAL Postgres
// and REAL OpenFGA. A workspace delete revokes every member's tuple before its
// teardown runs, so that nobody creates a resource or mints a key while it is
// in flight. When a purger then failed, the admin's identical retry answered
// 403: nobody held can_manage any more. Now the retry through
// workspaces.Service.Delete completes, nobody else can use it, and the
// workspace stays closed to everyone until it does, a personal one included.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestHalfDeletedWorkspaceE2E -v
//
// Additive like its siblings: every subject is unique to the run.

// teardownFault fails the pre-cascade teardown of each workspace it holds, as
// an OpenBao or Stripe outage does.
type teardownFault struct {
	mu      sync.Mutex
	failing map[string]bool
}

func (f *teardownFault) set(workspaceID string, failing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing[workspaceID] = failing
}

func (f *teardownFault) PurgeWorkspace(_ context.Context, workspaceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing[workspaceID] {
		return errors.New("openbao: connection refused")
	}
	return nil
}

func TestHalfDeletedWorkspaceE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	fgaToken := os.Getenv("BEX_TEST_OPENFGA_TOKEN")
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	admin, dev, former, outsider, owner := "admin-"+run, "dev-"+run, "former-"+run, "outsider-"+run, "owner-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, fgaToken)
	roles := checker.(store.MembershipGranter)
	revoker := checker.(workspaces.WorkspaceRevoker)

	doomed, err := st.CreateWorkspace(ctx, "doomed-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := st.CreateWorkspace(ctx, "elsewhere-"+run, store.PlanPro, outsider)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ subject, role string }{{dev, "developer"}, {former, "admin"}} {
		if err := st.AddMember(ctx, m.subject, doomed.ID, m.role); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []struct{ workspace, subject, role string }{
		{doomed.ID, admin, "admin"}, {doomed.ID, dev, "developer"}, {doomed.ID, former, "admin"}, {elsewhere.ID, outsider, "admin"},
	} {
		if err := roles.GrantWorkspaceRole(ctx, g.workspace, "user:"+g.subject, g.role); err != nil {
			t.Fatalf("grant %s %s: %v", g.subject, g.role, err)
		}
	}
	// former was an admin here and was removed before anyone deleted it.
	if err := st.RemoveMember(ctx, doomed.ID, former); err != nil {
		t.Fatal(err)
	}
	if err := revoker.RevokeWorkspaceMember(ctx, doomed.ID, "user:"+former, "admin"); err != nil {
		t.Fatal(err)
	}

	tenantSvc := tenancy.NewTenantService(st, roles)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: tenantSvc}
	keys := newFakeKeyStore()
	keys.n = int(time.Now().UnixMicro()) // the revocation marker is permanent per client id
	fault := &teardownFault{failing: map[string]bool{}}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		WorkspaceGranter: checker.(workspaces.WorkspaceGranter), WorkspaceRevoker: revoker,
		APIKeys: keys, KeyBinder: tenantSvc,
		Secrets: newMemSecretStore(), EnvGroupsStore: st,
		WorkspacePreCascadePurgers: []workspaces.WorkspacePurger{fault},
	})
	mux := srv.restHandler()
	as := func(subject string) context.Context {
		return core.WithIdentity(ctx, core.Identity{Subject: subject, Method: core.MethodSession, Human: true})
	}
	n := 0
	createGroup := func(subject, workspace string) int {
		n++
		return e2eCall(mux, ctx, subject, "POST", "/v1/env-groups",
			`{"name":"g-`+run+`-`+strconv.Itoa(n)+`","ownerId":"`+workspace+`"}`).Code
	}
	mintKey := func(subject, workspace string) int {
		return e2eCall(mux, ctx, subject, "POST", "/v1/api-keys", `{"name":"k","ownerId":"`+workspace+`"}`).Code
	}
	tuplesOn := func(subject, workspace string) []string {
		var on []string
		for _, tu := range fgaWorkspaceTuples(t, fgaURL, fgaToken, "user:"+subject) {
			if strings.HasSuffix(tu, " "+core.WorkspaceObject(workspace)) {
				on = append(on, tu)
			}
		}
		return on
	}
	exists := func(workspace string) bool {
		t.Helper()
		_, err := st.GetTenant(ctx, workspace)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			t.Fatal(err)
		}
		return err == nil
	}

	// Before the delete, the admin and the developer create and mint there.
	for _, subject := range []string{admin, dev} {
		if code := createGroup(subject, doomed.ID); code != http.StatusCreated {
			t.Fatalf("precondition: %s creates an env group: %d", subject, code)
		}
		if code := mintKey(subject, doomed.ID); code != http.StatusCreated {
			t.Fatalf("precondition: %s mints a key: %d", subject, code)
		}
	}

	// The admin's delete fails in its teardown, after the revoke.
	fault.set(doomed.ID, true)
	confirm := workspaces.DeleteConfirmation(doomed.Label())
	if err := srv.Workspaces.Delete(as(admin), doomed.ID, confirm); err == nil {
		t.Fatal("the delete reported success while its teardown failed")
	}
	if !exists(doomed.ID) {
		t.Fatal("the failed delete dropped the workspace, so nothing is left to retry")
	}

	// Half deleted, it is closed to every member: no tuple, no resource, no key.
	for _, subject := range []string{admin, dev} {
		if on := tuplesOn(subject, doomed.ID); len(on) != 0 {
			t.Errorf("%s still holds %v after the failed delete", subject, on)
		}
		if code := createGroup(subject, doomed.ID); code != http.StatusForbidden {
			t.Errorf("%s creates an env group in the half-deleted workspace: %d, want 403", subject, code)
		}
		if code := mintKey(subject, doomed.ID); code != http.StatusForbidden {
			t.Errorf("%s mints a key in the half-deleted workspace: %d, want 403", subject, code)
		}
	}

	// Nobody but its recorded admin can retry it.
	for _, who := range []struct{ name, subject string }{
		{"its developer", dev},
		{"an admin removed before the delete", former},
		{"an admin of another workspace", outsider},
		{"a stranger", "stranger-" + run},
	} {
		if err := srv.Workspaces.Delete(as(who.subject), doomed.ID, confirm); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s retries the delete: %v, want forbidden", who.name, err)
		}
	}
	if err := srv.Workspaces.Delete(as(admin), doomed.ID, workspaces.DeleteConfirmation(elsewhere.Label())); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("the admin's retry with another workspace's phrase: %v, want bad request", err)
	}
	if !exists(doomed.ID) {
		t.Fatal("a refused retry deleted the workspace")
	}

	// A personal workspace too: its owner's next request no longer hands them
	// back the admin tuple the delete revoked.
	personal, err := tenantSvc.EnsureTenant(ctx, owner, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if code := createGroup(owner, personal); code != http.StatusCreated {
		t.Fatalf("precondition: the owner creates in their personal workspace: %d", code)
	}
	mine, err := st.GetTenant(ctx, personal)
	if err != nil {
		t.Fatal(err)
	}
	fault.set(personal, true)
	if err := srv.Workspaces.Delete(as(owner), personal, workspaces.DeleteConfirmation(mine.Label())); err == nil {
		t.Fatal("the personal delete reported success while its teardown failed")
	}
	if got, err := tenantSvc.EnsureTenant(ctx, owner, "", false); err != nil || got != personal {
		t.Fatalf("the owner's next request resolves to (%q, %v), want the half-deleted %q", got, err, personal)
	}
	if on := tuplesOn(owner, personal); len(on) != 0 {
		t.Errorf("the owner's next request regranted %v on their half-deleted workspace", on)
	}
	if code := createGroup(owner, personal); code != http.StatusForbidden {
		t.Errorf("the owner creates in their half-deleted workspace: %d, want 403", code)
	}

	// The admins' identical retries complete through the public verb.
	fault.set(doomed.ID, false)
	fault.set(personal, false)
	if err := srv.Workspaces.Delete(as(admin), doomed.ID, confirm); err != nil {
		t.Fatalf("the admin's retry: %v", err)
	}
	if err := srv.Workspaces.Delete(as(owner), personal, workspaces.DeleteConfirmation(mine.Label())); err != nil {
		t.Fatalf("the owner's retry: %v", err)
	}
	for _, w := range []string{doomed.ID, personal} {
		if exists(w) {
			t.Errorf("%s survived its admin's retry", w)
		}
	}
	if on := tuplesOn(outsider, elsewhere.ID); !slices.Equal(on, []string{"admin " + core.WorkspaceObject(elsewhere.ID)}) {
		t.Errorf("the other workspace's admin holds %v", on)
	}
	// The owner starts over in a new personal workspace of their own.
	fresh, err := tenantSvc.EnsureTenant(ctx, owner, "", false)
	if err != nil || fresh == personal {
		t.Fatalf("after the delete the owner resolves to (%q, %v), want a new personal workspace", fresh, err)
	}
	if code := createGroup(owner, fresh); code != http.StatusCreated {
		t.Errorf("the owner creates in their new personal workspace: %d", code)
	}
}
