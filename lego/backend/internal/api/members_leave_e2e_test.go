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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// members_leave_e2e_test.go is w5/m102's proof against REAL infrastructure —
// live Postgres and live OpenFGA, enforced — driving the REAL REST router: a
// member leaves, and both the row and the role tuple are gone; the owner and
// the last admin are refused; and the leaver's very next resolution no longer
// answers with the workspace they just left (the m13 stale-cache failure).
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestLeaveWorkspaceE2E -v
//
// Additive like its siblings: every subject is unique to the run.
func TestLeaveWorkspaceE2E(t *testing.T) {
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

	run := uniqueRunSuffix()
	owner, admin2, leaver := "owner-"+run, "admin2-"+run, "dev-"+run

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)

	// The workspace the caller leaves, plus a second one they keep — so the
	// post-leave resolution has somewhere correct to land.
	home, err := st.CreateTenantWithMember(ctx, owner, store.PlanPro)
	if err != nil {
		t.Fatalf("mint workspace: %v", err)
	}
	keep, err := st.CreateWorkspace(ctx, "keep-"+run, store.PlanPro, leaver)
	if err != nil {
		t.Fatalf("mint the leaver's own workspace: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, home.ID, "user:"+owner, "admin"); err != nil {
		t.Fatalf("grant owner: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, keep.ID, "user:"+leaver, "admin"); err != nil {
		t.Fatalf("grant leaver on their own workspace: %v", err)
	}
	for _, m := range []struct{ subject, role string }{{admin2, "admin"}, {leaver, "developer"}} {
		if err := st.AddMember(ctx, m.subject, home.ID, m.role); err != nil {
			t.Fatalf("add %s: %v", m.subject, err)
		}
		if err := roles.GrantWorkspaceRole(ctx, home.ID, "user:"+m.subject, m.role); err != nil {
			t.Fatalf("grant %s: %v", m.subject, err)
		}
	}

	resolver := NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz:     checker,
		Workspace: resolver,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
	})
	mux := srv.restHandler()
	leavePath := "/v1/workspaces/" + home.ID + "/members/me"

	codeOf := func(t *testing.T, body []byte) string {
		t.Helper()
		var decoded struct{ Code string }
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
		return decoded.Code
	}
	hasTuple := func(t *testing.T, subject, relation, tenantID string) bool {
		t.Helper()
		ok, err := checker.Check(ctx, "user:"+subject, relation, "workspace:"+tenantID)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		return ok
	}

	// 1. The owner cannot leave — the binding would be stranded exactly as a
	//    removal would strand it.
	rec := e2eCall(mux, ctx, owner, "DELETE", leavePath, "")
	if rec.Code != http.StatusConflict || codeOf(t, rec.Body.Bytes()) != members.ErrorOwnerCannotLeave {
		t.Fatalf("owner leaving: %d %s", rec.Code, rec.Body)
	}
	if _, err := st.GetTenantMember(ctx, home.ID, owner); err != nil {
		t.Fatalf("owner's row gone after a refused leave: %v", err)
	}

	// 2. Prime the resolver's caches the way a live request would, so step 4
	//    proves eviction rather than an empty cache.
	leaverID := core.Identity{Subject: leaver, Method: "session"}
	if _, ok := resolver.Tenant(ctx, leaverID); !ok {
		t.Fatal("leaver resolves to no workspace before leaving")
	}
	if member, err := resolver.IsMember(ctx, leaverID, home.ID); err != nil || !member {
		t.Fatalf("leaver is not a member of the workspace they are about to leave: %v %v", member, err)
	}

	// 3. The leave itself: row and tuple both gone.
	rec = e2eCall(mux, ctx, leaver, "DELETE", leavePath, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave: %d %s", rec.Code, rec.Body)
	}
	if _, err := st.GetTenantMember(ctx, home.ID, leaver); err == nil {
		t.Error("membership row survived the leave")
	}
	if hasTuple(t, leaver, "developer", home.ID) {
		t.Error("role tuple survived the leave")
	}

	// 4. The m13 regression: the leaver's very NEXT resolution must not answer
	//    with the workspace they just left, and their membership positive for it
	//    must be gone — no PositiveTTL window.
	if member, err := resolver.IsMember(ctx, leaverID, home.ID); err != nil || member {
		t.Errorf("stale membership positive for the workspace just left: member=%v err=%v", member, err)
	}
	if got, ok := resolver.Tenant(ctx, leaverID); !ok || got == home.ID {
		t.Errorf("post-leave resolution = %q (ok=%v), want the workspace they still belong to (%s)", got, ok, keep.ID)
	}

	// 5. The last admin cannot leave. admin2 is now the only admin of home
	//    (the owner is an admin too, so demote the owner out of the count by
	//    leaving them be and checking the true single-admin case on `keep`,
	//    where the leaver is the sole admin of their own workspace).
	rec = e2eCall(mux, ctx, leaver, "DELETE", "/v1/workspaces/"+keep.ID+"/members/me", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("last admin leaving: %d %s, want 400", rec.Code, rec.Body)
	}
	if _, err := st.GetTenantMember(ctx, keep.ID, leaver); err != nil {
		t.Fatalf("last admin's row gone after a refused leave: %v", err)
	}

	// 6. The route cannot be aimed at anyone else: "me" is a literal segment, so
	//    naming another subject falls through to the admin removal route, which
	//    a non-member of that workspace is not authorized for.
	rec = e2eCall(mux, ctx, leaver, "DELETE", "/v1/workspaces/"+home.ID+"/members/"+admin2, "")
	if rec.Code == http.StatusNoContent {
		t.Fatalf("a departed member removed someone else through the members route: %d", rec.Code)
	}
	if _, err := st.GetTenantMember(ctx, home.ID, admin2); err != nil {
		t.Errorf("admin2's membership was affected: %v", err)
	}
}
