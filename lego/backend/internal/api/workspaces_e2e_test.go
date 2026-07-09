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
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestWorkspaceLifecycleE2E proves the w6/m1 definition of done end-to-end
// against REAL infrastructure — a live Postgres (the source of truth) and a
// live OpenFGA (enforced authorization) — driving the actual GraphQL surface
// through the real authz checker, exactly as the dashboard would. It stands in
// for a full cluster run: the only substitutions are a controller-runtime fake
// apiserver (so App-CR teardown is observable without a kube-apiserver) and the
// direct GraphQL schema call in place of the generic HTTP auth middleware
// (already covered by the auth tests).
//
// Hermetic-by-default: skipped unless BOTH point at throwaway infra:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:55432/bex?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58080 \
//	  go test ./internal/api/ -run TestWorkspaceLifecycleE2E -v
//
// The OpenFGA at that URL must have a store named "bex" carrying
// deploy/gitops/authz/model.json (scripts/authz-model.sh, or the curl in the
// w6/m1 runbook).
func TestWorkspaceLifecycleE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		t.Skip("BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()

	// --- real Postgres source of truth ---
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := store.NewPGStore(pool)

	// --- real OpenFGA enforced authorization ---
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	revoker := checker.(workspaces.WorkspaceRevoker)
	// The caller needs can_create on workspace:default to create workspaces (the
	// create verb authorizes there until w1/m9's per-caller scoping). admin
	// implies can_create. This is the m9-mint stand-in: it's how "alice" comes to
	// exist as an authorized identity. The grant is idempotent-tolerant (OpenFGA
	// 400s a duplicate tuple, which is fine across repeated runs) — the actual
	// precondition is asserted by the can_create check that follows.
	_ = granter.GrantWorkspaceAdmin(ctx, "default", "user:alice")
	if ok, err := checker.Check(ctx, "user:alice", "can_create", core.DefaultWorkspace); err != nil || !ok {
		t.Fatalf("precondition: alice must have can_create on workspace:default: ok=%v err=%v", ok, err)
	}

	// --- fake apiserver + reconciler so App-CR teardown is observable ---
	cl := fakeClient()
	rec := store.NewReconciler(cl, st, "default")
	base := &core.Base{Client: cl, Namespace: "default", Authz: checker}

	srv := NewServer(base, Deps{
		Store:            st,
		WorkspaceStore:   st,
		WorkspaceGranter: granter,
		WorkspaceRevoker: revoker,
		// Synchronous reconcile as the kick so the App-CR prune is deterministic
		// in-test (prod uses the async rec.Kick).
		WorkspaceKick: func() { _ = rec.ReconcileOnce(ctx) },
	})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	run := func(subject, query string, vars map[string]any) *graphql.Result {
		ictx := core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "session"})
		return graphql.Do(graphql.Params{Schema: schema, RequestString: query, VariableValues: vars, Context: ictx})
	}
	mustOK := func(t *testing.T, r *graphql.Result) {
		t.Helper()
		if len(r.Errors) != 0 {
			t.Fatalf("graphql errors: %v", r.Errors)
		}
	}

	const createMut = `mutation($n:String!,$p:String){ createWorkspace(name:$n, plan:$p){ id name plan role } }`

	// (1) alice's first workspace (the m9-minted stand-in) + a SECOND one.
	r := run("alice", createMut, map[string]any{"n": "first", "p": "hobby"})
	mustOK(t, r)
	r = run("alice", createMut, map[string]any{"n": "second", "p": "hobby"})
	mustOK(t, r)
	ws := r.Data.(map[string]any)["createWorkspace"].(map[string]any)
	secondID, _ := ws["id"].(string)
	if !strings.HasPrefix(secondID, "tea-") || ws["plan"] != "hobby" || ws["role"] != "admin" {
		t.Fatalf("second workspace = %+v", ws)
	}
	// Real tenants row + tenant_members row landed.
	if got, err := st.GetTenant(ctx, secondID); err != nil || got.Name != "second" {
		t.Fatalf("tenants row: %+v %v", got, err)
	}
	if members, _ := st.ListTenantMembers(ctx, secondID); len(members) != 1 || members[0].Subject != "alice" {
		t.Fatalf("tenant_members: %+v", members)
	}
	// Real FGA admin tuple: alice is admin on workspace:<secondID>, and admin
	// implies can_manage — enforced by the live OpenFGA, not a fake.
	if ok, err := checker.Check(ctx, "user:alice", "can_manage", core.WorkspaceObject(secondID)); err != nil || !ok {
		t.Fatalf("alice should manage her workspace: ok=%v err=%v", ok, err)
	}

	// (2) 6th Hobby workspace refused (alice already has first+second = 2 hobby).
	for _, n := range []string{"w3", "w4", "w5"} { // → 5 hobby
		mustOK(t, run("alice", createMut, map[string]any{"n": n, "p": "hobby"}))
	}
	r = run("alice", createMut, map[string]any{"n": "w6", "p": "hobby"})
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0].Message, "5 hobby") {
		t.Fatalf("6th hobby: want a limit error, got %v", r.Errors)
	}

	// (3) rename is visible in the workspaces list.
	mustOK(t, run("alice", `mutation($id:String!){ renameWorkspace(id:$id, name:"second2"){ id name } }`, map[string]any{"id": secondID}))
	r = run("alice", `{ workspaces { id name } }`, nil)
	mustOK(t, r)
	if !listHasNamed(r, secondID, "second2") {
		t.Fatalf("rename not visible in list: %+v", r.Data)
	}

	// (4) a non-admin cannot rename or delete — enforced by real OpenFGA
	// (bob has no tuple on workspace:<secondID>).
	r = run("bob", `mutation($id:String!){ renameWorkspace(id:$id, name:"hijack"){ id } }`, map[string]any{"id": secondID})
	if len(r.Errors) == 0 || !strings.Contains(strings.ToLower(r.Errors[0].Message), "forbidden") {
		t.Fatalf("non-admin rename: want forbidden, got %v", r.Errors)
	}
	r = run("bob", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"second2") }`, map[string]any{"id": secondID})
	if len(r.Errors) == 0 || !strings.Contains(strings.ToLower(r.Errors[0].Message), "forbidden") {
		t.Fatalf("non-admin delete: want forbidden, got %v", r.Errors)
	}

	// (5) delete tears down the workspace's App CRs + FGA tuple + rows.
	// Seed an app row under the workspace and project it into an App CR.
	if _, err := st.CreateApp(ctx, store.App{TenantID: secondID, Name: "web", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); err != nil {
		t.Fatalf("seed app: %v", err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n := managedAppCRCount(t, cl); n != 1 {
		t.Fatalf("App CR not projected: count=%d", n)
	}
	// Wrong confirmation is a no-op (guard).
	r = run("alice", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"WRONG") }`, map[string]any{"id": secondID})
	if len(r.Errors) == 0 {
		t.Fatal("wrong confirmation should error")
	}
	if _, err := st.GetTenant(ctx, secondID); err != nil {
		t.Fatalf("workspace destroyed on wrong confirmation: %v", err)
	}
	// Correct delete.
	mustOK(t, run("alice", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"second2") }`, map[string]any{"id": secondID}))
	// Row gone (404-shaped).
	if _, err := st.GetTenant(ctx, secondID); err == nil {
		t.Fatal("tenants row not deleted")
	}
	// App CR pruned (the kick reconciled synchronously).
	if n := managedAppCRCount(t, cl); n != 0 {
		t.Fatalf("App CR not pruned on workspace delete: count=%d", n)
	}
	// FGA admin tuple revoked — alice can no longer manage the deleted workspace.
	// Verified through a COLD checker: the working checker caches positive
	// results (revocation is TTL-eventual through a warm cache, by design), and
	// the earlier authorize/Check calls warmed it. A fresh checker reads OpenFGA
	// live, so a false here proves the tuple is actually gone.
	cold := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	if ok, err := cold.Check(ctx, "user:alice", "can_manage", core.WorkspaceObject(secondID)); err != nil || ok {
		t.Fatalf("FGA tuple not revoked on delete: ok=%v err=%v", ok, err)
	}
	// The deleted workspace no longer appears in alice's list.
	r = run("alice", `{ workspaces { id } }`, nil)
	mustOK(t, r)
	if listHasID(r, secondID) {
		t.Fatal("deleted workspace still listed")
	}

	// (6) REST /v1/owners stays free of mutations (Render parity: owners is
	// read-only; m1 adds no REST at all).
	restMux := srv.restHandler()
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		rr := httptest.NewRecorder()
		restMux.ServeHTTP(rr, httptest.NewRequest(method, "/v1/owners", nil))
		if rr.Code != 404 {
			t.Errorf("%s /v1/owners = %d, want 404 (no mutation route)", method, rr.Code)
		}
	}
}

func listHasNamed(r *graphql.Result, id, name string) bool {
	for _, w := range workspaceList(r) {
		if w["id"] == id && w["name"] == name {
			return true
		}
	}
	return false
}

func listHasID(r *graphql.Result, id string) bool {
	for _, w := range workspaceList(r) {
		if w["id"] == id {
			return true
		}
	}
	return false
}

func workspaceList(r *graphql.Result) []map[string]any {
	data, _ := r.Data.(map[string]any)
	raw, _ := data["workspaces"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// managedAppCRCount counts the reconciler-managed App CRs in the fake client.
func managedAppCRCount(t *testing.T, cl client.Client) int {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list App CRs: %v", err)
	}
	n := 0
	for i := range list.Items {
		if list.Items[i].Labels[store.LabelManagedBy] == store.ManagedByValue {
			n++
		}
	}
	return n
}
