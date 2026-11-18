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

package workspaces

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// organization_login_pg_test.go covers the org setting of w7/m466
// (docs/ADR012-auth.md §9c): an Owner turns "require two-factor
// authentication" and "require Google login" on and off for the whole org,
// with the workspace settings' plan, no-op and lock-out rules; each change is
// one org audit row (Render's ChangeOrg2FAEnforcementEvent and
// ChangeOrgAllowedLoginMethodsEvent); and the real gate then holds every org
// workspace and the org's own objects to it. Real Postgres, the real
// twofactor.Gate.

// orgLoginFixture is a Scale org over real Postgres: the primary with an
// enrolled Owner (signed in with Google), an unenrolled Owner, and a Member;
// a secondary whose admin is a Member of the org; a Guest in the secondary;
// and a standalone Pro workspace the Member also belongs to.
type orgLoginFixture struct {
	pg                    *store.PGStore
	svc                   *Service
	gate                  *twofactor.Gate
	audit                 *auditCapture
	changed               []string
	org                   store.Organization
	primary, secondary    string
	standalone            string
	owner, owner2, member string
	wsAdmin, guest        string
}

func newOrgLoginFixture(t *testing.T) *orgLoginFixture {
	t.Helper()
	pg := settlePG(t)
	ctx := context.Background()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	f := &orgLoginFixture{
		pg: pg, audit: &auditCapture{},
		owner: "ol-owner-" + stamp, owner2: "ol-owner2-" + stamp, member: "ol-member-" + stamp,
		wsAdmin: "ol-wsadmin-" + stamp, guest: "ol-guest-" + stamp,
	}
	primary, err := pg.CreateWorkspace(ctx, "ol-primary-"+stamp, store.PlanPro, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	f.primary = primary.ID
	for subject, role := range map[string]string{f.owner2: "admin", f.member: "developer", f.wsAdmin: "developer"} {
		if err := pg.AddMember(ctx, subject, f.primary, role); err != nil {
			t.Fatal(err)
		}
	}
	_, org, _, err := pg.UpgradeToOrganization(ctx, f.primary, store.PlanScale, []string{f.owner, f.owner2})
	if err != nil {
		t.Fatal(err)
	}
	f.org = org
	secondary, err := pg.CreateOrganizationWorkspace(ctx, org.ID, "ol-secondary-"+stamp, store.PrivacyInviteOnly, f.wsAdmin)
	if err != nil {
		t.Fatal(err)
	}
	f.secondary = secondary.ID
	if err := pg.AddMember(ctx, f.member, f.secondary, "developer"); err != nil {
		t.Fatal(err)
	}
	// A Guest, as an accepted guest invite leaves them: in the org, and in the
	// one workspace.
	if _, err := pg.Pool.Exec(ctx, `INSERT INTO organization_members (organization_id, subject, role) VALUES ($1, $2, 'guest')`, org.ID, f.guest); err != nil {
		t.Fatal(err)
	}
	if err := pg.AddMember(ctx, f.guest, f.secondary, "developer"); err != nil {
		t.Fatal(err)
	}
	standalone, err := pg.CreateWorkspace(ctx, "ol-alone-"+stamp, store.PlanPro, f.member)
	if err != nil {
		t.Fatal(err)
	}
	f.standalone = standalone.ID
	t.Cleanup(func() {
		for _, id := range []string{f.secondary, f.standalone, f.primary} {
			_ = pg.DeleteTenant(context.Background(), id)
		}
	})

	identities := fakeIdentities{f.owner: {MFAEnabled: true}, f.owner2: {}, f.member: {}, f.wsAdmin: {}, f.guest: {}}
	f.gate = twofactor.New(twofactor.Config{
		Settings: pg, Enrollment: identities,
		Google: googleAccounts{f.owner: true, f.owner2: false, f.member: false, f.wsAdmin: false, f.guest: false},
	})
	f.svc = &Service{
		Base:             &core.Base{Audit: f.audit, TwoFactor: f.gate},
		Store:            pg,
		Identities:       identities,
		GoogleLogin:      f.gate,
		TwoFactorChanged: func(id string) { f.changed = append(f.changed, id); f.gate.Invalidate(id) },
	}
	return f
}

// orgRows are the allowed org audit rows of verb.
func (a *auditCapture) orgRows(verb string) []core.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []core.AuditEvent
	for _, e := range a.events {
		if e.Verb == verb && e.Outcome == core.AuditAllowed {
			out = append(out, e)
		}
	}
	return out
}

func TestSetOrganizationRequireTwoFactor_AnOwnerTogglesItAndEachChangeIsOneOrgRow(t *testing.T) {
	f := newOrgLoginFixture(t)
	ctx := asGoogle(f.owner)

	o, err := f.svc.SetOrganizationRequireTwoFactor(ctx, f.org.ID, true)
	if err != nil || !o.RequireTwoFactor || o.RequireGoogleLogin || o.ID != f.org.ID {
		t.Fatalf("enable = %+v, %v", o, err)
	}
	rows := f.audit.orgRows(core.AuditVerbOrgSetRequireTwoFactor)
	if len(rows) != 1 || rows[0].Resource != core.OrganizationObject(f.org.ID) || rows[0].TwoFactorRequiredTo == nil || !*rows[0].TwoFactorRequiredTo {
		t.Fatalf("org rows = %+v, want one on org:%s with to=true", rows, f.org.ID)
	}
	// This replica drops the org's cached setting and each workspace's.
	want := []string{core.OrganizationObject(f.org.ID), f.primary, f.secondary}
	if !slices.Equal(f.changed, want) {
		t.Fatalf("invalidated %v, want %v", f.changed, want)
	}

	// A no-op writes nothing, records nothing and invalidates nothing.
	f.changed = nil
	if o, err := f.svc.SetOrganizationRequireTwoFactor(ctx, f.org.ID, true); err != nil || !o.RequireTwoFactor {
		t.Fatalf("enable again = %+v, %v", o, err)
	}
	if rows := f.audit.orgRows(core.AuditVerbOrgSetRequireTwoFactor); len(rows) != 1 || len(f.changed) != 0 {
		t.Fatalf("a no-op recorded %d rows and invalidated %v", len(rows), f.changed)
	}

	// Turning it off is one more row, to=false, and leaves each workspace's
	// own setting as it was.
	if _, _, err := f.pg.SetTenantRequireTwoFactor(context.Background(), f.secondary, true); err != nil {
		t.Fatal(err)
	}
	if o, err := f.svc.SetOrganizationRequireTwoFactor(ctx, f.org.ID, false); err != nil || o.RequireTwoFactor {
		t.Fatalf("disable = %+v, %v", o, err)
	}
	rows = f.audit.orgRows(core.AuditVerbOrgSetRequireTwoFactor)
	if len(rows) != 2 || rows[1].TwoFactorRequiredTo == nil || *rows[1].TwoFactorRequiredTo {
		t.Fatalf("org rows after disable = %+v", rows)
	}
	if ws, _ := f.pg.GetTenant(context.Background(), f.secondary); !ws.RequireTwoFactor {
		t.Fatalf("the secondary's own setting changed with the org's: %+v", ws)
	}
	// Neither row is a workspace row.
	for _, e := range f.audit.events {
		if e.Verb == core.AuditVerbSetRequireTwoFactor {
			t.Fatalf("an org change recorded a workspace row: %+v", e)
		}
	}
}

func TestSetOrganizationRequireGoogleLogin_AnOwnerTogglesItAndItIsAudited(t *testing.T) {
	f := newOrgLoginFixture(t)
	ctx := asGoogle(f.owner)
	o, err := f.svc.SetOrganizationRequireGoogleLogin(ctx, f.org.ID, true)
	if err != nil || !o.RequireGoogleLogin || o.RequireTwoFactor {
		t.Fatalf("enable = %+v, %v", o, err)
	}
	rows := f.audit.orgRows(core.AuditVerbOrgSetRequireGoogleLogin)
	if len(rows) != 1 || rows[0].GoogleLoginRequiredTo == nil || !*rows[0].GoogleLoginRequiredTo || rows[0].TwoFactorRequiredTo != nil {
		t.Fatalf("org rows = %+v, want one with google to=true", rows)
	}
	if _, err := f.svc.SetOrganizationRequireGoogleLogin(ctx, f.org.ID, true); err != nil {
		t.Fatal(err)
	}
	if rows := f.audit.orgRows(core.AuditVerbOrgSetRequireGoogleLogin); len(rows) != 1 {
		t.Fatalf("a no-op recorded a row: %+v", rows)
	}
	if o, err := f.svc.SetOrganizationRequireGoogleLogin(ctx, f.org.ID, false); err != nil || o.RequireGoogleLogin {
		t.Fatalf("disable = %+v, %v", o, err)
	}
}

func TestSetOrganizationLoginRequirement_OnlyAnOwnerMayChangeIt(t *testing.T) {
	f := newOrgLoginFixture(t)
	for _, who := range []string{f.member, f.wsAdmin, f.guest} {
		if _, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle(who), f.org.ID, true); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s turning org 2FA on = %v, want forbidden", who, err)
		}
		if _, err := f.svc.SetOrganizationRequireGoogleLogin(asGoogle(who), f.org.ID, true); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s turning org Google login on = %v, want forbidden", who, err)
		}
	}
	if _, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle("ol-stranger"), f.org.ID, true); !isCoded(err, core.ErrForbidden, core.CodeOrganizationNotFound) {
		t.Fatalf("a stranger = %v, want ORGANIZATION_NOT_FOUND", err)
	}
	if o, _ := f.pg.GetOrganization(context.Background(), f.org.ID); o.RequireTwoFactor || o.RequireGoogleLogin {
		t.Fatalf("a refused caller changed the org: %+v", o)
	}
	if rows := f.audit.orgRows(core.AuditVerbOrgSetRequireTwoFactor); len(rows) != 0 {
		t.Fatalf("a refused change recorded an allowed row: %+v", rows)
	}
	// Every person in the org reads both settings.
	for _, who := range []string{f.member, f.guest} {
		o, err := f.svc.GetOrganization(asGoogle(who), f.org.ID)
		if err != nil || o.RequireTwoFactor || o.RequireGoogleLogin {
			t.Fatalf("%s reading the org = %+v, %v", who, o, err)
		}
	}
}

func TestSetOrganizationLoginRequirement_AnOwnerWhoDoesNotMeetItCannotLockThemselvesOut(t *testing.T) {
	f := newOrgLoginFixture(t)
	_, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle(f.owner2), f.org.ID, true)
	var coded *core.CodedError
	if !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorAdminNotEnrolled) || !errors.As(err, &coded) || coded.Params["organizationId"] != f.org.ID {
		t.Fatalf("an unenrolled Owner turning org 2FA on = %v, want 409 %s naming the org", err, core.CodeWorkspaceTwoFactorAdminNotEnrolled)
	}
	if _, err := f.svc.SetOrganizationRequireGoogleLogin(asPassword(f.owner), f.org.ID, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginAdminNotSignedIn) {
		t.Fatalf("an Owner signed in with a password turning org Google login on = %v", err)
	}
	if _, err := f.svc.SetOrganizationRequireGoogleLogin(asGoogle(f.owner2), f.org.ID, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginAdminNotSignedIn) {
		t.Fatalf("an Owner whose Google account is not their email's = %v", err)
	}
	// Without a lookup the setting cannot be enforced, so it cannot be turned on.
	f.svc.Identities, f.svc.GoogleLogin = nil, nil
	if _, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle(f.owner), f.org.ID, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorUnavailable) {
		t.Fatalf("2FA with no lookup = %v", err)
	}
	if _, err := f.svc.SetOrganizationRequireGoogleLogin(asGoogle(f.owner), f.org.ID, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginUnavailable) {
		t.Fatalf("Google with no lookup = %v", err)
	}
	if len(f.audit.orgRows(core.AuditVerbOrgSetRequireTwoFactor))+len(f.audit.orgRows(core.AuditVerbOrgSetRequireGoogleLogin)) != 0 {
		t.Fatal("a refused change recorded a row")
	}
}

// The gate holds every org workspace and the org's own objects to the org's
// requirement, whatever each workspace's own setting is, and the refusal
// names the org; a standalone workspace is untouched; the exempt org read
// still answers, with the setting, so a blocked person sees why.
func TestOrganizationTwoFactorBindsEveryOrgWorkspaceAndTheOrg(t *testing.T) {
	f := newOrgLoginFixture(t)
	if _, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle(f.owner), f.org.ID, true); err != nil {
		t.Fatal(err)
	}
	session := func(subject string) core.Identity {
		return core.Identity{Subject: subject, Method: core.MethodSession, Human: true, SignIn: core.SignInGoogle}
	}
	ctx := context.Background()
	for _, ws := range []string{f.primary, f.secondary} {
		err := f.gate.Check(ctx, ws, session(f.member))
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired || coded.Params["organizationId"] != f.org.ID || coded.Params["workspaceId"] != ws {
			t.Fatalf("a Member without a second factor in %s = %v, want 403 naming the org", ws, err)
		}
		if err := f.gate.Check(ctx, ws, session(f.owner)); err != nil {
			t.Fatalf("an enrolled Owner in %s = %v", ws, err)
		}
	}
	if err := f.gate.Check(ctx, f.secondary, session(f.guest)); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a Guest in their one workspace = %v, want refused", err)
	}
	if err := f.gate.Check(ctx, f.standalone, session(f.member)); err != nil {
		t.Fatalf("the Member's standalone workspace = %v, want untouched", err)
	}
	// The org's own objects: an unenrolled Owner is refused on them...
	err := f.gate.CheckOrganization(ctx, f.org.ID, session(f.owner2))
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired || coded.Params["organizationId"] != f.org.ID || coded.Params["workspaceId"] != nil {
		t.Fatalf("an unenrolled Owner on the org = %v, want 403 naming the org and no workspace", err)
	}
	if _, err := f.svc.RenameOrganization(asGoogle(f.owner2), f.org.ID, "renamed"); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceTwoFactorRequired) {
		t.Fatalf("an unenrolled Owner renaming the org = %v, want the 2FA refusal", err)
	}
	// ...but still reads the org and its setting, and lists their orgs.
	o, err := f.svc.GetOrganization(asGoogle(f.owner2), f.org.ID)
	if err != nil || !o.RequireTwoFactor {
		t.Fatalf("the exempt org read = %+v, %v", o, err)
	}
	if list, err := f.svc.ListOrganizations(asGoogle(f.member)); err != nil || len(list) != 1 || !list[0].RequireTwoFactor {
		t.Fatalf("the exempt org list = %+v, %v", list, err)
	}

	// Turning it off lets the Member back in at once on this replica.
	if _, err := f.svc.SetOrganizationRequireTwoFactor(asGoogle(f.owner), f.org.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := f.gate.Check(ctx, f.secondary, session(f.member)); err != nil {
		t.Fatalf("after the org turned it off = %v, want allowed at once", err)
	}
	if err := f.gate.CheckOrganization(ctx, f.org.ID, session(f.owner2)); err != nil {
		t.Fatalf("the org after it turned it off = %v", err)
	}
}

func TestOrganizationGoogleLoginBindsEveryOrgWorkspaceAndTheOrg(t *testing.T) {
	f := newOrgLoginFixture(t)
	if _, err := f.svc.SetOrganizationRequireGoogleLogin(asGoogle(f.owner), f.org.ID, true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	password := core.Identity{Subject: f.owner, Method: core.MethodSession, Human: true, SignIn: core.SignInPassword}
	err := f.gate.Check(ctx, f.secondary, password)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceGoogleLoginRequired || coded.Params["organizationId"] != f.org.ID {
		t.Fatalf("a password session in an org workspace = %v, want 403 naming the org", err)
	}
	if err := f.gate.CheckOrganization(ctx, f.org.ID, password); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceGoogleLoginRequired) {
		t.Fatalf("a password session on the org = %v", err)
	}
	google := password
	google.SignIn = core.SignInGoogle
	if err := f.gate.Check(ctx, f.secondary, google); err != nil {
		t.Fatalf("a Google session = %v", err)
	}
	if err := f.gate.Check(ctx, f.standalone, password); err != nil {
		t.Fatalf("the standalone workspace = %v, want untouched", err)
	}
}
