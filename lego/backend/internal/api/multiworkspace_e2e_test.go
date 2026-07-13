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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// multiworkspace_e2e_test.go is w6/m14's end-to-end proof (t007), against REAL
// infrastructure — a live Postgres (the membership source of truth) and a live
// OpenFGA (enforced, per-workspace roles) — driving the REAL REST surface
// through the REAL store-backed workspace resolver (api.tenantService, not a
// fake): the whole chain the field bug ran through.
//
// It is the same harness TestWorkspaceLifecycleE2E uses, and skipped the same
// way unless both point at throwaway infra:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run TestMultiWorkspaceTargetingE2E -v
//
// The OpenFGA at that URL must have a store named "bex" carrying
// deploy/gitops/authz/model.json (scripts/authz-model.sh).
func TestMultiWorkspaceTargetingE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		t.Skip("BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	// This test is ADDITIVE — it never truncates. `go test ./...` runs packages
	// concurrently, so a TRUNCATE here would race internal/store's tests against
	// the same throwaway database and fail them from the outside. Instead every
	// workspace name and every subject is unique to this run, so the assertions
	// (notably "dana's oldest membership") hold no matter what else is in the DB.
	// A short, unique-per-run suffix. The TRAILING chars of an xid are its
	// counter/pid; the leading ones are a timestamp, so two runs in the same
	// second would collide on a prefix. Short because an App name is a DNS label
	// (30 chars max) and these names are embedded in one.
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	dana, carl, erin := "dana-"+run, "carl-"+run, "erin-"+run
	ws := func(kind string) string { return kind + "-" + run }

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	// The full role-granting seam (store.MembershipGranter) — needed to make carl a
	// VIEWER of bravo, a role WorkspaceGranter's admin-only grant cannot express.
	roles := checker.(store.MembershipGranter)

	// dana's two workspaces, in the order she got them: A first (her default), B
	// second. Real tenants + tenant_members rows; real FGA admin tuples.
	wsA, err := st.CreateWorkspace(ctx, ws("alpha"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	wsB, err := st.CreateWorkspace(ctx, ws("bravo"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create bravo: %v", err)
	}
	// erin's workspace — one dana has no membership in at all.
	wsE, err := st.CreateWorkspace(ctx, ws("echo"), store.PlanHobby, erin)
	if err != nil {
		t.Fatalf("create echo: %v", err)
	}
	for _, w := range []store.Tenant{wsA, wsB} {
		if err := granter.GrantWorkspaceAdmin(ctx, w.ID, "user:"+dana); err != nil {
			t.Fatalf("grant admin on %s: %v", w.Name, err)
		}
	}
	if err := granter.GrantWorkspaceAdmin(ctx, wsE.ID, "user:"+erin); err != nil {
		t.Fatalf("grant admin on echo: %v", err)
	}
	// carl is a VIEWER of bravo and an ADMIN of his own workspace — the caller
	// who makes membership-alone-is-enough a privilege escalation.
	wsC, err := st.CreateWorkspace(ctx, ws("charlie"), store.PlanHobby, carl)
	if err != nil {
		t.Fatalf("create charlie: %v", err)
	}
	if err := granter.GrantWorkspaceAdmin(ctx, wsC.ID, "user:"+carl); err != nil {
		t.Fatalf("grant admin on charlie: %v", err)
	}
	if err := st.AddMember(ctx, carl, wsB.ID, "viewer"); err != nil {
		t.Fatalf("add carl to bravo: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, wsB.ID, "user:"+carl, "viewer"); err != nil {
		t.Fatalf("grant carl viewer on bravo: %v", err)
	}

	// The REAL resolver: store-backed TenantForIdentity (the ORDER BY) + IsMember.
	cl := fakeClient()
	base := &core.Base{
		Client: cl, Namespace: "default",
		Authz:     checker,
		Workspace: NewTenantService(st, roles),
	}
	srv := NewServer(base, Deps{Store: st, WorkspaceStore: st, WorkspaceGranter: granter})
	mux := srv.restHandler()

	// call drives the real REST router as `subject` (the auth middleware's job —
	// attaching the Identity — is done here, as the other e2e tests do).
	call := func(subject, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("content-type", "application/json")
		rec := httptest.NewRecorder()
		ictx := core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "session"})
		mux.ServeHTTP(rec, req.WithContext(ictx))
		return rec
	}
	appTenantOf := func(t *testing.T, name string) string {
		t.Helper()
		var a appv1alpha1.App
		if err := cl.Get(ctx, k8sKey(name), &a); err != nil {
			t.Fatalf("App %s: %v", name, err)
		}
		return a.Labels[core.LabelTenant]
	}

	bravoWeb, stolen := "bravo-web-"+run, "stolen-"+run

	// (0) dana's DEFAULT workspace is alpha — her OLDEST membership, not a coin
	// flip. This is the t001 contract, read through the production resolver.
	if got, err := st.TenantForIdentity(ctx, dana); err != nil || got.ID != wsA.ID {
		t.Fatalf("default workspace = %+v (err %v), want alpha %s", got, err, wsA.ID)
	}

	// (1) A create NAMING bravo lands in bravo — not in alpha, which is what the
	// implicit resolution would have picked.
	rec := call(dana, "POST", "/v1/services", `{"name":"`+bravoWeb+`","image":{"imagePath":"nginx"},"ownerId":"`+wsB.ID+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services ownerId=bravo: %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ OwnerID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.OwnerID != wsB.ID {
		t.Errorf("response ownerId = %q, want bravo %q", created.OwnerID, wsB.ID)
	}
	if got := appTenantOf(t, bravoWeb); got != wsB.ID {
		t.Errorf("App landed in %q, want bravo %q", got, wsB.ID)
	}

	// (2) THE w6/m11 REGRESSION: dana reads her bravo service with NO ownerId, so
	// the implicit resolution picks alpha. It must serve her own service, not 403.
	if rec := call(dana, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusOK {
		t.Errorf("GET her own bravo service (implicit resolution = alpha): %d %s — want 200; an owner must never be 403'd by which of her workspaces resolved",
			rec.Code, rec.Body.String())
	}
	// ...and she can operate on it, too (the write path through the same gate).
	if rec := call(dana, "POST", "/v1/services/"+bravoWeb+"/restart", ""); rec.Code != http.StatusOK {
		t.Errorf("restart her own bravo service: %d %s — want 200", rec.Code, rec.Body.String())
	}

	// (3) A create naming a workspace dana does NOT belong to is refused — and
	// crucially, refused rather than silently redirected into one of her own.
	rec = call(dana, "POST", "/v1/services", `{"name":"`+stolen+`","image":{"imagePath":"nginx"},"ownerId":"`+wsE.ID+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST ownerId=echo (not dana's): %d %s, want 403", rec.Code, rec.Body.String())
	}
	var stray appv1alpha1.App
	if err := cl.Get(ctx, k8sKey(stolen), &stray); err == nil {
		t.Errorf("the refused create still wrote an App, labeled %q — it must write nothing", stray.Labels[core.LabelTenant])
	}

	// (4) NO CROSS-WORKSPACE PRIVILEGE ESCALATION. carl is an ADMIN of charlie and
	// only a VIEWER of bravo. He may READ bravo's service (viewer ⇒ can_view)...
	if rec := call(carl, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusOK {
		t.Errorf("carl (viewer of bravo) GET bravo-web: %d %s, want 200", rec.Code, rec.Body.String())
	}
	// ...but he must NOT be able to delete it: can_create does not hold for a
	// viewer in bravo, even though he is an admin in his own default workspace,
	// which is the workspace his implicit resolution picks.
	if rec := call(carl, "DELETE", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("carl (viewer of bravo, admin of charlie) DELETE bravo-web: %d %s, want 403 — a role must not leak across workspaces",
			rec.Code, rec.Body.String())
	}
	if err := cl.Get(ctx, k8sKey(bravoWeb), &stray); err != nil {
		t.Errorf("bravo-web was deleted by a viewer: %v", err)
	}
	// ...and he cannot read its env vars either (can_view_sensitive: developer+).
	if rec := call(carl, "GET", "/v1/services/"+bravoWeb+"/env-vars", ""); rec.Code == http.StatusOK {
		t.Errorf("carl (viewer of bravo) read bravo-web's env vars: %d — want a refusal", rec.Code)
	}

	// (5) erin, a member of NEITHER of dana's workspaces, cannot reach her
	// service at all — the isolation that predates m14 and must still hold.
	if rec := call(erin, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("erin GET dana's service: %d %s, want 403", rec.Code, rec.Body.String())
	}
}

// k8sKey is the fake apiserver's object key for an App in the test namespace.
func k8sKey(name string) client.ObjectKey {
	return client.ObjectKey{Namespace: "default", Name: name}
}
