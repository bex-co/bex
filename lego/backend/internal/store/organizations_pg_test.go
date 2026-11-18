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

package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w7/m464 (docs/ADR115-organizations.md D9): migration 0293 makes every
// existing Scale and Enterprise workspace the primary of its own org, with its
// admins as Owners and its other people as Members, and leaves Hobby and Pro
// workspaces standalone. API keys' machine rows join no org. The down
// migration removes the schema.
func TestOrganizationBackfillMigrationPG(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	up, err := migrationsFS.ReadFile("migrations/0293_organizations.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0293_organizations.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("0293 down: %v", err)
	}
	seed := map[string]string{} // plan -> tenant id
	for _, plan := range []string{PlanHobby, PlanPro, PlanScale, PlanEnterprise} {
		id := ids.New(ids.Workspace)
		seed[plan] = id
		display := ""
		if plan == PlanScale {
			display = "Acme Scale"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, plan, display_name) VALUES ($1, $2, $3, $4)`, id, "bf-"+plan+"-"+stamp, plan, display); err != nil {
			t.Fatalf("seed %s: %v", plan, err)
		}
		for _, m := range []struct{ subject, role, kind string }{
			{"admin-" + plan + stamp, "admin", "user"},
			{"admin2-" + plan + stamp, "admin", "user"},
			{"dev-" + plan + stamp, "developer", "user"},
			{"key-" + plan + stamp, "developer", "machine"},
		} {
			if _, err := tx.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role, kind) VALUES ($1, $2, $3, $4)`, id, m.subject, m.role, m.kind); err != nil {
				t.Fatalf("seed member: %v", err)
			}
		}
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("0293 up: %v", err)
	}
	for plan, id := range seed {
		var orgID *string
		if err := tx.QueryRow(ctx, `SELECT organization_id FROM tenants WHERE id = $1`, id).Scan(&orgID); err != nil {
			t.Fatal(err)
		}
		inOrg := plan == PlanScale || plan == PlanEnterprise
		if (orgID != nil) != inOrg {
			t.Fatalf("%s workspace organization_id = %v, want an org: %v", plan, orgID, inOrg)
		}
		if !inOrg {
			continue
		}
		if !strings.HasPrefix(*orgID, "org-") || !ids.WellFormed(*orgID) {
			t.Errorf("%s org id %q is not an org- id", plan, *orgID)
		}
		var name, primary string
		if err := tx.QueryRow(ctx, `SELECT name, primary_workspace_id FROM organizations WHERE id = $1`, *orgID).Scan(&name, &primary); err != nil {
			t.Fatal(err)
		}
		wantName := "bf-" + plan + "-" + stamp
		if plan == PlanScale {
			wantName = "Acme Scale"
		}
		if primary != id || name != wantName {
			t.Errorf("%s org = (%q, primary %s), want (%q, %s)", plan, name, primary, wantName, id)
		}
		rows, err := tx.Query(ctx, `SELECT subject, role FROM organization_members WHERE organization_id = $1 ORDER BY subject`, *orgID)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for rows.Next() {
			var subject, role string
			if err := rows.Scan(&subject, &role); err != nil {
				t.Fatal(err)
			}
			got[subject] = role
		}
		rows.Close()
		want := map[string]string{
			"admin-" + plan + stamp: OrgRoleOwner, "admin2-" + plan + stamp: OrgRoleOwner, "dev-" + plan + stamp: OrgRoleMember,
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s org members = %v, want %v (an API key joins no org)", plan, got, want)
		}
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("0293 down after up: %v", err)
	}
	var orgs, privacy *string
	if err := tx.QueryRow(ctx, `SELECT to_regclass('organizations')::text, (SELECT column_name::text FROM information_schema.columns WHERE table_name = 'tenants' AND column_name = 'privacy')`).Scan(&orgs, &privacy); err != nil {
		t.Fatal(err)
	}
	if orgs != nil || privacy != nil {
		t.Fatalf("down migration left organizations=%v tenants.privacy=%v", orgs, privacy)
	}
	_ = st
}

// orgFixture is a Scale workspace upgraded into an org by its creator.
type orgFixture struct {
	st      *PGStore
	owner   string
	member  string
	primary Tenant
	org     Organization
}

func newOrgFixture(t *testing.T, ctx context.Context, st *PGStore, label string) orgFixture {
	t.Helper()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	f := orgFixture{st: st, owner: label + "-owner-" + stamp, member: label + "-member-" + stamp}
	ws, err := st.CreateWorkspace(ctx, label+"-"+stamp, PlanPro, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	if _, err := st.Pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'developer')`, ws.ID, f.member); err != nil {
		t.Fatal(err)
	}
	primary, org, members, err := st.UpgradeToOrganization(ctx, ws.ID, PlanScale, []string{f.owner})
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if primary.Plan != PlanScale || primary.OrganizationID != org.ID || primary.Privacy != PrivacyInviteOnly {
		t.Fatalf("upgraded primary = %+v", primary)
	}
	if org.PrimaryWorkspaceID != ws.ID || org.Plan != PlanScale || org.Name != ws.Label() {
		t.Fatalf("org = %+v", org)
	}
	roles := map[string]string{}
	for _, m := range members {
		roles[m.Subject] = m.Role
	}
	if roles[f.owner] != OrgRoleOwner || roles[f.member] != OrgRoleMember || len(roles) != 2 {
		t.Fatalf("org members = %v, want the upgrader as Owner and the rest as Members", roles)
	}
	f.primary, f.org = primary, org
	return f
}

// w7/m464 t002: the org store verbs, each under its lock.
func TestOrganizationStorePG(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	f := newOrgFixture(t, ctx, st, "orgstore")

	// An upgrade of a workspace already in an org is refused.
	if _, _, _, err := st.UpgradeToOrganization(ctx, f.primary.ID, PlanEnterprise, nil); !errors.Is(err, ErrInOrganization) {
		t.Fatalf("second upgrade = %v, want ErrInOrganization", err)
	}

	// A secondary takes the org's plan and the requested privacy; its creator
	// is its admin.
	second, err := st.CreateOrganizationWorkspace(ctx, f.org.ID, "orgstore-second-"+f.owner[len(f.owner)-6:], PrivacyPublic, f.owner)
	if err != nil {
		t.Fatalf("create in org: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), second.ID) })
	if second.Plan != PlanScale || second.OrganizationID != f.org.ID || second.Privacy != PrivacyPublic {
		t.Fatalf("secondary = %+v", second)
	}
	if m, err := st.GetTenantMember(ctx, second.ID, f.owner); err != nil || m.Role != "admin" {
		t.Fatalf("creator membership = %+v %v", m, err)
	}
	workspaces, err := st.ListOrganizationWorkspaces(ctx, f.org.ID)
	if err != nil || len(workspaces) != 2 || workspaces[0].ID != f.primary.ID {
		t.Fatalf("org workspaces = %+v %v, want the primary first", workspaces, err)
	}

	// Scale -> Enterprise moves every workspace.
	moved, err := st.ChangeOrganizationPlan(ctx, f.org.ID, PlanEnterprise)
	if err != nil || len(moved) != 2 {
		t.Fatalf("change org plan = %+v %v", moved, err)
	}
	for _, w := range moved {
		if got, _ := st.GetTenant(ctx, w.ID); got.Plan != PlanEnterprise {
			t.Fatalf("%s plan = %s after the org moved to enterprise", w.ID, got.Plan)
		}
	}

	// Privacy: a change reports changed once; a standalone workspace has none.
	if _, changed, err := st.SetWorkspacePrivacy(ctx, second.ID, PrivacyInviteOnly); err != nil || !changed {
		t.Fatalf("privacy change = %v %v", changed, err)
	}
	if _, changed, err := st.SetWorkspacePrivacy(ctx, second.ID, PrivacyInviteOnly); err != nil || changed {
		t.Fatalf("same privacy again = %v %v, want unchanged", changed, err)
	}
	standalone, err := st.CreateWorkspace(ctx, "orgstore-solo-"+f.owner[len(f.owner)-6:], PlanPro, f.member)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), standalone.ID) })
	if _, _, err := st.SetWorkspacePrivacy(ctx, standalone.ID, PrivacyPublic); !errors.Is(err, ErrInvalid) {
		t.Fatalf("privacy on a standalone workspace = %v, want ErrInvalid", err)
	}

	// The last Owner can be neither demoted nor removed.
	if _, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.owner, OrgRoleMember); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("demote last owner = %v, want ErrLastOwner", err)
	}
	if err := st.RemoveOrganizationMember(ctx, f.org.ID, f.owner); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("remove last owner = %v, want ErrLastOwner", err)
	}
	if old, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.member, OrgRoleOwner); err != nil || old != OrgRoleMember {
		t.Fatalf("promote member = %q %v", old, err)
	}
	if _, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.owner, OrgRoleMember); err != nil {
		t.Fatalf("demote one of two owners: %v", err)
	}
	if _, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.owner, OrgRoleOwner); err != nil {
		t.Fatal(err)
	}

	// A Guest belongs to at most one org workspace: the owner is in two.
	if _, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.owner, OrgRoleGuest); !errors.Is(err, ErrGuestSingleWorkspace) {
		t.Fatalf("guest in two workspaces = %v, want ErrGuestSingleWorkspace", err)
	}
	// The member is in only the primary, so they can be a Guest; a workspace
	// invite to a second org workspace then stays pending at sign-in and is
	// refused by its token.
	if _, err := st.SetOrganizationMemberRole(ctx, f.org.ID, f.member, OrgRoleGuest); err != nil {
		t.Fatalf("make guest: %v", err)
	}
	if got, _ := st.SubjectOrganizationWorkspaces(ctx, f.org.ID, f.member); !slices.Equal(got, []string{f.primary.ID}) {
		t.Fatalf("guest workspaces = %v", got)
	}
	email := "guest-" + f.member + "@example.test"
	inv, err := st.CreateInvite(ctx, second.ID, email, "developer", "tok-"+f.member, f.owner, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := st.AcceptInvitesForEmail(ctx, email, f.member); err != nil || len(accepted) != 0 {
		t.Fatalf("login redemption of a guest's second workspace = %+v %v, want it left pending", accepted, err)
	}
	if _, err := st.AcceptInviteByToken(ctx, inv.Token, f.member); !errors.Is(err, ErrGuestSingleWorkspace) {
		t.Fatalf("token redemption of a guest's second workspace = %v, want ErrGuestSingleWorkspace", err)
	}

	// A workspace invite to someone outside the org makes them a Member.
	newcomer := "orgstore-newcomer-" + f.owner
	inv2, err := st.CreateInvite(ctx, second.ID, newcomer+"@example.test", "developer", "tok2-"+f.member, f.owner, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.AcceptInviteByToken(ctx, inv2.Token, newcomer)
	if err != nil || got.JoinedOrganizationID != f.org.ID {
		t.Fatalf("newcomer redemption = %+v %v, want JoinedOrganizationID %s", got, err, f.org.ID)
	}
	if m, err := st.GetOrganizationMember(ctx, f.org.ID, newcomer); err != nil || m.Role != OrgRoleMember {
		t.Fatalf("newcomer org role = %+v %v", m, err)
	}

	// Two Billing people at most: holders and pending invites both count.
	if _, err := st.Pool.Exec(ctx, `UPDATE tenant_members SET role = 'billing' WHERE tenant_id = $1 AND subject = $2`, second.ID, newcomer); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateInvite(ctx, f.primary.ID, "billing2@example.test", "billing", "tok3-"+f.member, f.owner, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	subjects, emails, err := st.OrganizationBillingHolders(ctx, f.org.ID)
	if err != nil || !slices.Equal(subjects, []string{newcomer}) || !slices.Equal(emails, []string{"billing2@example.test"}) {
		t.Fatalf("billing holders = %v %v %v", subjects, emails, err)
	}

	// Org invites: one pending per email, redeemed at sign-in; someone
	// already in the org keeps their role.
	if _, err := st.CreateOrganizationInvite(ctx, f.org.ID, "owner2@example.test", OrgRoleOwner, "otok-"+f.member, f.owner, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateOrganizationInvite(ctx, f.org.ID, "owner2@example.test", OrgRoleMember, "otok2-"+f.member, f.owner, time.Now().Add(time.Hour)); !errors.Is(err, ErrConflict) {
		t.Fatalf("second pending org invite = %v, want ErrConflict", err)
	}
	owner2 := "orgstore-owner2-" + f.owner
	joined, err := st.AcceptOrganizationInvitesForEmail(ctx, "owner2@example.test", owner2)
	if err != nil || len(joined) != 1 || joined[0].Role != OrgRoleOwner {
		t.Fatalf("sign-in redemption = %+v %v", joined, err)
	}
	if again, err := st.AcceptOrganizationInvitesForEmail(ctx, "owner2@example.test", owner2); err != nil || len(again) != 0 {
		t.Fatalf("second sign-in = %+v %v, want nothing left to redeem", again, err)
	}
	if _, err := st.CreateOrganizationInvite(ctx, f.org.ID, "member-again@example.test", OrgRoleOwner, "otok3-"+f.member, f.owner, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if joined, err := st.AcceptOrganizationInvitesForEmail(ctx, "member-again@example.test", f.member); err != nil || len(joined) != 0 {
		t.Fatalf("an invite to someone already in the org = %+v %v, want their role kept", joined, err)
	}
	if m, _ := st.GetOrganizationMember(ctx, f.org.ID, f.member); m.Role != OrgRoleGuest {
		t.Fatalf("an invite changed an existing org role to %q", m.Role)
	}

	// Dissolving refuses while a secondary exists; then leaves the primary
	// standalone on the new plan with its own members unchanged.
	if _, err := st.DissolveOrganization(ctx, f.org.ID, PlanPro); !errors.Is(err, ErrOrganizationHasWorkspaces) {
		t.Fatalf("dissolve with a secondary = %v", err)
	}
	if err := st.DeleteTenant(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ListTenantMembers(ctx, f.primary.ID)
	primary, err := st.DissolveOrganization(ctx, f.org.ID, PlanPro)
	if err != nil {
		t.Fatalf("dissolve: %v", err)
	}
	if primary.Plan != PlanPro || primary.OrganizationID != "" {
		t.Fatalf("dissolved = %+v", primary)
	}
	if _, err := st.GetOrganization(ctx, f.org.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org after dissolve = %v, want ErrNotFound", err)
	}
	after, _ := st.ListTenantMembers(ctx, f.primary.ID)
	if len(before) != len(after) {
		t.Fatalf("primary members %d -> %d across the dissolve", len(before), len(after))
	}

	// Support's transfer: the standalone workspace joins another org at its
	// plan, its people become Members, and a workspace in an org is refused.
	g := newOrgFixture(t, ctx, st, "orgmove")
	moved2, added, err := st.AttachWorkspaceToOrganization(ctx, standalone.ID, g.org.ID)
	if err != nil || moved2.Plan != PlanScale || moved2.OrganizationID != g.org.ID || !slices.Equal(added, []string{f.member}) {
		t.Fatalf("attach = %+v %v %v", moved2, added, err)
	}
	if _, _, err := st.AttachWorkspaceToOrganization(ctx, standalone.ID, g.org.ID); !errors.Is(err, ErrInOrganization) {
		t.Fatalf("attach twice = %v, want ErrInOrganization", err)
	}

	// Deleting a primary deletes its org.
	if err := st.DeleteTenant(ctx, standalone.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTenant(ctx, g.primary.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOrganization(ctx, g.org.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org after its primary was deleted = %v", err)
	}
}

// w7/083: an org invite redeems by its emailed token for whoever holds it,
// once, before it lapses; someone already in the org keeps their role.
func TestOrganizationInviteByTokenPG(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	f := newOrgFixture(t, ctx, st, "orgtoken")
	invite := func(email, role, token string, expiresAt time.Time) OrganizationInvite {
		t.Helper()
		inv, err := st.CreateOrganizationInvite(ctx, f.org.ID, email, role, token, f.owner, expiresAt)
		if err != nil {
			t.Fatal(err)
		}
		return inv
	}

	// Someone signed in under another address redeems the invite by its token.
	inv := invite("invited@example.test", OrgRoleOwner, "otok-a-"+f.owner, time.Now().Add(time.Hour))
	if got, err := st.GetOrganizationInviteByToken(ctx, inv.Token); err != nil || got.ID != inv.ID || got.AcceptedAt != nil {
		t.Fatalf("read by token = %+v %v", got, err)
	}
	other := "orgtoken-other-" + f.owner
	got, joined, err := st.AcceptOrganizationInviteByToken(ctx, inv.Token, other)
	if err != nil || !joined || got.ID != inv.ID || got.Role != OrgRoleOwner || got.OrganizationID != f.org.ID {
		t.Fatalf("redemption = %+v joined=%v %v", got, joined, err)
	}
	if m, err := st.GetOrganizationMember(ctx, f.org.ID, other); err != nil || m.Role != OrgRoleOwner {
		t.Fatalf("redeemer's org role = %+v %v", m, err)
	}
	// The link works once, for anyone, and stays readable for the preview.
	for _, subject := range []string{other, "orgtoken-third-" + f.owner} {
		if _, _, err := st.AcceptOrganizationInviteByToken(ctx, inv.Token, subject); !errors.Is(err, ErrInviteAlreadyAccepted) {
			t.Fatalf("reused link as %s = %v, want ErrInviteAlreadyAccepted", subject, err)
		}
	}
	if got, err := st.GetOrganizationInviteByToken(ctx, inv.Token); err != nil || got.AcceptedAt == nil {
		t.Fatalf("used invite by token = %+v %v", got, err)
	}
	// It no longer redeems at the invitee's sign-in either.
	if joined, err := st.AcceptOrganizationInvitesForEmail(ctx, "invited@example.test", "orgtoken-invitee-"+f.owner); err != nil || len(joined) != 0 {
		t.Fatalf("sign-in after the link was used = %+v %v", joined, err)
	}

	// A lapsed invite and an unknown token are refused by name.
	lapsed := invite("lapsed@example.test", OrgRoleMember, "otok-b-"+f.owner, time.Now().Add(-time.Minute))
	if _, _, err := st.AcceptOrganizationInviteByToken(ctx, lapsed.Token, "orgtoken-late-"+f.owner); !errors.Is(err, ErrInviteExpired) {
		t.Fatalf("lapsed invite = %v, want ErrInviteExpired", err)
	}
	if _, err := st.GetOrganizationMember(ctx, f.org.ID, "orgtoken-late-"+f.owner); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a lapsed invite added its redeemer: %v", err)
	}
	if _, _, err := st.AcceptOrganizationInviteByToken(ctx, "no-such-token", other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token = %v, want ErrNotFound", err)
	}
	if _, err := st.GetOrganizationInviteByToken(ctx, "no-such-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token read = %v, want ErrNotFound", err)
	}

	// A Member redeeming an Owner invite stays a Member, and the invite is used.
	again := invite("again@example.test", OrgRoleOwner, "otok-c-"+f.owner, time.Now().Add(time.Hour))
	got, joined, err = st.AcceptOrganizationInviteByToken(ctx, again.Token, f.member)
	if err != nil || joined || got.Role != OrgRoleMember {
		t.Fatalf("an existing member's redemption = %+v joined=%v %v, want their Member role kept", got, joined, err)
	}
	if m, _ := st.GetOrganizationMember(ctx, f.org.ID, f.member); m.Role != OrgRoleMember {
		t.Fatalf("redemption changed an existing org role to %q", m.Role)
	}
	if pending, err := st.ListOrganizationInvites(ctx, f.org.ID); err != nil || len(pending) != 0 {
		t.Fatalf("pending invites = %+v %v, want none", pending, err)
	}
}
