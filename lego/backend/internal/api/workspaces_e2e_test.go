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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeWorkspace is a map-backed core.WorkspaceResolver stand-in for the w6/m4
// teardown assertions below (mirroring the same helper in postgres/keyvalue's
// ownerid_test.go): it pins a subject's resolved tenant so CreatePostgres/
// CreateKeyValue/secrets verbs (which stamp whichever workspace
// core.Base.Workspace resolves the caller to — they take no explicit ownerId
// of their own, unlike the list verbs) land in the exact workspace this test
// is about to delete, rather than whichever of alice's several memberships the
// real store-backed resolver would otherwise pick. Identities not in the map
// resolve ok=false.
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
	r = run("bob", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"sudo delete workspace second2") }`, map[string]any{"id": secondID})
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
	// Wrong confirmation is a no-op (guard). The bare workspace name is
	// insufficient: Render's guard is the full "sudo delete workspace <name>"
	// phrase (w6/m5, docs/render-artifacts/workspace-lifecycle.md).
	r = run("alice", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"second2") }`, map[string]any{"id": secondID})
	if len(r.Errors) == 0 {
		t.Fatal("wrong confirmation should error")
	}
	if _, err := st.GetTenant(ctx, secondID); err != nil {
		t.Fatalf("workspace destroyed on wrong confirmation: %v", err)
	}
	// Correct delete.
	mustOK(t, run("alice", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"sudo delete workspace second2") }`, map[string]any{"id": secondID}))
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
	// read-only). 404 (no such route) and 405 (the path exists, this method does
	// not) both prove it — which one Go's mux returns depends on whether ANY
	// method is registered for the path, and w6/m7's read API registered GET, so
	// it became 405. The assertion is "not routed to a mutation", not a
	// particular refusal code.
	restMux := srv.restHandler()
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		rr := httptest.NewRecorder()
		restMux.ServeHTTP(rr, httptest.NewRequest(method, "/v1/owners", nil))
		if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /v1/owners = %d, want 404 or 405 (no mutation route)", method, rr.Code)
		}
	}

	// (7) w6/m4: a workspace delete tears down every managed Postgres, every
	// managed KeyValue, and (when a test OpenBao is available) every OpenBao
	// secret it owns — not just the tenant row and FGA tuples. A fresh
	// workspace on a non-Hobby plan (alice already holds her 5-Hobby cap from
	// section 2 above), so this doesn't interfere with the App-CR assertions
	// already made against secondID above.
	r = run("alice", createMut, map[string]any{"n": "datastores", "p": "pro"})
	mustOK(t, r)
	ws = r.Data.(map[string]any)["createWorkspace"].(map[string]any)
	dsID, _ := ws["id"].(string)

	// mustCreateWithOwner runs a create mutation, asserts the returned
	// object's ownerId, and returns its id (the CR's metadata.name since the
	// dpg-/red- identity split), cutting the createDatabase/createKeyValue
	// duplication.
	mustCreateWithOwner := func(mutation, field string, args map[string]any, wantOwner string) string {
		t.Helper()
		r := run("alice", mutation, args)
		mustOK(t, r)
		obj := r.Data.(map[string]any)[field].(map[string]any)
		if got := obj["ownerId"]; got != wantOwner {
			t.Fatalf("%s ownerId = %v, want %s", field, got, wantOwner)
		}
		id, _ := obj["id"].(string)
		return id
	}
	// mustListCount runs an ownerId-scoped list query and asserts its length,
	// cutting the databases/keyValues duplication.
	mustListCount := func(query, field string, args map[string]any, wantN int) {
		t.Helper()
		r := run("alice", query, args)
		mustOK(t, r)
		if got := r.Data.(map[string]any)[field].([]any); len(got) != wantN {
			t.Fatalf("%s(%v) = %+v, want %d", field, args, got, wantN)
		}
	}

	// Pin alice's resolved tenant to dsID for what follows: CreatePostgres/
	// CreateKeyValue/secrets take no explicit ownerId of their own, they stamp
	// whichever workspace core.Base.Workspace resolves the caller to. Every
	// feature Service embeds this same *core.Base pointer, so mutating the field
	// now only affects calls from here on — everything above already ran.
	base.Workspace = fakeWorkspace{"alice": dsID}

	// (7a) Render parity spot-check (t007): a SECOND, unrelated workspace with
	// its own Database must never leak into dsID's ownerId-scoped list below —
	// flip the resolver to it just long enough to create one, then back.
	r = run("alice", createMut, map[string]any{"n": "leakcheck", "p": "pro"})
	mustOK(t, r)
	leakID, _ := r.Data.(map[string]any)["createWorkspace"].(map[string]any)["id"].(string)
	base.Workspace = fakeWorkspace{"alice": leakID}
	mustOK(t, run("alice", `mutation($n:String!){ createDatabase(name:$n){ id } }`, map[string]any{"n": "leak-pg"}))
	base.Workspace = fakeWorkspace{"alice": dsID}

	// Seed an App under dsID for the secrets assertion (secrets.GetApp gates on
	// the App's own tenant label, same store-seeded path section 5 uses).
	if _, err := st.CreateApp(ctx, store.App{TenantID: dsID, Name: "worker", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); err != nil {
		t.Fatalf("seed worker app: %v", err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	mustCreateWithOwner(`mutation($n:String!){ createDatabase(name:$n){ id ownerId } }`,
		"createDatabase", map[string]any{"n": "ds-pg"}, dsID)
	kvID := mustCreateWithOwner(`mutation($n:String!){ createKeyValue(name:$n){ id ownerId } }`,
		"createKeyValue", map[string]any{"n": "ds-kv"}, dsID)

	// The KeyValue CR itself carries the workspace label the same-workspace
	// NetworkPolicy selector matches on (docs/ADR022-tenant-isolation.md) — the label
	// t002 stamps is what lets dsID's own App reach its own Valkey instance.
	// Since w9/m6 the CR is named by its immutable red- id; "ds-kv" is the
	// mutable display name in spec.name.
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: kvID}, &kv); err != nil {
		t.Fatalf("get KeyValue CR: %v", err)
	}
	if kv.Spec.Name != "ds-kv" {
		t.Fatalf("KeyValue CR spec.name = %q, want ds-kv", kv.Spec.Name)
	}
	if kv.Labels[core.LabelWorkspace] != dsID {
		t.Fatalf("KeyValue CR workspace label = %q, want %s", kv.Labels[core.LabelWorkspace], dsID)
	}

	// ownerId list-filter correctness pre-delete (t001/t002): scoped to dsID
	// returns exactly the seeded resources, never another workspace's (proven
	// by leak-pg above sharing the namespace under a different owner).
	mustListCount(`query($o:String!){ databases(ownerId:$o){ id } }`, "databases", map[string]any{"o": dsID}, 1)
	mustListCount(`query($o:String!){ keyValues(ownerId:$o){ id } }`, "keyValues", map[string]any{"o": dsID}, 1)

	// Secrets: only when a test OpenBao is available (BEX_TEST_OPENBAO_URL) — the
	// purge assertion needs a real KV v2 store, hermetic-by-default like DB/FGA
	// above.
	var secretsStore core.SecretKV
	var envGroupID string
	if baoURL := os.Getenv("BEX_TEST_OPENBAO_URL"); baoURL != "" {
		secretsStore = secrets.NewOpenBaoStore(baoURL)
		secretsSvc := &secrets.Service{Base: base, Store: secretsStore}
		aliceCtx := core.WithIdentity(ctx, core.Identity{Subject: "alice", Method: "session"})
		if _, err := secretsSvc.SetEnvVar(aliceCtx, "worker", "FOO", secrets.EnvVarWrite{Value: "bar"}); err != nil {
			t.Fatalf("seed secret: %v", err)
		}
		group, err := (&envgroups.Service{Base: base, Store: secretsStore}).CreateEnvGroup(aliceCtx, envgroups.CreateEnvGroupRequest{Name: "workspace-delete"})
		if err != nil {
			t.Fatalf("seed env group: %v", err)
		}
		envGroupID = group.ID
	}

	// Wire the purgers exactly as cmd/api/main.go does (t005) — mutating the
	// already-built srv.Workspaces in place takes effect immediately, since the
	// schema's resolvers read s.Purgers at call time through the same pointer.
	srv.Workspaces.Purgers = []workspaces.WorkspacePurger{
		&secrets.WorkspacePurger{Service: &secrets.Service{Base: base, Store: secretsStore}},
		&envgroups.WorkspacePurger{Service: &envgroups.Service{Base: base, Store: secretsStore}},
		&postgres.WorkspacePurger{Service: &postgres.Service{Base: base}},
		&keyvalue.WorkspacePurger{Service: &keyvalue.Service{Base: base}},
	}
	mustOK(t, run("alice", `mutation($id:String!){ deleteWorkspace(id:$id, confirmation:"sudo delete workspace datastores") }`, map[string]any{"id": dsID}))

	var dbList appv1alpha1.DatabaseList
	if err := cl.List(ctx, &dbList, client.MatchingLabels{core.LabelTenant: dsID}); err != nil {
		t.Fatalf("list Databases: %v", err)
	}
	if len(dbList.Items) != 0 {
		t.Fatalf("Database CR not purged on workspace delete: %+v", dbList.Items)
	}
	var kvList appv1alpha1.KeyValueList
	if err := cl.List(ctx, &kvList, client.MatchingLabels{core.LabelTenant: dsID}); err != nil {
		t.Fatalf("list KeyValues: %v", err)
	}
	if len(kvList.Items) != 0 {
		t.Fatalf("KeyValue CR not purged on workspace delete: %+v", kvList.Items)
	}
	if secretsStore != nil {
		env, err := secretsStore.Get(ctx, "services/worker/env")
		if err != nil {
			t.Fatalf("read purged secret: %v", err)
		}
		if len(env) != 0 {
			t.Fatalf("secret not purged on workspace delete: %+v", env)
		}
		meta, err := secretsStore.Get(ctx, "env-groups/"+envGroupID+"/meta")
		if err != nil {
			t.Fatalf("read purged env-group meta: %v", err)
		}
		if len(meta) != 0 {
			t.Fatalf("env group not purged on workspace delete: %+v", meta)
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
