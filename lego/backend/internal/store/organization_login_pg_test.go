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
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w7/m466 (migration 0295, docs/ADR012-auth.md §9c): an org's two login
// settings default to off, are written by conditional setters, and are ORed
// into every org workspace's LoginRequirements, naming the org; a standalone
// workspace reads exactly as before, and the settings guard names an org
// workspace when only the org requires Google login.
func TestPGOrganizationLoginRequirements(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	owner := "ol-owner-" + stamp
	primary, err := st.CreateTenantWithMember(ctx, owner, PlanScale)
	if err != nil {
		t.Fatalf("create primary: %v", err)
	}
	_, org, _, err := st.UpgradeToOrganization(ctx, primary.ID, PlanScale, []string{owner})
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if _, err := st.RenameOrganization(ctx, org.ID, "Acme "+stamp); err != nil {
		t.Fatal(err)
	}
	secondary, err := st.CreateOrganizationWorkspace(ctx, org.ID, "ol-second-"+stamp, PrivacyInviteOnly, owner)
	if err != nil {
		t.Fatalf("create secondary: %v", err)
	}
	standalone, err := st.CreateWorkspace(ctx, "ol-alone-"+stamp, PlanPro, owner)
	if err != nil {
		t.Fatalf("create standalone: %v", err)
	}
	t.Cleanup(func() {
		for _, id := range []string{secondary.ID, standalone.ID, primary.ID} {
			_ = st.DeleteTenant(context.Background(), id)
		}
	})
	orgName := "Acme " + stamp

	if org.RequireTwoFactor || org.RequireGoogleLogin {
		t.Fatalf("a new org requires something: %+v", org)
	}
	for _, id := range []string{primary.ID, secondary.ID, standalone.ID} {
		if r, err := st.LoginRequirements(ctx, id); err != nil || r != (core.LoginRequirements{}) {
			t.Fatalf("LoginRequirements(%s) before any = (%+v, %v), want nothing", id, r, err)
		}
	}
	if r, err := st.OrganizationLoginRequirements(ctx, org.ID); err != nil || r != (core.LoginRequirements{}) {
		t.Fatalf("OrganizationLoginRequirements before any = (%+v, %v)", r, err)
	}

	// Turning org 2FA on: one change, then a no-op.
	if changed, err := st.SetOrganizationRequireTwoFactor(ctx, org.ID, true); err != nil || !changed {
		t.Fatalf("enable 2FA = (changed=%v, %v)", changed, err)
	}
	if changed, err := st.SetOrganizationRequireTwoFactor(ctx, org.ID, true); err != nil || changed {
		t.Fatalf("enable 2FA again = (changed=%v, %v), want changed=false", changed, err)
	}
	byOrg := core.LoginOrganization{ID: org.ID, Name: orgName, TwoFactor: true}
	// Every org workspace whose own setting is off reads the org's, naming it;
	// the standalone workspace is untouched.
	for _, id := range []string{primary.ID, secondary.ID} {
		if r, err := st.LoginRequirements(ctx, id); err != nil || r != (core.LoginRequirements{TwoFactor: true, Organization: byOrg}) {
			t.Fatalf("LoginRequirements(%s) = (%+v, %v), want 2FA by %s", id, r, err, org.ID)
		}
	}
	if r, err := st.LoginRequirements(ctx, standalone.ID); err != nil || r != (core.LoginRequirements{}) {
		t.Fatalf("standalone LoginRequirements = (%+v, %v), want nothing", r, err)
	}
	if r, err := st.OrganizationLoginRequirements(ctx, org.ID); err != nil || r != (core.LoginRequirements{TwoFactor: true, Organization: byOrg}) {
		t.Fatalf("OrganizationLoginRequirements = (%+v, %v)", r, err)
	}
	// The org's reads carry it.
	if o, err := st.GetOrganization(ctx, org.ID); err != nil || !o.RequireTwoFactor || o.RequireGoogleLogin {
		t.Fatalf("GetOrganization = (%+v, %v)", o, err)
	}
	if ms, err := st.ListOrganizationsForSubject(ctx, owner); err != nil || len(ms) != 1 || !ms[0].RequireTwoFactor {
		t.Fatalf("ListOrganizationsForSubject = (%+v, %v)", ms, err)
	}

	// A workspace's own Google login ORs with the org's 2FA.
	if _, _, err := st.SetTenantRequireGoogleLogin(ctx, secondary.ID, true); err != nil {
		t.Fatal(err)
	}
	if r, err := st.LoginRequirements(ctx, secondary.ID); err != nil || r != (core.LoginRequirements{TwoFactor: true, GoogleLogin: true, Organization: byOrg}) {
		t.Fatalf("own Google + org 2FA = (%+v, %v)", r, err)
	}
	if _, _, err := st.SetTenantRequireGoogleLogin(ctx, secondary.ID, false); err != nil {
		t.Fatal(err)
	}

	// Org Google login: the settings guard names the org workspaces the member
	// belongs to, under the org's name, and not the standalone one.
	if names, err := st.GoogleLoginWorkspaces(ctx, owner); err != nil || len(names) != 0 {
		t.Fatalf("GoogleLoginWorkspaces with org 2FA only = (%v, %v), want none", names, err)
	}
	if changed, err := st.SetOrganizationRequireGoogleLogin(ctx, org.ID, true); err != nil || !changed {
		t.Fatalf("enable Google = (changed=%v, %v)", changed, err)
	}
	both := core.LoginOrganization{ID: org.ID, Name: orgName, TwoFactor: true, GoogleLogin: true}
	if r, err := st.LoginRequirements(ctx, primary.ID); err != nil || r != (core.LoginRequirements{TwoFactor: true, GoogleLogin: true, Organization: both}) {
		t.Fatalf("LoginRequirements with both org settings = (%+v, %v)", r, err)
	}
	names, err := st.GoogleLoginWorkspaces(ctx, owner)
	want := []string{orgName + " / " + primary.Label(), orgName + " / ol-second-" + stamp}
	if err != nil || !slices.Equal(names, want) {
		t.Fatalf("GoogleLoginWorkspaces = (%v, %v), want %v", names, err, want)
	}

	// Turning the org settings off restores each workspace's own values: the
	// secondary's own 2FA, set meanwhile, stays; nothing names the org.
	if _, _, err := st.SetTenantRequireTwoFactor(ctx, secondary.ID, true); err != nil {
		t.Fatal(err)
	}
	if changed, err := st.SetOrganizationRequireTwoFactor(ctx, org.ID, false); err != nil || !changed {
		t.Fatalf("disable 2FA = (changed=%v, %v)", changed, err)
	}
	if changed, err := st.SetOrganizationRequireGoogleLogin(ctx, org.ID, false); err != nil || !changed {
		t.Fatalf("disable Google = (changed=%v, %v)", changed, err)
	}
	if r, err := st.LoginRequirements(ctx, secondary.ID); err != nil || r != (core.LoginRequirements{TwoFactor: true}) {
		t.Fatalf("secondary after the org turned off = (%+v, %v), want its own 2FA alone", r, err)
	}
	if r, err := st.LoginRequirements(ctx, primary.ID); err != nil || r != (core.LoginRequirements{}) {
		t.Fatalf("primary after the org turned off = (%+v, %v), want nothing", r, err)
	}
	if names, err := st.GoogleLoginWorkspaces(ctx, owner); err != nil || len(names) != 0 {
		t.Fatalf("GoogleLoginWorkspaces after = (%v, %v), want none", names, err)
	}

	// An org that does not exist: its setters change nothing, its own read
	// requires nothing.
	if changed, err := st.SetOrganizationRequireTwoFactor(ctx, "org-missing"+stamp, true); err != nil || changed {
		t.Fatalf("setting a missing org's 2FA = (changed=%v, %v), want no change", changed, err)
	}
	if r, err := st.OrganizationLoginRequirements(ctx, "org-missing"+stamp); err != nil || r != (core.LoginRequirements{}) {
		t.Fatalf("a missing org's requirements = (%+v, %v)", r, err)
	}
}

// w7/m466: a workspace moved into an enforcing org by support's transfer reads
// the org's requirement at once, and an org dissolved by a downgrade while a
// requirement is on leaves its primary with its own settings alone.
func TestPGOrganizationLoginRequirementsFollowMembershipOfTheOrg(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	owner := "olm-owner-" + stamp
	primary, err := st.CreateTenantWithMember(ctx, owner, PlanScale)
	if err != nil {
		t.Fatal(err)
	}
	_, org, _, err := st.UpgradeToOrganization(ctx, primary.ID, PlanScale, []string{owner})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := st.CreateWorkspace(ctx, "olm-moved-"+stamp, PlanPro, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.DeleteTenant(context.Background(), moved.ID)
		_ = st.DeleteTenant(context.Background(), primary.ID)
	})
	if _, err := st.SetOrganizationRequireTwoFactor(ctx, org.ID, true); err != nil {
		t.Fatal(err)
	}
	if r, err := st.LoginRequirements(ctx, moved.ID); err != nil || r.TwoFactor {
		t.Fatalf("a standalone workspace before its transfer = (%+v, %v)", r, err)
	}
	if _, _, err := st.AttachWorkspaceToOrganization(ctx, moved.ID, org.ID); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if r, err := st.LoginRequirements(ctx, moved.ID); err != nil || !r.TwoFactor || r.Organization.ID != org.ID {
		t.Fatalf("a transferred workspace = (%+v, %v), want the org's 2FA", r, err)
	}

	// Dissolving needs the primary alone: delete the moved workspace first.
	if err := st.DeleteTenant(ctx, moved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DissolveOrganization(ctx, org.ID, PlanPro); err != nil {
		t.Fatalf("dissolve: %v", err)
	}
	if r, err := st.LoginRequirements(ctx, primary.ID); err != nil || r != (core.LoginRequirements{}) {
		t.Fatalf("the primary after its org dissolved = (%+v, %v), want its own settings alone", r, err)
	}
}
