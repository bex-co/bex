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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// envgroups_ownerid_e2e_test.go is w6/m24's end-to-end proof (t009), the
// env-groups mirror of readside_ownerid_e2e_test.go's TestReadSideOwnerIDTargetingE2E:
// before this milestone, envgroups.Service gated every verb on the CALLER's own
// workspace while groups lived unattributed in one shared store namespace — a
// caller in workspace A could list, get, and reveal workspace B's env groups (and
// link B's group into A's service) merely by knowing/guessing the id. This
// proves the cross-tenant isolation directly, against REAL infrastructure (a
// live Postgres + a live OpenFGA), same harness and skip condition as
// readside_ownerid_e2e_test.go:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run TestEnvGroupReadSideOwnerIDTargetingE2E -v
func TestEnvGroupReadSideOwnerIDTargetingE2E(t *testing.T) {
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

	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	dana, erin := "dana-"+run, "erin-"+run
	ws := func(kind string) string { return kind + "-" + run }

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	roles := checker.(store.MembershipGranter)

	wsA, err := st.CreateWorkspace(ctx, ws("alpha"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	wsB, err := st.CreateWorkspace(ctx, ws("bravo"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create bravo: %v", err)
	}
	for _, w := range []store.Tenant{wsA, wsB} {
		if err := granter.GrantWorkspaceAdmin(ctx, w.ID, "user:"+dana); err != nil {
			t.Fatalf("grant admin on %s: %v", w.Name, err)
		}
	}

	tenantSvc := NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz:     checker,
		Workspace: tenantSvc,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, WorkspaceGranter: granter,
		Secrets: newMemSecretStore(),
	})
	mux := srv.restHandler()
	call := func(subject, method, path, body string) *http.Response {
		return e2eCall(mux, ctx, subject, method, path, body).Result()
	}

	// A group MINTED in bravo (explicit ownerId) is attributed there.
	createResp := call(dana, "POST", "/v1/env-groups", `{"name":"shared-alpha"}`) // no ownerId => alpha
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create shared-alpha: %d", createResp.StatusCode)
	}
	groupA := decodeJSON[envgroups.EnvGroupView](t, createResp)
	if groupA.OwnerID != wsA.ID {
		t.Fatalf("shared-alpha ownerId = %q, want alpha %q", groupA.OwnerID, wsA.ID)
	}

	createResp = call(dana, "POST", "/v1/env-groups", `{"name":"shared-bravo","ownerId":"`+wsB.ID+`"}`)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create shared-bravo: %d", createResp.StatusCode)
	}
	groupB := decodeJSON[envgroups.EnvGroupView](t, createResp)
	if groupB.OwnerID != wsB.ID {
		t.Fatalf("shared-bravo ownerId = %q, want bravo %q", groupB.OwnerID, wsB.ID)
	}
	if groupB.CreatedAt == "" || groupB.UpdatedAt == "" {
		t.Errorf("created group missing timestamps: %+v", groupB)
	}

	// --- list: no ownerId (default alpha) sees only shared-alpha; ownerId=bravo
	// sees only shared-bravo — before this milestone List had NO tenant filter at
	// all and returned every workspace's groups. ---

	alphaList := decodeJSON[[]envgroups.EnvGroupView](t, call(dana, "GET", "/v1/env-groups", ""))
	if !onlyContainsGroup(alphaList, groupA.ID) {
		t.Errorf("GET /v1/env-groups (no ownerId) = %+v, want exactly [%s]", idsOfGroups(alphaList), groupA.ID)
	}

	bravoList := decodeJSON[[]envgroups.EnvGroupView](t, call(dana, "GET", "/v1/env-groups?ownerId="+wsB.ID, ""))
	if !onlyContainsGroup(bravoList, groupB.ID) {
		t.Errorf("GET /v1/env-groups?ownerId=bravo = %+v, want exactly [%s]", idsOfGroups(bravoList), groupB.ID)
	}

	// erin, a member of neither workspace, cannot list bravo's groups.
	if resp := call(erin, "GET", "/v1/env-groups?ownerId="+wsB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin GET /v1/env-groups?ownerId=bravo: %d, want 403", resp.StatusCode)
	}

	// --- get/reveal: erin cannot reach bravo's group by id at all, even
	// without naming ownerId — the resource's OWN workspace gates it. ---

	if resp := call(erin, "GET", "/v1/env-groups/"+groupB.ID, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin GET /v1/env-groups/%s: %d, want 403", groupB.ID, resp.StatusCode)
	}

	if resp := call(dana, "PUT", "/v1/env-groups/"+groupB.ID+"/env-vars", `[{"key":"TOKEN","value":"s3cret"}]`); resp.StatusCode != http.StatusOK {
		t.Fatalf("seed bravo's TOKEN: %d", resp.StatusCode)
	}
	if resp := call(erin, "GET", "/v1/env-groups/"+groupB.ID+"/env-vars/TOKEN", ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("erin reveal bravo's TOKEN: %d, want 403", resp.StatusCode)
	}
	// dana, who owns both, can still reveal it directly.
	revealed := decodeJSON[envgroups.EnvVarView](t, call(dana, "GET", "/v1/env-groups/"+groupB.ID+"/env-vars/TOKEN", ""))
	if revealed.Value != "s3cret" {
		t.Errorf("dana reveal bravo's TOKEN: %+v", revealed)
	}

	// --- link: alpha's own service may link alpha's own group, but not
	// bravo's — a write-side variant of the read leak (t004). ---

	if resp := call(dana, "POST", "/v1/services", `{"name":"web-`+run+`","image":{"imagePath":"nginx:latest"}}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create web service: %d", resp.StatusCode)
	}
	svcName := "web-" + run

	if resp := call(dana, "POST", "/v1/env-groups/"+groupB.ID+"/services/"+svcName, ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("link bravo's group into alpha's service: %d, want 403", resp.StatusCode)
	}
	if resp := call(dana, "POST", "/v1/env-groups/"+groupA.ID+"/services/"+svcName, ""); resp.StatusCode != http.StatusNoContent {
		t.Errorf("link alpha's own group into alpha's service: %d, want 204", resp.StatusCode)
	}
}

func onlyContainsGroup(groups []envgroups.EnvGroupView, id string) bool {
	if len(groups) != 1 {
		return false
	}
	return groups[0].ID == id
}

func idsOfGroups(groups []envgroups.EnvGroupView) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.ID
	}
	return out
}
