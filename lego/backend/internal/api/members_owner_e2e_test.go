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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// members_owner_e2e_test.go is the row↔tuple half of the membership-invariant
// matrix (w5/m101, amended by w5/m372), against REAL infrastructure — live
// Postgres AND live OpenFGA, enforced — driving the REAL REST router: the self
// refusals and the creator's demotion and removal must leave the row, its
// authorization tuple and the onboarding binding agreeing. A write that revoked
// the tuple but kept the row (or the reverse) is the half-state the ordering
// exists to prevent, and only a real checker can show it.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestMembershipInvariantsE2E -v
//
// The OpenFGA at that URL must have a store named "bex" carrying
// deploy/gitops/authz/model.json (scripts/authz-model.sh). Additive like its
// siblings: every subject and workspace name is unique to the run.
func TestMembershipInvariantsE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
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
	owner, admin2, teammate := "owner-"+run, "admin2-"+run, "dev-"+run

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)

	// A personal workspace, so the owner binding is real (only onboarding's
	// CreateTenantWithMember sets owner_identity_id).
	tenant, err := st.CreateTenantWithMember(ctx, owner, store.PlanPro)
	if err != nil {
		t.Fatalf("mint personal workspace: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, tenant.ID, "user:"+owner, "admin"); err != nil {
		t.Fatalf("grant owner admin: %v", err)
	}
	for _, m := range []struct{ subject, role string }{{admin2, "admin"}, {teammate, "developer"}} {
		if err := st.AddMember(ctx, m.subject, tenant.ID, m.role); err != nil {
			t.Fatalf("add %s: %v", m.subject, err)
		}
		if err := roles.GrantWorkspaceRole(ctx, tenant.ID, "user:"+m.subject, m.role); err != nil {
			t.Fatalf("grant %s %s: %v", m.subject, m.role, err)
		}
	}

	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz:     checker,
		Workspace: NewTenantService(st, roles),
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
	})
	mux := srv.restHandler()

	// hasTuple asks the REAL checker whether the subject still holds the role.
	hasTuple := func(t *testing.T, subject, relation string) bool {
		t.Helper()
		ok, err := checker.Check(ctx, "user:"+subject, relation, "workspace:"+tenant.ID)
		if err != nil {
			t.Fatalf("check %s %s: %v", subject, relation, err)
		}
		return ok
	}
	isMember := func(t *testing.T, subject string) bool {
		t.Helper()
		_, err := st.GetTenantMember(ctx, tenant.ID, subject)
		return err == nil
	}
	codeOf := func(t *testing.T, rec *httptest.ResponseRecorder) string {
		t.Helper()
		var body struct{ Code string }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
		return body.Code
	}

	membersPath := "/v1/workspaces/" + tenant.ID + "/members/"

	binding := func(t *testing.T) string {
		t.Helper()
		b, err := st.TenantOwnerSubject(ctx, tenant.ID)
		if err != nil {
			t.Fatalf("owner binding: %v", err)
		}
		return b
	}

	// 1. admin2 tries to remove themselves — refused, unchanged.
	rec := e2eCall(mux, ctx, admin2, "DELETE", membersPath+admin2, "")
	if rec.Code != http.StatusConflict || codeOf(t, rec) != members.ErrorCannotRemoveSelf {
		t.Fatalf("self removal: %d %s", rec.Code, rec.Body)
	}
	if !isMember(t, admin2) || !hasTuple(t, admin2, "admin") {
		t.Errorf("caller half-removed: row=%v tuple=%v", isMember(t, admin2), hasTuple(t, admin2, "admin"))
	}

	// 2. admin2 tries to change their own role — refused, role unchanged in both
	//    the row and OpenFGA.
	rec = e2eCall(mux, ctx, admin2, "PATCH", membersPath+admin2, `{"role":"VIEWER"}`)
	if rec.Code != http.StatusConflict || codeOf(t, rec) != members.ErrorCannotChangeOwnRole {
		t.Fatalf("self role change: %d %s", rec.Code, rec.Body)
	}
	if hasTuple(t, admin2, "viewer") || !hasTuple(t, admin2, "admin") {
		t.Error("caller's own role changed in OpenFGA despite the refusal")
	}

	// 3. admin2 demotes the workspace's creator — allowed, as on Render
	//    (w5/m372): the creator is an ordinary Admin. The role moves in the row
	//    and OpenFGA, and the onboarding binding ends, so a session cannot
	//    restore the admin tuple.
	rec = e2eCall(mux, ctx, admin2, "PATCH", membersPath+owner, `{"role":"DEVELOPER"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("creator demotion: %d %s", rec.Code, rec.Body)
	}
	if hasTuple(t, owner, "admin") || !hasTuple(t, owner, "developer") {
		t.Errorf("creator's tuples after demotion: admin=%v developer=%v", hasTuple(t, owner, "admin"), hasTuple(t, owner, "developer"))
	}
	if b := binding(t); b != "" {
		t.Errorf("owner binding = %q after demoting the creator, want none", b)
	}

	// 4. Removing an ordinary teammate drops BOTH the row and the tuple.
	rec = e2eCall(mux, ctx, admin2, "DELETE", membersPath+teammate, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove teammate: %d %s", rec.Code, rec.Body)
	}
	if isMember(t, teammate) || hasTuple(t, teammate, "developer") {
		t.Errorf("teammate half-removed: row=%v tuple=%v", isMember(t, teammate), hasTuple(t, teammate, "developer"))
	}

	// 5. admin2 promotes the creator back to Admin: the role returns, the
	//    binding does not (it ended with the demotion).
	rec = e2eCall(mux, ctx, admin2, "PATCH", membersPath+owner, `{"role":"ADMIN"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("creator re-promotion: %d %s", rec.Code, rec.Body)
	}
	if !hasTuple(t, owner, "admin") || hasTuple(t, owner, "developer") {
		t.Errorf("creator's tuples after re-promotion: admin=%v developer=%v", hasTuple(t, owner, "admin"), hasTuple(t, owner, "developer"))
	}
	if b := binding(t); b != "" {
		t.Errorf("owner binding = %q after re-promotion, want it still ended", b)
	}

	// 6. admin2 removes the creator while still bound — Render's DELETE answers
	//    204 (w5/m372): row, tuple and binding end together. The binding is
	//    restored first, as for a creator who was never demoted.
	if _, err := pool.Exec(ctx, `UPDATE tenants SET owner_identity_id = $2 WHERE id = $1`, tenant.ID, owner); err != nil {
		t.Fatalf("restore the creator's binding: %v", err)
	}
	rec = e2eCall(mux, ctx, admin2, "DELETE", membersPath+owner, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove creator: %d %s", rec.Code, rec.Body)
	}
	if isMember(t, owner) || hasTuple(t, owner, "admin") {
		t.Errorf("creator half-removed: row=%v admin tuple=%v", isMember(t, owner), hasTuple(t, owner, "admin"))
	}
	if b := binding(t); b != "" {
		t.Errorf("owner binding = %q after removing the creator, want none", b)
	}
}
