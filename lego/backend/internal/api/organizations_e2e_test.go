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
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// allowEnterprise approves every Enterprise selection: the operator's
// entitlement is not what these tests are about.
type allowEnterprise struct{}

func (allowEnterprise) CheckEnterpriseEntitlement(context.Context, string, string) error { return nil }

// orgE2E is a server over a real Postgres and a real OpenFGA, the way bex-api
// runs (w7/m464): every org verb writes its rows and its tuples, and the
// checks that gate the next verb are OpenFGA's own.
type orgE2E struct {
	t       *testing.T
	srv     *Server
	st      *store.PGStore
	tenants *tenantService
	fga     core.Checker
	stamp   string
}

func newOrgE2E(t *testing.T, opts ...func(*core.Base, *Deps)) *orgE2E {
	t.Helper()
	dbURI, fgaURL := os.Getenv("BEX_TEST_DB_URI"), os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	tenants := NewTenantService(st, checker.(store.MembershipGranter))
	tenants.Audit = st
	tenants.OrgTuples = checker.(core.OrganizationTupleWriter)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: tenants, Audit: st}
	deps := Deps{
		WorkspaceStore:        st,
		MembersStore:          st,
		WorkspaceGranter:      checker.(workspaces.WorkspaceGranter),
		WorkspaceRevoker:      checker.(workspaces.WorkspaceRevoker),
		MembersGranter:        checker.(members.RoleGranter),
		MembersRevoker:        checker.(members.RoleRevoker),
		OrgTuples:             checker.(core.OrganizationTupleWriter),
		EnterpriseEntitlement: allowEnterprise{},
		Identities:            metadataIdentities{},
		Audit:                 &audit.Service{Base: base, Store: st, UserIDs: st, Emails: metadataIdentities{}},
	}
	for _, opt := range opts {
		opt(base, &deps)
	}
	srv := NewServer(base, deps)
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv.schema = schema
	return &orgE2E{t: t, srv: srv, st: st, tenants: tenants, fga: checker, stamp: fmt.Sprintf("%d", time.Now().UnixNano())}
}

// person is a signed-in human with a personal Hobby workspace, as onboarding
// makes on first sign-in.
func (e *orgE2E) person(name string) core.Identity {
	e.t.Helper()
	subject := name + "-" + e.stamp
	if _, err := e.tenants.EnsureTenant(context.Background(), subject, subject+"@example.com", true); err != nil {
		e.t.Fatalf("onboard %s: %v", subject, err)
	}
	return core.Identity{Subject: subject, Method: "session", Human: true}
}

// signInAgain is a person's next sign-in: onboarding redeems the invites
// addressed to their email (the resolution cache would otherwise answer).
func (e *orgE2E) signInAgain(id core.Identity) {
	e.t.Helper()
	e.tenants.InvalidateTenant(id.Subject)
	if _, err := e.tenants.EnsureTenant(context.Background(), id.Subject, id.Subject+"@example.com", true); err != nil {
		e.t.Fatalf("sign in %s: %v", id.Subject, err)
	}
}

func (e *orgE2E) gql(id core.Identity, query string) map[string]any {
	e.t.Helper()
	return graphqlDo(e.t, e.srv, id, query)
}

// ok runs a GraphQL operation that must succeed and returns its data.
func (e *orgE2E) ok(id core.Identity, query string) map[string]any {
	e.t.Helper()
	out := e.gql(id, query)
	if errs, _ := out["errors"].([]any); len(errs) > 0 {
		e.t.Fatalf("%s as %s: %v", strings.SplitN(query, "(", 2)[0], id.Subject, errs)
	}
	data, _ := out["data"].(map[string]any)
	return data
}

// refused runs a GraphQL operation that must answer code.
func (e *orgE2E) refused(id core.Identity, query, code string) {
	e.t.Helper()
	if got := graphqlCode(e.gql(id, query)); got != code {
		e.t.Fatalf("%q as %s answered code %q, want %q", query, id.Subject, got, code)
	}
}

// mcpRefused calls an MCP tool that must answer code in its error text.
func (e *orgE2E) mcpRefused(id core.Identity, tool string, args map[string]any, code string) {
	e.t.Helper()
	res := mcpCall(e.t, e.srv, id, tool, args)
	if !res.IsError || !strings.Contains(mcpText(res), code) {
		e.t.Fatalf("%s as %s = %q (error %v), want %s", tool, id.Subject, mcpText(res), res.IsError, code)
	}
}

// TestOrganizationsE2E walks Render's organization lifecycle (w7/m464,
// docs/ADR115-organizations.md) through GraphQL, MCP and REST: an upgrade
// makes an org with its upgrader as Owner; the Owner creates Public and
// Invite-only workspaces on the org's plan; a Member joins only the Public
// one; an Owner joins any as admin but holds no workspace until then; a
// workspace invite makes a newcomer a Member, and a Guest gets one workspace;
// an org holds two Billing people; the org log is the Owner's and never mixes
// with a workspace's; a secondary's plan is the primary's; and a downgrade is
// refused by name until Render's preconditions hold, then deletes the org.
func TestOrganizationsE2E(t *testing.T) {
	e := newOrgE2E(t)
	ctx := context.Background()
	alice, bob, carol, dave := e.person("alice"), e.person("bob"), e.person("carol"), e.person("dave")

	// alice's workspace, with bob in it as a developer.
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

	// The upgrade to Scale creates the org: acme is the primary, alice the
	// Owner, bob a Member.
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { id plan organization { id } primary privacy } }`, acme.ID))
	orgs := e.ok(alice, `{ organizations { id name plan primaryWorkspaceId viewerRole workspaces { id primary privacy viewerIsMember } } }`)["organizations"].([]any)
	if len(orgs) != 1 {
		t.Fatalf("alice's organizations = %v", orgs)
	}
	org := orgs[0].(map[string]any)
	orgID := org["id"].(string)
	if org["primaryWorkspaceId"] != acme.ID || org["viewerRole"] != "OWNER" || org["plan"] != "scale" {
		t.Fatalf("org = %v", org)
	}
	if role := e.ok(bob, fmt.Sprintf(`{ organization(id: %q) { viewerRole } }`, orgID))["organization"].(map[string]any)["viewerRole"]; role != "MEMBER" {
		t.Fatalf("bob's org role = %v", role)
	}
	// A non-member is told the same thing for this org as for an unknown one,
	// on GraphQL and MCP.
	e.refused(dave, fmt.Sprintf(`{ organization(id: %q) { id } }`, orgID), core.CodeOrganizationNotFound)
	e.refused(dave, `{ organization(id: "org-doesnotexist00000000") { id } }`, core.CodeOrganizationNotFound)
	e.mcpRefused(dave, "get_organization", map[string]any{"organizationId": orgID}, core.CodeOrganizationNotFound)
	// GraphQL and MCP read the same org.
	var mcpOrg workspaces.OrganizationView
	raw, _ := json.Marshal(mcpCall(t, e.srv, alice, "get_organization", map[string]any{"organizationId": orgID}).StructuredContent)
	if err := json.Unmarshal(raw, &mcpOrg); err != nil || mcpOrg.ID != orgID || mcpOrg.ViewerRole != "OWNER" || mcpOrg.PrimaryWorkspaceID != acme.ID {
		t.Fatalf("MCP get_organization = %+v %v", mcpOrg, err)
	}

	// alice creates a Public workspace on GraphQL and an Invite-only one on
	// MCP: both on the org's plan, with no payment step.
	pub := e.ok(alice, fmt.Sprintf(`mutation { createOrganizationWorkspace(organizationId: %q, name: "pub-%s", privacy: "public") { id plan privacy primary } }`, orgID, e.stamp))["createOrganizationWorkspace"].(map[string]any)
	pubID := pub["id"].(string)
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), pubID) })
	if pub["plan"] != "scale" || pub["privacy"] != "public" || pub["primary"] != false {
		t.Fatalf("public workspace = %v", pub)
	}
	privRes := mcpCall(t, e.srv, alice, "create_organization_workspace", map[string]any{"organizationId": orgID, "name": "priv-" + e.stamp})
	if privRes.IsError {
		t.Fatalf("MCP create_organization_workspace: %s", mcpText(privRes))
	}
	var priv workspaces.WorkspaceView
	raw, _ = json.Marshal(privRes.StructuredContent)
	_ = json.Unmarshal(raw, &priv)
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), priv.ID) })
	if p, _ := e.st.GetTenant(ctx, priv.ID); p.Plan != store.PlanScale || p.Privacy != store.PrivacyInviteOnly || p.OrganizationID != orgID {
		t.Fatalf("invite-only workspace = %+v", p)
	}
	// Only an Owner creates org workspaces.
	e.refused(bob, fmt.Sprintf(`mutation { createOrganizationWorkspace(organizationId: %q, name: "nope-%s") { id } }`, orgID, e.stamp), "FORBIDDEN")

	// bob, a Member, joins the Public workspace as a Developer and is refused
	// the Invite-only one, on GraphQL and MCP alike.
	joined := e.ok(bob, fmt.Sprintf(`mutation { joinWorkspace(workspaceId: %q) { role joined } }`, pubID))["joinWorkspace"].(map[string]any)
	if joined["role"] != "DEVELOPER" || joined["joined"] != true {
		t.Fatalf("bob's join = %v", joined)
	}
	if out := e.gql(bob, fmt.Sprintf(`mutation { joinWorkspace(workspaceId: %q) { role } }`, priv.ID)); graphqlCode(out) == "" && out["errors"] == nil {
		t.Fatalf("bob joined an Invite-only workspace: %v", out)
	}
	if res := mcpCall(t, e.srv, bob, "join_workspace", map[string]any{"workspaceId": priv.ID}); !res.IsError {
		t.Fatalf("MCP join of an Invite-only workspace succeeded: %s", mcpText(res))
	}
	// bob is no Owner: no joining as admin.
	if res := mcpCall(t, e.srv, bob, "join_workspace_as_admin", map[string]any{"workspaceId": priv.ID}); !res.IsError {
		t.Fatalf("a Member joined as admin: %s", mcpText(res))
	}

	// dave enters the org as an Owner through an org invite redeemed at his
	// next sign-in. He is in no org workspace until he joins one as admin.
	e.ok(alice, fmt.Sprintf(`mutation { inviteOrganizationMember(organizationId: %q, email: %q, role: "OWNER") { id role } }`, orgID, dave.Subject+"@example.com"))
	e.signInAgain(dave)
	if role := e.ok(dave, fmt.Sprintf(`{ organization(id: %q) { viewerRole } }`, orgID))["organization"].(map[string]any)["viewerRole"]; role != "OWNER" {
		t.Fatalf("dave's org role after sign-in = %v", role)
	}
	if out := e.gql(dave, fmt.Sprintf(`{ workspace(id: %q) { id } }`, priv.ID)); out["errors"] == nil {
		t.Fatalf("an Owner read a workspace he never joined: %v", out)
	}
	if ok, _ := e.fga.(core.FreshChecker).CheckFresh(ctx, "user:"+dave.Subject, core.RelCanManage, core.WorkspaceObject(priv.ID)); ok {
		t.Fatal("an Owner is an implicit workspace admin")
	}
	asAdmin := e.ok(dave, fmt.Sprintf(`mutation { joinWorkspaceAsAdmin(workspaceId: %q) { role joined } }`, priv.ID))["joinWorkspaceAsAdmin"].(map[string]any)
	if asAdmin["role"] != "ADMIN" {
		t.Fatalf("dave's join as admin = %v", asAdmin)
	}
	e.ok(dave, fmt.Sprintf(`{ workspace(id: %q) { id } }`, priv.ID))

	// carol enters through a workspace invite to the Public workspace: she
	// becomes a Member of the org, as Render does without SSO. alice makes
	// her a Guest; a Guest sees only her workspace and not the members, and a
	// workspace invite to a second org workspace is refused by name.
	e.ok(alice, fmt.Sprintf(`mutation { inviteWorkspaceMember(workspaceId: %q, email: %q, role: "DEVELOPER") { id } }`, pubID, carol.Subject+"@example.com"))
	e.signInAgain(carol)
	if m, err := e.st.GetOrganizationMember(ctx, orgID, carol.Subject); err != nil || m.Role != store.OrgRoleMember {
		t.Fatalf("carol's org role after redemption = %+v %v", m, err)
	}
	e.ok(alice, fmt.Sprintf(`mutation { changeOrganizationMemberRole(organizationId: %q, subject: %q, role: "GUEST") { role } }`, orgID, carol.Subject))
	guestView := e.ok(carol, fmt.Sprintf(`{ organization(id: %q) { viewerRole workspaces { id } } }`, orgID))["organization"].(map[string]any)
	if ws := guestView["workspaces"].([]any); guestView["viewerRole"] != "GUEST" || len(ws) != 1 || ws[0].(map[string]any)["id"] != pubID {
		t.Fatalf("carol's view as a Guest = %v", guestView)
	}
	if out := e.gql(carol, fmt.Sprintf(`{ organizationMembers(organizationId: %q) { subject } }`, orgID)); out["errors"] == nil {
		t.Fatalf("a Guest listed the org's members: %v", out)
	}
	e.refused(alice, fmt.Sprintf(`mutation { inviteWorkspaceMember(workspaceId: %q, email: %q, role: "DEVELOPER") { id } }`, priv.ID, carol.Subject+"@example.com"), core.CodeOrganizationGuestSingleWorkspace)
	// Members and Owners see the members, with each one's workspaces.
	listed := e.ok(bob, fmt.Sprintf(`{ organizationMembers(organizationId: %q) { subject role workspaces } }`, orgID))["organizationMembers"].([]any)
	if len(listed) != 4 {
		t.Fatalf("org members = %v", listed)
	}

	// Two Billing people per org: bob in acme and a pending invite to the
	// Public workspace; a third is refused on GraphQL, REST and MCP alike.
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspaceMemberRole(workspaceId: %q, subject: %q, role: "BILLING") { role } }`, acme.ID, bob.Subject))
	e.ok(alice, fmt.Sprintf(`mutation { inviteWorkspaceMember(workspaceId: %q, email: "billing2-%s@example.com", role: "BILLING") { id } }`, pubID, e.stamp))
	e.refused(alice, fmt.Sprintf(`mutation { inviteWorkspaceMember(workspaceId: %q, email: "billing3-%s@example.com", role: "BILLING") { id } }`, priv.ID, e.stamp), core.CodeOrganizationBillingLimit)
	e.refused(alice, fmt.Sprintf(`mutation { changeWorkspaceMemberRole(workspaceId: %q, subject: %q, role: "BILLING") { role } }`, pubID, carol.Subject), core.CodeOrganizationBillingLimit)
	e.mcpRefused(alice, "change_workspace_member_role", map[string]any{"workspaceId": pubID, "subject": carol.Subject, "role": "BILLING"}, core.CodeOrganizationBillingLimit)
	ownIDs, err := e.st.OwnerIDsForSubjects(ctx, []string{carol.Subject})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/owners/"+pubID+"/members/"+ownIDs[carol.Subject], strings.NewReader(`{"role":"WORKSPACE_BILLING"}`)).
		WithContext(core.WithIdentity(ctx, alice))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.srv.restHandler().ServeHTTP(rec, req)
	var restErr map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &restErr)
	if rec.Code != http.StatusConflict || restErr["code"] != core.CodeOrganizationBillingLimit {
		t.Fatalf("REST PATCH to a third Billing member = %d %s", rec.Code, rec.Body)
	}

	// A secondary's plan is the primary's; Scale -> Enterprise on the
	// primary moves all three.
	e.refused(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "enterprise") { id } }`, pubID), core.CodeOrganizationPlanOnPrimary)
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "enterprise") { id } }`, acme.ID))
	for _, id := range []string{acme.ID, pubID, priv.ID} {
		if w, _ := e.st.GetTenant(ctx, id); w.Plan != store.PlanEnterprise {
			t.Fatalf("%s plan = %s after the primary moved to enterprise", id, w.Plan)
		}
	}

	// The downgrade is refused, naming each unmet precondition; the primary
	// cannot be deleted before its secondaries.
	out := e.gql(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "pro") { id } }`, acme.ID))
	if graphqlCode(out) != core.CodeOrganizationDowngradeBlocked {
		t.Fatalf("downgrade with preconditions = %v", out)
	}
	kinds := preconditionKinds(out)
	if !slices.Equal(kinds, []string{"secondary_workspaces", "member_roles"}) {
		t.Fatalf("downgrade preconditions = %v, want the secondaries and bob's Billing role", kinds)
	}
	e.refused(alice, fmt.Sprintf(`mutation { deleteWorkspace(id: %q, confirmation: %q) }`, acme.ID, workspaces.DeleteConfirmation(acme.Label())), core.CodeOrganizationPrimaryHasWorkspaces)

	// The org log is the Owner's: it lists the org's events, and no
	// workspace log carries them. REST and GraphQL list the same rows.
	status, raw := restRaw(t, e.srv, alice, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs?limit=100")
	if status != http.StatusOK {
		t.Fatalf("org audit log = %d %s", status, raw)
	}
	var rows []struct {
		AuditLog struct {
			ID, Event string
		} `json:"auditLog"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	events := map[string]int{}
	var restIDs []string
	for _, r := range rows {
		events[r.AuditLog.Event]++
		restIDs = append(restIDs, r.AuditLog.ID)
	}
	for _, want := range []string{"CreateWorkspaceEvent", "JoinTeamEvent", "InviteToOrgEvent", "AcceptOrgInviteEvent", "AddOrgMemberEvent", "ChangeOrgRoleEvent"} {
		if events[want] == 0 {
			t.Errorf("org log lacks %s: %v", want, events)
		}
	}
	gqlRows := e.ok(alice, fmt.Sprintf(`{ organizationAuditLogs(organizationId: %q, limit: 100) { id action } }`, orgID))["organizationAuditLogs"].([]any)
	var gqlIDs []string
	for _, r := range gqlRows {
		gqlIDs = append(gqlIDs, r.(map[string]any)["id"].(string))
	}
	if !slices.Equal(restIDs, gqlIDs) {
		t.Fatalf("org log rows differ: REST %v, GraphQL %v", restIDs, gqlIDs)
	}
	for _, who := range []core.Identity{bob, carol} {
		if status, _ := restRaw(t, e.srv, who, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs"); status != http.StatusForbidden {
			t.Errorf("org audit log as %s = %d, want 403", who.Subject, status)
		}
	}
	status, raw = restRaw(t, e.srv, alice, http.MethodGet, "/v1/owners/"+acme.ID+"/audit-logs?limit=1000")
	if status != http.StatusOK {
		t.Fatalf("workspace audit log = %d %s", status, raw)
	}
	for _, orgEvent := range []string{"JoinTeamEvent", "InviteToOrgEvent", "AcceptOrgInviteEvent", "AddOrgMemberEvent", "ChangeOrgRoleEvent", "ChangeWorkspacePrivacyEvent"} {
		if strings.Contains(string(raw), orgEvent) {
			t.Errorf("the workspace log carries the org event %s", orgEvent)
		}
	}

	// Clear the preconditions: delete the secondaries, give bob back a role
	// Pro offers. The downgrade then deletes the org and leaves acme
	// standalone with its members.
	for _, w := range []string{pubID, priv.ID} {
		ws, _ := e.st.GetTenant(ctx, w)
		e.ok(alice, fmt.Sprintf(`mutation { deleteWorkspace(id: %q, confirmation: %q) }`, w, workspaces.DeleteConfirmation(ws.Label())))
	}
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspaceMemberRole(workspaceId: %q, subject: %q, role: "DEVELOPER") { role } }`, acme.ID, bob.Subject))
	done := e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "pro") { plan organization { id } primary privacy } }`, acme.ID))["changeWorkspacePlan"].(map[string]any)
	if done["plan"] != "pro" || done["organization"] != nil || done["primary"] != false {
		t.Fatalf("after the downgrade = %v", done)
	}
	if orgs := e.ok(alice, `{ organizations { id } }`)["organizations"].([]any); len(orgs) != 0 {
		t.Fatalf("organizations after the downgrade = %v", orgs)
	}
	if ok, _ := e.fga.(core.FreshChecker).CheckFresh(ctx, "user:"+alice.Subject, core.RelCanManage, core.OrganizationObject(orgID)); ok {
		t.Fatal("the dissolved org's Owner tuple survived")
	}
	if ms, _ := e.st.ListTenantMembers(ctx, acme.ID); len(ms) != 2 {
		t.Fatalf("acme's members after the downgrade = %+v", ms)
	}
}

// orgInviteMailer keeps the org invite links it is asked to send.
type orgInviteMailer struct{ links map[string]string }

func (m *orgInviteMailer) Send(_ context.Context, to, _, text, _ string) error {
	if _, after, ok := strings.Cut(text, "/invite#org-invite="); ok {
		m.links[to], _, _ = strings.Cut(after, "\n")
	}
	return nil
}

// TestOrganizationInviteLinkE2E (w7/083): an org invite's emailed link joins
// the org for a signed-in person whose email is not the invited one, on
// GraphQL and MCP alike, and a link used once is refused by name for anyone
// else. The redemption is the org's AcceptOrgInviteEvent.
func TestOrganizationInviteLinkE2E(t *testing.T) {
	mailer := &orgInviteMailer{links: map[string]string{}}
	e := newOrgE2E(t, func(_ *core.Base, d *Deps) {
		d.Mailer = mailer
		d.InviteBaseURL = "https://dash.example"
	})
	ctx := context.Background()
	alice, erin, frank := e.person("alice"), e.person("erin"), e.person("frank")
	acme, err := e.st.CreateWorkspace(ctx, "acme-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	upgraded := e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { organization { id name } } }`, acme.ID))
	org := upgraded["changeWorkspacePlan"].(map[string]any)["organization"].(map[string]any)
	orgID, orgName := org["id"].(string), org["name"].(string)

	// alice invites an address erin does not sign in with; the mail's link
	// carries the token.
	invited := "erin-elsewhere-" + e.stamp + "@example.com"
	e.ok(alice, fmt.Sprintf(`mutation { inviteOrganizationMember(organizationId: %q, email: %q, role: "MEMBER") { id } }`, orgID, invited))
	token := mailer.links[invited]
	if token == "" {
		t.Fatalf("the org invite mail carries no token: %v", mailer.links)
	}

	// erin reviews it, then joins through it.
	preview := e.ok(erin, fmt.Sprintf(`{ organizationInvitePreview(token: %q) { organizationId organizationName role inviterEmail alreadyMember } }`, token))["organizationInvitePreview"].(map[string]any)
	if preview["organizationId"] != orgID || preview["organizationName"] != orgName || preview["role"] != "MEMBER" ||
		preview["inviterEmail"] != alice.Subject+"@example.com" || preview["alreadyMember"] != false {
		t.Fatalf("erin's preview = %v", preview)
	}
	accepted := e.ok(erin, fmt.Sprintf(`mutation { acceptOrganizationInvite(token: %q) { organizationId organizationName role joined } }`, token))["acceptOrganizationInvite"].(map[string]any)
	if accepted["organizationId"] != orgID || accepted["organizationName"] != orgName || accepted["role"] != "MEMBER" || accepted["joined"] != true {
		t.Fatalf("erin's acceptance = %v", accepted)
	}
	if role := e.ok(erin, fmt.Sprintf(`{ organization(id: %q) { viewerRole } }`, orgID))["organization"].(map[string]any)["viewerRole"]; role != "MEMBER" {
		t.Fatalf("erin's org role after the link = %v", role)
	}
	// She may reopen her own link; nobody else may use it, on either surface.
	again := e.ok(erin, fmt.Sprintf(`{ organizationInvitePreview(token: %q) { role alreadyMember } }`, token))["organizationInvitePreview"].(map[string]any)
	if again["alreadyMember"] != true || again["role"] != "MEMBER" {
		t.Fatalf("erin's preview after joining = %v", again)
	}
	e.refused(frank, fmt.Sprintf(`{ organizationInvitePreview(token: %q) { role } }`, token), members.InviteErrorAlreadyAccepted)
	e.refused(frank, fmt.Sprintf(`mutation { acceptOrganizationInvite(token: %q) { role } }`, token), members.InviteErrorAlreadyAccepted)
	e.mcpRefused(frank, "accept_organization_invite", map[string]any{"token": token}, members.InviteErrorAlreadyAccepted)
	e.refused(frank, fmt.Sprintf(`mutation { acceptOrganizationInvite(token: %q) { role } }`, strings.Repeat("0", 32)), members.InviteErrorInvalid)
	if _, err := e.st.GetOrganizationMember(ctx, orgID, frank.Subject); err == nil {
		t.Fatal("a used link added a second person to the org")
	}

	// frank joins as an Owner through his own link on MCP.
	invitedFrank := "frank-elsewhere-" + e.stamp + "@example.com"
	e.ok(alice, fmt.Sprintf(`mutation { inviteOrganizationMember(organizationId: %q, email: %q, role: "OWNER") { id } }`, orgID, invitedFrank))
	res := mcpCall(t, e.srv, frank, "accept_organization_invite", map[string]any{"token": mailer.links[invitedFrank]})
	if res.IsError {
		t.Fatalf("MCP accept_organization_invite: %s", mcpText(res))
	}
	var viaMCP members.AcceptedOrganizationInviteView
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &viaMCP); err != nil || viaMCP.OrganizationID != orgID || viaMCP.Role != "OWNER" || !viaMCP.Joined {
		t.Fatalf("MCP acceptance = %+v %v", viaMCP, err)
	}
	if role := e.ok(frank, fmt.Sprintf(`{ organization(id: %q) { viewerRole } }`, orgID))["organization"].(map[string]any)["viewerRole"]; role != "OWNER" {
		t.Fatalf("frank's org role after the link = %v", role)
	}

	// Each redemption is one AcceptOrgInviteEvent in the org log, by its redeemer.
	status, body := restRaw(t, e.srv, alice, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs?limit=100")
	if status != http.StatusOK {
		t.Fatalf("org audit log = %d %s", status, body)
	}
	var rows []struct {
		AuditLog struct {
			Event string `json:"event"`
		} `json:"auditLog"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	accepts := 0
	for _, r := range rows {
		if r.AuditLog.Event == "AcceptOrgInviteEvent" {
			accepts++
		}
	}
	if accepts != 2 {
		t.Fatalf("org log has %d AcceptOrgInviteEvent rows, want 2: %s", accepts, body)
	}
}

// preconditionKinds reads the kinds of an ORGANIZATION_DOWNGRADE_BLOCKED
// refusal's preconditions.
func preconditionKinds(out map[string]any) []string {
	errs, _ := out["errors"].([]any)
	if len(errs) == 0 {
		return nil
	}
	// GraphQL carries a coded error's params beside its code in extensions.
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	pre, _ := ext["preconditions"].([]any)
	var kinds []string
	for _, p := range pre {
		kinds = append(kinds, p.(map[string]any)["kind"].(string))
	}
	return kinds
}

// TestOrganizationDowngradePreconditionsE2E clears Render's downgrade
// preconditions one at a time (w7/m464 t005, render.com/docs/organizations:
// "Reassign all team members and pending invites with the Billing, Viewer, or
// Contributor role", "Delete any inbound IP rules for services, environments,
// and workspaces"): each refusal names exactly what is still unmet, and the
// downgrade goes through once nothing is. The org log also starts at the org:
// a row from before it was created is not listed.
func TestOrganizationDowngradePreconditionsE2E(t *testing.T) {
	e := newOrgE2E(t)
	ctx := context.Background()
	alice, vic := e.person("alice"), e.person("vic")
	acme, err := e.st.CreateWorkspace(ctx, "pre-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { id } }`, acme.ID))
	org, err := e.st.ListOrganizationsForSubject(ctx, alice.Subject)
	if err != nil || len(org) != 1 {
		t.Fatalf("orgs = %+v %v", org, err)
	}
	orgID := org[0].ID

	// A row from before the org existed is not in its log.
	early := "aud-early" + e.stamp
	if _, err := e.st.Pool.Exec(ctx, `INSERT INTO audit_events (id, workspace_id, verb, resource, outcome, at) VALUES ($1, $2, $3, $4, 'allowed', $5)`,
		early, orgID, core.AuditVerbOrgRenamed, core.OrganizationObject(orgID), org[0].AuditVisibleFrom.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, raw := restRaw(t, e.srv, alice, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs?limit=1000"); strings.Contains(string(raw), early) {
		t.Fatalf("the org log lists a row from before the org: %s", raw)
	}

	// A Viewer, a pending Contributor invite, a workspace rule and an
	// environment rule.
	if _, err := e.st.Pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'viewer')`, acme.ID, vic.Subject); err != nil {
		t.Fatal(err)
	}
	e.ok(alice, fmt.Sprintf(`mutation { inviteWorkspaceMember(workspaceId: %q, email: "contrib-%s@example.com", role: "CONTRIBUTOR") { id } }`, acme.ID, e.stamp))
	rule := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}
	if err := e.st.SetWorkspaceIPAllowList(ctx, acme.ID, rule); err != nil {
		t.Fatal(err)
	}
	project, err := e.st.CreateProject(ctx, acme.ID, "pre-"+e.stamp)
	if err != nil {
		t.Fatal(err)
	}
	env, err := e.st.CreateEnvironment(ctx, project.ID, acme.ID, "production")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetEnvironmentACL(ctx, env.ID, "unprotected", false, rule); err != nil {
		t.Fatal(err)
	}

	downgrade := fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "pro") { plan } }`, acme.ID)
	wantKinds := func(want ...string) {
		t.Helper()
		out := e.gql(alice, downgrade)
		if graphqlCode(out) != core.CodeOrganizationDowngradeBlocked {
			t.Fatalf("downgrade = %v, want %s", out, core.CodeOrganizationDowngradeBlocked)
		}
		if got := preconditionKinds(out); !slices.Equal(got, want) {
			t.Fatalf("downgrade preconditions = %v, want %v", got, want)
		}
	}
	wantKinds("member_roles", "invite_roles", "inbound_ip_rules")
	if _, err := e.st.Pool.Exec(ctx, `UPDATE tenant_members SET role = 'developer' WHERE tenant_id = $1 AND subject = $2`, acme.ID, vic.Subject); err != nil {
		t.Fatal(err)
	}
	wantKinds("invite_roles", "inbound_ip_rules")
	if _, err := e.st.Pool.Exec(ctx, `DELETE FROM tenant_invites WHERE tenant_id = $1`, acme.ID); err != nil {
		t.Fatal(err)
	}
	wantKinds("inbound_ip_rules")
	if err := e.st.SetWorkspaceIPAllowList(ctx, acme.ID, core.DefaultEnvironmentAllowList()); err != nil {
		t.Fatal(err)
	}
	wantKinds("inbound_ip_rules") // the environment's rule is still there
	if err := e.st.SetEnvironmentACL(ctx, env.ID, "unprotected", false, core.DefaultEnvironmentAllowList()); err != nil {
		t.Fatal(err)
	}
	if plan := e.ok(alice, downgrade)["changeWorkspacePlan"].(map[string]any)["plan"]; plan != "pro" {
		t.Fatalf("downgrade with nothing unmet = %v", plan)
	}
	if _, err := e.st.GetOrganization(ctx, orgID); err == nil {
		t.Fatal("the org survived its downgrade")
	}
}
