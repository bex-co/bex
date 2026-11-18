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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// enrolledIdentities is metadataIdentities with a second factor for the
// subjects in enrolled.
type enrolledIdentities struct {
	metadataIdentities
	enrolled map[string]bool
}

func (e enrolledIdentities) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	return e.enrolled[subject], true, nil
}

// TestOrganizationLoginRequirementsE2E walks w7/m466 (docs/ADR012-auth.md §9c)
// over real Postgres, real OpenFGA and the real gate: an Owner turns org 2FA
// on on GraphQL; a Member cannot; the Member, with no second factor, is then
// refused in both org workspaces (the secondary's own setting off) on REST,
// GraphQL and MCP with the org named, and on the org's own pages, while the
// org list still shows why; the org log reads ChangeOrg2FAEnforcementEvent
// with metadata.to on REST and GraphQL, and no workspace log carries it;
// turning it off lets the Member back in at once.
func TestOrganizationLoginRequirementsE2E(t *testing.T) {
	var gate *twofactor.Gate
	var identities enrolledIdentities
	e := newOrgE2E(t, func(base *core.Base, d *Deps) {
		identities = enrolledIdentities{enrolled: map[string]bool{}}
		gate = twofactor.New(twofactor.Config{Settings: d.WorkspaceStore.(*store.PGStore), Enrollment: identities})
		base.TwoFactor = gate
		d.Identities = identities
		d.TwoFactorChanged = gate.Invalidate
	})
	ctx := context.Background()
	alice, bob := e.person("alice"), e.person("bob")
	identities.enrolled[alice.Subject] = true

	acme, err := e.st.CreateWorkspace(ctx, "acme-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.Pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'developer')`, acme.ID, bob.Subject); err != nil {
		t.Fatal(err)
	}
	if err := e.fga.(members.RoleGranter).GrantWorkspaceRole(ctx, acme.ID, "user:"+bob.Subject, "developer"); err != nil {
		t.Fatal(err)
	}
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { id } }`, acme.ID))
	orgID := e.ok(alice, `{ organizations { id } }`)["organizations"].([]any)[0].(map[string]any)["id"].(string)
	second := e.ok(alice, fmt.Sprintf(`mutation { createOrganizationWorkspace(organizationId: %q, name: "second-%s", privacy: "public") { id } }`, orgID, e.stamp))["createOrganizationWorkspace"].(map[string]any)["id"].(string)
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), second) })
	e.ok(bob, fmt.Sprintf(`mutation { joinWorkspace(workspaceId: %q) { joined } }`, second))

	// Only an Owner changes it; the Member reads it.
	e.refused(bob, fmt.Sprintf(`mutation { setOrganizationRequireTwoFactor(organizationId: %q, enabled: true) { id } }`, orgID), "FORBIDDEN")
	on := e.ok(alice, fmt.Sprintf(`mutation { setOrganizationRequireTwoFactor(organizationId: %q, enabled: true) { requireTwoFactor requireGoogleLogin } }`, orgID))["setOrganizationRequireTwoFactor"].(map[string]any)
	if on["requireTwoFactor"] != true || on["requireGoogleLogin"] != false {
		t.Fatalf("after enabling = %v", on)
	}
	// A repeat is a no-op: one row only (checked on the log below).
	e.ok(alice, fmt.Sprintf(`mutation { setOrganizationRequireTwoFactor(organizationId: %q, enabled: true) { id } }`, orgID))

	// bob, with no second factor, is refused in both org workspaces, naming
	// the org, on every surface.
	for _, ws := range []string{acme.ID, second} {
		status, body := restDo(t, e.srv, bob, http.MethodGet, "/v1/services?ownerId="+ws)
		params, _ := body["params"].(map[string]any)
		if status != http.StatusForbidden || body["code"] != twoFactorCode || params["organizationId"] != orgID || params["workspaceId"] != ws {
			t.Fatalf("REST in %s = %d %v, want 403 %s naming %s", ws, status, body, twoFactorCode, orgID)
		}
		e.refused(bob, fmt.Sprintf(`{ services(ownerId: %q) { id } }`, ws), twoFactorCode)
		e.mcpRefused(bob, "list_services", map[string]any{"workspaceId": ws}, twoFactorCode)
	}
	// The org's own objects refuse him too; the org list still shows why.
	e.refused(bob, fmt.Sprintf(`{ organizationMembers(organizationId: %q) { subject } }`, orgID), twoFactorCode)
	e.mcpRefused(bob, "list_organization_members", map[string]any{"organizationId": orgID}, twoFactorCode)
	orgs := e.ok(bob, `{ organizations { id requireTwoFactor } }`)["organizations"].([]any)
	if len(orgs) != 1 || orgs[0].(map[string]any)["requireTwoFactor"] != true {
		t.Fatalf("bob's exempt org list = %v", orgs)
	}
	var mcpOrg workspaces.OrganizationView
	raw, _ := json.Marshal(mcpCall(t, e.srv, bob, "get_organization", map[string]any{"organizationId": orgID}).StructuredContent)
	if err := json.Unmarshal(raw, &mcpOrg); err != nil || !mcpOrg.RequireTwoFactor {
		t.Fatalf("MCP get_organization as the blocked Member = %+v %v", mcpOrg, err)
	}
	// alice, enrolled, is unaffected; her workspace's own switch reads the org.
	if status, _ := restDo(t, e.srv, alice, http.MethodGet, "/v1/services?ownerId="+second); status != http.StatusOK {
		t.Fatalf("the enrolled Owner = %d", status)
	}
	ws := e.ok(alice, fmt.Sprintf(`{ workspace(id: %q) { requireTwoFactor organization { requireTwoFactor requireGoogleLogin } } }`, second))["workspace"].(map[string]any)
	if ws["requireTwoFactor"] != false || ws["organization"].(map[string]any)["requireTwoFactor"] != true {
		t.Fatalf("the secondary as alice sees it = %v, want its own setting off and the org's on", ws)
	}

	// The org log: one allowed ChangeOrg2FAEnforcementEvent with to=true (the
	// repeat wrote nothing), on REST and GraphQL; no workspace log carries it.
	status, raw := restRaw(t, e.srv, alice, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs?limit=100")
	if status != http.StatusOK {
		t.Fatalf("org audit log = %d %s", status, raw)
	}
	var rows []struct {
		AuditLog struct {
			Event    string            `json:"event"`
			Metadata map[string]string `json:"metadata"`
		} `json:"auditLog"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var enforcement []map[string]string
	for _, r := range rows {
		// bob's refused attempt is on the record too, as a denied row.
		if r.AuditLog.Event == "ChangeOrg2FAEnforcementEvent" && r.AuditLog.Metadata["outcome"] != "denied" {
			enforcement = append(enforcement, r.AuditLog.Metadata)
		}
	}
	if len(enforcement) != 1 || enforcement[0]["to"] != "true" {
		t.Fatalf("org log enforcement rows = %v, want one with to=true", enforcement)
	}
	gql := e.ok(alice, fmt.Sprintf(`{ organizationAuditLogs(organizationId: %q, limit: 100) { action metadata { to } } }`, orgID))["organizationAuditLogs"].([]any)
	var gqlTo []any
	for _, r := range gql {
		row := r.(map[string]any)
		if md, ok := row["metadata"].(map[string]any); ok && row["action"] == "ChangeOrg2FAEnforcementEvent" {
			gqlTo = append(gqlTo, md["to"])
		}
	}
	if len(gqlTo) != 1 || gqlTo[0] != true {
		t.Fatalf("GraphQL org log enforcement rows = %v", gqlTo)
	}
	for _, w := range []string{acme.ID, second} {
		_, raw := restRaw(t, e.srv, alice, http.MethodGet, "/v1/owners/"+w+"/audit-logs?limit=1000")
		if strings.Contains(string(raw), "ChangeOrg2FAEnforcementEvent") || strings.Contains(string(raw), "ChangeTeam2FAEnforcementEvent") {
			t.Fatalf("workspace %s's log carries the org's enforcement row: %s", w, raw)
		}
	}

	// Turning it off lets bob back in at once on this replica, and the
	// secondary's own setting is still off.
	e.ok(alice, fmt.Sprintf(`mutation { setOrganizationRequireTwoFactor(organizationId: %q, enabled: false) { id } }`, orgID))
	if status, _ := restDo(t, e.srv, bob, http.MethodGet, "/v1/services?ownerId="+second); status != http.StatusOK {
		t.Fatalf("bob after the org turned it off = %d, want 200", status)
	}
	if got, _ := e.st.GetTenant(ctx, second); got.RequireTwoFactor {
		t.Fatalf("the secondary's own setting = %+v", got)
	}
}
