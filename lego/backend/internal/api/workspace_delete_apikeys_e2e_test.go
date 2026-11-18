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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// workspace_delete_apikeys_e2e_test.go is w9/120's proof against REAL Postgres
// and REAL OpenFGA: deleting a workspace revokes every API key bound there,
// whoever minted it. Before, the delete revoked the tuples of the people
// (ListTenantMembers reads kind='user' only) and the cascade then dropped the
// keys' bindings, so each key kept its tuple and its Hydra client and no list
// could show or revoke it again.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestDeletingAWorkspaceRevokesItsAPIKeysE2E -v
//
// Additive like its siblings: every subject is unique to the run.

// fgaWorkspaceTuples reads every workspace tuple that names user from the real
// OpenFGA, as "relation object" strings.
func fgaWorkspaceTuples(t *testing.T, fgaURL, token, user string) []string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"tuple_key": map[string]string{"user": user, "object": "workspace:"},
		"page_size": 100,
	})
	code, out := fgaStoreCall(t, fgaURL, token, http.MethodPost, "/read", body)
	if code != http.StatusOK {
		t.Fatalf("openfga read: %d %s", code, out)
	}
	var read struct {
		Tuples []struct {
			Key struct{ Relation, Object string } `json:"key"`
		} `json:"tuples"`
	}
	if err := json.Unmarshal(out, &read); err != nil {
		t.Fatalf("read tuples: %v", err)
	}
	tuples := []string{}
	for _, tu := range read.Tuples {
		tuples = append(tuples, tu.Key.Relation+" "+tu.Key.Object)
	}
	return tuples
}

// unreachableHydra is the fake key registry with the Hydra delete of one key
// failing, as when Hydra's admin API drops a request.
type unreachableHydra struct {
	*fakeKeyStore
	failing string
}

func (h *unreachableHydra) Delete(ctx context.Context, id string) error {
	if id == h.failing {
		return errors.New("hydra admin: connection reset")
	}
	return h.fakeKeyStore.Delete(ctx, id)
}

func TestDeletingAWorkspaceRevokesItsAPIKeysE2E(t *testing.T) {
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
	admin, dev := "admin-"+run, "dev-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, fgaToken)
	roles := checker.(store.MembershipGranter)

	doomed, err := st.CreateWorkspace(ctx, "doomed-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := st.CreateWorkspace(ctx, "kept-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, dev, doomed.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct{ workspace, subject, role string }{
		{doomed.ID, admin, "admin"}, {kept.ID, admin, "admin"}, {doomed.ID, dev, "developer"},
	} {
		if err := roles.GrantWorkspaceRole(ctx, g.workspace, "user:"+g.subject, g.role); err != nil {
			t.Fatalf("grant %s %s: %v", g.subject, g.role, err)
		}
	}

	tenantSvc := tenancy.NewTenantService(st, roles)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: tenantSvc}
	hydra := &unreachableHydra{fakeKeyStore: newFakeKeyStore()}
	// The revocation marker is permanent per client id: start where no other
	// run has been.
	hydra.n = int(time.Now().UnixMicro())
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		WorkspaceGranter: checker.(workspaces.WorkspaceGranter),
		WorkspaceRevoker: checker.(workspaces.WorkspaceRevoker),
		APIKeys:          hydra, KeyBinder: tenantSvc,
	})
	mux := srv.restHandler()
	mint := func(subject, workspace string) string {
		t.Helper()
		resp := e2eCall(mux, ctx, subject, "POST", "/v1/api-keys", `{"name":"k","ownerId":"`+workspace+`"}`).Result()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s mints in %s: %d", subject, workspace, resp.StatusCode)
		}
		return decodeJSON[apikeys.APIKey](t, resp).ID
	}
	adminKey, devKey, keptKey := mint(admin, doomed.ID), mint(dev, doomed.ID), mint(admin, kept.ID)
	tuples := func(key string) []string { return fgaWorkspaceTuples(t, fgaURL, fgaToken, "user:"+key) }
	for key, want := range map[string]string{adminKey: "admin workspace:" + doomed.ID, devKey: "developer workspace:" + doomed.ID} {
		if got := tuples(key); len(got) != 1 || got[0] != want {
			t.Fatalf("precondition: %s holds %v, want [%s]", key, got, want)
		}
	}
	hydraKnows := func(key string) bool {
		hydra.mu.Lock()
		defer hydra.mu.Unlock()
		_, ok := hydra.keys[key]
		return ok
	}
	refused := func(key string) bool {
		t.Helper()
		_, revoked, err := st.OAuthRevokedAt(ctx, key, key)
		if err != nil {
			t.Fatal(err)
		}
		return revoked
	}

	// The admin deletes the workspace while Hydra drops the dev key's delete.
	// The delete fails and the workspace stays, but both keys have already lost
	// their tuples and are refused on every replica, and the dev key is still
	// bound, so the retry finds it.
	hydra.failing = devKey
	confirm := workspaces.DeleteConfirmation(doomed.Label())
	actor := core.WithIdentity(ctx, core.Identity{Subject: admin, Method: "session", Human: true})
	if err := srv.Workspaces.Delete(actor, doomed.ID, confirm); err == nil {
		t.Fatal("the delete reported success while Hydra still holds the dev key")
	}
	if _, err := st.GetTenant(ctx, doomed.ID); err != nil {
		t.Fatalf("the failed delete dropped the workspace: %v", err)
	}
	for _, key := range []string{adminKey, devKey} {
		if got := tuples(key); len(got) != 0 {
			t.Errorf("%s still holds %v after the failed delete", key, got)
		}
		if !refused(key) {
			t.Errorf("%s is not refused after the failed delete", key)
		}
	}
	if !hydraKnows(devKey) {
		t.Fatal("precondition: the dev key's Hydra delete was meant to fail")
	}
	if owner, ok := tenantSvc.TenantForKey(ctx, devKey); !ok || owner != doomed.ID {
		t.Fatalf("the dev key is no longer bound to the workspace (%q, %v), so no retry finds it", owner, ok)
	}

	// The retry (the account worker's path, which re-runs the same teardown)
	// converges: the workspace is gone, Hydra knows neither key, no tuple names
	// either, and no binding row is left.
	hydra.failing = ""
	if err := (workspaces.AccountTeardown{Service: srv.Workspaces}).Delete(ctx, doomed.ID); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := st.GetTenant(ctx, doomed.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the workspace survived the retry: %v", err)
	}
	for _, key := range []string{adminKey, devKey} {
		if hydraKnows(key) {
			t.Errorf("Hydra still knows %s", key)
		}
		if got := tuples(key); len(got) != 0 {
			t.Errorf("%s still holds %v", key, got)
		}
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE subject = $1`, key).Scan(&rows); err != nil || rows != 0 {
			t.Errorf("%s keeps %d binding rows (%v)", key, rows, err)
		}
	}

	// The admin's key in the other workspace is not the deleted one's business.
	if !hydraKnows(keptKey) || refused(keptKey) {
		t.Error("the key bound to the other workspace was revoked")
	}
	if got := tuples(keptKey); len(got) != 1 || got[0] != "admin workspace:"+kept.ID {
		t.Errorf("the key bound to the other workspace holds %v", got)
	}
}
