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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

type subjectScopedEnterprise struct {
	mu      sync.Mutex
	allowed map[string]bool
	seen    []string
}

func (e *subjectScopedEnterprise) CheckEnterpriseEntitlement(_ context.Context, subject, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seen = append(e.seen, subject)
	if !e.allowed[subject] {
		return core.NewBadRequestError("ENTERPRISE_ENTITLEMENT_REQUIRED", "Enterprise workspace plans require explicit operator approval", nil)
	}
	return nil
}

// TestChangeWorkspacePlanAPIKeyParityE2E (w34/013) exercises GraphQL
// changeWorkspacePlan with an API key holding KeyWorkspaceSettingsManage
// against real Postgres and real OpenFGA, proving parity with the key
// creator's human session across all four caller-attributed paths:
//
//   - requirePlanEntitlement receives the creator's subject, not the key id;
//   - upgradeToOrganization promotes only the key creator to Org Owner (other
//     admins stay Member);
//   - downgradePreconditions keeps the key creator in Hobby's one seat and
//     names other admins in member_limit;
//   - guardPerUserWorkspaceCap enforces the creator's 5-Hobby-workspace cap
//     on standalone and org downgrades alike.
func TestChangeWorkspacePlanAPIKeyParityE2E(t *testing.T) {
	run := uniqueRunSuffix()
	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	ent := &subjectScopedEnterprise{allowed: map[string]bool{}}

	e := newOrgE2E(t, func(base *core.Base, deps *Deps) {
		base.KeyCreators = keys
		base.KeyGrants = base.Workspace.(core.KeyGrantResolver)
		deps.APIKeys = keys
		deps.KeyBinder = base.Workspace.(apikeys.KeyBinder)
		deps.EnterpriseEntitlement = ent
	})
	ctx := context.Background()
	mux := e.srv.restHandler()
	gatedREST := e.srv.withScopeClassREST(mux, mux)

	alice := e.person("alice")
	bob := e.person("bob")
	carol := e.person("carol")

	ent.mu.Lock()
	ent.allowed[bob.Subject] = true
	ent.mu.Unlock()

	keyOf := func(id string) core.Identity {
		return core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2}
	}
	mintPlanKey := func(creator core.Identity, workspaceID string) core.Identity {
		t.Helper()
		body := fmt.Sprintf(`{"name":"plan-key","ownerId":%q,"permissions":{"grants":[{"permission":"workspaces","level":"read_write"}]}}`, workspaceID)
		req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gatedREST.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, creator)))
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil {
			t.Fatalf("%s mints a key on %s: %d %s", creator.Subject, workspaceID, rec.Code, rec.Body)
		}
		return keyOf(key.ID)
	}

	// Workspace with alice (admin, joined first), bob (admin, joined second),
	// and carol (developer).
	acme, err := e.st.CreateWorkspace(ctx, "plan-key-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ subject, role string }{{bob.Subject, "admin"}, {carol.Subject, "developer"}} {
		if err := e.st.AddMember(ctx, m.subject, acme.ID, m.role); err != nil {
			t.Fatal(err)
		}
		if err := e.fga.(members.RoleGranter).GrantWorkspaceRole(ctx, acme.ID, "user:"+m.subject, m.role); err != nil {
			t.Fatal(err)
		}
	}
	aliceKey := mintPlanKey(alice, acme.ID)
	bobKey := mintPlanKey(bob, acme.ID)

	// 1. requirePlanEntitlement: alice is not entitled for Enterprise (her key
	//    is refused just like her session); bob is entitled, and bob's key
	//    passes entitlement under bob.Subject — NOT bobKey.Subject — creating
	//    the Enterprise org with ONLY bob as Owner.
	e.refused(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "enterprise") { id plan } }`, acme.ID), "ENTERPRISE_ENTITLEMENT_REQUIRED")
	e.refused(aliceKey, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "enterprise") { id plan } }`, acme.ID), "ENTERPRISE_ENTITLEMENT_REQUIRED")

	// 2. upgradeToOrganization: bob's API key upgrades acme to Enterprise.
	//    Only bob becomes Org Owner; alice (the other admin) and carol stay
	//    Org Members — matching bob's human session.
	upgraded := e.ok(bobKey, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "enterprise") { id plan organization { id } } }`, acme.ID))["changeWorkspacePlan"].(map[string]any)
	if upgraded["plan"] != "enterprise" {
		t.Fatalf("upgraded workspace = %v", upgraded)
	}
	orgID := upgraded["organization"].(map[string]any)["id"].(string)

	ent.mu.Lock()
	seen := slices.Clone(ent.seen)
	ent.mu.Unlock()
	if !slices.Equal(seen, []string{alice.Subject, alice.Subject, bob.Subject}) {
		t.Fatalf("entitlement checked subjects = %v, want [%s %s %s]", seen, alice.Subject, alice.Subject, bob.Subject)
	}

	orgMembers, err := e.st.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, m := range orgMembers {
		roles[m.Subject] = m.Role
	}
	if roles[bob.Subject] != store.OrgRoleOwner || roles[alice.Subject] != store.OrgRoleMember || roles[carol.Subject] != store.OrgRoleMember {
		t.Fatalf("org member roles after API-key upgrade = %v, want bob=owner alice=member carol=member", roles)
	}

	// 3. downgradePreconditions (unseated): bob's API key asks to downgrade to
	//    Hobby. Like bob's human session, bob keeps Hobby's single seat while
	//    alice (first-joined admin) and carol are listed in member_limit.
	hobbyMutation := fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "hobby") { plan } }`, acme.ID)
	humanDowngrade := e.gql(bob, hobbyMutation)
	keyDowngrade := e.gql(bobKey, hobbyMutation)
	if graphqlCode(humanDowngrade) != core.CodeOrganizationDowngradeBlocked || graphqlCode(keyDowngrade) != core.CodeOrganizationDowngradeBlocked {
		t.Fatalf("downgrade codes: human=%v key=%v, want %s", humanDowngrade, keyDowngrade, core.CodeOrganizationDowngradeBlocked)
	}
	wantUnseated := []string{alice.Subject + "@example.com:admin", carol.Subject + "@example.com:developer"}
	humanItems := preconditionItems(humanDowngrade)[core.PreconditionMemberLimit]
	keyItems := preconditionItems(keyDowngrade)[core.PreconditionMemberLimit]
	if !slices.Equal(humanItems, wantUnseated) || !slices.Equal(keyItems, wantUnseated) {
		t.Fatalf("member_limit items: human=%v key=%v want=%v", humanItems, keyItems, wantUnseated)
	}

	// 4. guardPerUserWorkspaceCap: alice already has 1 personal Hobby workspace
	//    from onboarding. Give her 4 more Hobby workspaces (5 total) plus a
	//    standalone Pro workspace, then try downgrading the Pro workspace via
	//    alice's API key — must be refused just like alice's human session.
	for i := range 4 {
		hw, err := e.st.CreateWorkspace(ctx, fmt.Sprintf("hobby-%d-%s", i, e.stamp), store.PlanHobby, alice.Subject)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), hw.ID) })
	}
	sixth, err := e.st.CreateWorkspace(ctx, "sixth-pro-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), sixth.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, sixth.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	sixthKey := mintPlanKey(alice, sixth.ID)
	expectErrorContaining := func(id core.Identity, query, wantSubstr string) {
		t.Helper()
		out := e.gql(id, query)
		errs, _ := out["errors"].([]any)
		if len(errs) == 0 {
			t.Fatalf("%q as %s succeeded (%v), want error containing %q", query, id.Subject, out, wantSubstr)
		}
		msg, _ := errs[0].(map[string]any)["message"].(string)
		if !strings.Contains(msg, wantSubstr) {
			t.Fatalf("%q as %s error = %q, want substring %q", query, id.Subject, msg, wantSubstr)
		}
	}
	sixthDowngrade := fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "hobby") { plan } }`, sixth.ID)
	expectErrorContaining(alice, sixthDowngrade, "at most 5 hobby workspaces per user")
	expectErrorContaining(sixthKey, sixthDowngrade, "at most 5 hobby workspaces per user")
	if got, err := e.st.GetTenant(ctx, sixth.ID); err != nil || got.Plan != store.PlanPro {
		t.Fatalf("sixth workspace after refused API-key downgrade = %+v %v, want pro", got, err)
	}

	// 5. Same 5-Hobby cap on an org primary downgrade via bob's API key:
	//    remove alice and carol from acme so downgradePreconditions passes,
	//    give bob 4 more Hobby workspaces (5 total with his onboarding one),
	//    then confirm bob's API key is still refused and the org survives.
	if _, err := e.st.Pool.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id = $1 AND subject = ANY($2)`, acme.ID, []string{alice.Subject, carol.Subject}); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		hw, err := e.st.CreateWorkspace(ctx, fmt.Sprintf("bob-hobby-%d-%s", i, e.stamp), store.PlanHobby, bob.Subject)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), hw.ID) })
	}
	expectErrorContaining(bob, hobbyMutation, "at most 5 hobby workspaces per user")
	expectErrorContaining(bobKey, hobbyMutation, "at most 5 hobby workspaces per user")
	if got, err := e.st.GetTenant(ctx, acme.ID); err != nil || got.Plan != store.PlanEnterprise || got.OrganizationID != orgID {
		t.Fatalf("org primary after refused API-key downgrade = %+v %v, want enterprise in %s", got, err, orgID)
	}
}
