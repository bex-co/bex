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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// w6013_e2e_test.go is w6/m17's live proof (t005), against REAL infrastructure
// — the same harness multiworkspace_e2e_test.go uses (real Postgres, real
// OpenFGA, the real REST router, the real store-backed resolver) — driving the
// EXACT scenario w6/013 diagnosed: an invited viewer's default workspace is
// often the workspace they were invited into, not the one they own, because
// EnsureTenant redeems a pending invite before minting the invitee's personal
// tenant (so the invited membership is older — the default).
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run TestW6013_InvitedViewerRestartsOwnWorkspaceServiceE2E -v
func TestW6013_InvitedViewerRestartsOwnWorkspaceServiceE2E(t *testing.T) {
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

	// Additive, never truncates (see multiworkspace_e2e_test.go) — every name
	// and subject is unique to this run.
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	bob, alice := "bob-"+run, "alice-"+run

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	roles := checker.(store.MembershipGranter)

	// tea-team: alice's workspace. bob is INVITED as a viewer — his FIRST
	// (oldest) membership, so it becomes his default (store.TenantForIdentity's
	// ORDER BY created_at).
	teamWS, err := st.CreateWorkspace(ctx, "team-"+run, store.PlanHobby, alice)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := granter.GrantWorkspaceAdmin(ctx, teamWS.ID, "user:"+alice); err != nil {
		t.Fatalf("grant admin on team: %v", err)
	}
	if err := st.AddMember(ctx, bob, teamWS.ID, "viewer"); err != nil {
		t.Fatalf("add bob to team: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, teamWS.ID, "user:"+bob, "viewer"); err != nil {
		t.Fatalf("grant bob viewer on team: %v", err)
	}

	// tea-mine: bob's OWN workspace, created (and so joined) AFTER team — his
	// SECOND (newer) membership. He is its admin.
	mineWS, err := st.CreateWorkspace(ctx, "mine-"+run, store.PlanHobby, bob)
	if err != nil {
		t.Fatalf("create mine: %v", err)
	}
	if err := granter.GrantWorkspaceAdmin(ctx, mineWS.ID, "user:"+bob); err != nil {
		t.Fatalf("grant admin on mine: %v", err)
	}

	// (0) bob's DEFAULT workspace is team — his OLDEST membership — even
	// though he owns and administers mine. This is the exact precondition
	// w6/013 traces to EnsureTenant's invite-before-mint ordering.
	if got, err := st.TenantForIdentity(ctx, bob); err != nil || got.ID != teamWS.ID {
		t.Fatalf("bob's default workspace = %+v (err %v), want team %s", got, err, teamWS.ID)
	}

	cl := fakeClient()
	base := &core.Base{
		Client: cl, Namespace: "default",
		Authz:     checker,
		Workspace: NewTenantService(st, roles),
	}
	srv := NewServer(base, Deps{Store: st, WorkspaceStore: st, WorkspaceGranter: granter})
	mux := srv.restHandler()

	call := func(subject, method, path, body string) *httptest.ResponseRecorder {
		return e2eCall(mux, ctx, subject, method, path, body)
	}

	mineWeb := "mine-web-" + run

	// (1) bob creates "mine-web" NAMING his own workspace (mine) — the create
	// path already worked before this milestone (w6/m14).
	rec := call(bob, "POST", "/v1/services", `{"name":"`+mineWeb+`","image":{"imagePath":"nginx"},"ownerId":"`+mineWS.ID+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services ownerId=mine: %d %s", rec.Code, rec.Body.String())
	}

	// (2) THE w6/013 REGRESSION: bob restarts his OWN service with NO ownerId —
	// implicit resolution picks his default (team), where he is only a viewer.
	// Before this milestone, the verb's own Authorize ran against team FIRST
	// and 403'd him there, before the resource-side gate (team's mine's
	// workspace) ever got a say. It must succeed.
	if rec := call(bob, "POST", "/v1/services/"+mineWeb+"/restart", ""); rec.Code != http.StatusOK {
		t.Fatalf("bob restarts his own service (implicit resolution = team, where he is a viewer): %d %s — want 200; "+
			"a resource-scoped verb must authorize against the RESOURCE's workspace, not the caller's default",
			rec.Code, rec.Body.String())
	}

	// ...and the same holds for suspend and delete, the other verbs w6/013
	// names explicitly.
	if rec := call(bob, "POST", "/v1/services/"+mineWeb+"/suspend", ""); rec.Code != http.StatusAccepted {
		t.Errorf("bob suspends his own service: %d %s — want 202", rec.Code, rec.Body.String())
	}
	if rec := call(bob, "POST", "/v1/services/"+mineWeb+"/resume", ""); rec.Code != http.StatusAccepted {
		t.Errorf("bob resumes his own service: %d %s — want 202", rec.Code, rec.Body.String())
	}
	// (set-env-vars is w6/013's fourth named verb; live-verified separately in
	// secrets.TestW6013_InvitedViewerCanSetEnvVarsOnTheirOwnWorkspacesService —
	// this harness has no OpenBao, so the REST leg would only prove
	// core.ErrSecretsUnavailable is wired, not the workspace gate.)
	if rec := call(bob, "DELETE", "/v1/services/"+mineWeb, ""); rec.Code != http.StatusNoContent {
		t.Errorf("bob deletes his own service: %d %s — want 204", rec.Code, rec.Body.String())
	}
}
