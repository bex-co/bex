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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// loginStatuses is metadataIdentities with what each account holds to sign
// in with, the Kratos admin read orgUserAuthStatus makes; a subject named in
// failing cannot be read.
type loginStatuses struct {
	metadataIdentities
	of      map[string]core.LoginStatus
	failing string
}

func (l *loginStatuses) LoginStatus(_ context.Context, subject string) (core.LoginStatus, bool, error) {
	if subject == l.failing {
		return core.LoginStatus{}, false, errors.New("kratos down")
	}
	status, ok := l.of[subject]
	return status, ok, nil
}

var _ workspaces.IdentityReader = &loginStatuses{}

// Render's orgUserAuthStatus (w1/m657) over a real Postgres and OpenFGA: an
// Owner reads any member's login methods, in Render's names and order, and
// 2FA, by the member's subject or userId; a Member asking, an Owner asking
// about someone the org does not hold, and an unknown user get the one 403;
// someone outside the org is told it was not found; an unreadable account is
// an error, never a member who reads as signed in with nothing.
func TestOrgUserAuthStatusE2E(t *testing.T) {
	statuses := &loginStatuses{of: map[string]core.LoginStatus{}}
	e := newOrgE2E(t, func(_ *core.Base, d *Deps) { d.Identities = statuses })
	ctx := context.Background()
	alice, bob, carol := e.person("alice"), e.person("bob"), e.person("carol")
	statuses.of[alice.Subject] = core.LoginStatus{Password: true, Google: true, TwoFactor: true}
	statuses.of[bob.Subject] = core.LoginStatus{GitHub: true}
	statuses.of[carol.Subject] = core.LoginStatus{Password: true}

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
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { id } }`, acme.ID))
	orgID := e.ok(alice, `{ organizations { id } }`)["organizations"].([]any)[0].(map[string]any)["id"].(string)

	status := func(caller core.Identity, userID string) map[string]any {
		return e.gql(caller, fmt.Sprintf(`{ orgUserAuthStatus(userID: %q, orgID: %q) { loginMethodStatuses { method signedIn } is2FAEnabled } }`, userID, orgID))
	}
	want := func(pwd, google, github bool) []any {
		return []any{
			map[string]any{"method": "pwd", "signedIn": pwd},
			map[string]any{"method": "google", "signedIn": google},
			map[string]any{"method": "github", "signedIn": github},
		}
	}
	read := func(out map[string]any) (any, any) {
		t.Helper()
		if errs, _ := out["errors"].([]any); len(errs) > 0 {
			t.Fatalf("orgUserAuthStatus: %v", errs)
		}
		got := out["data"].(map[string]any)["orgUserAuthStatus"].(map[string]any)
		return got["loginMethodStatuses"], got["is2FAEnabled"]
	}
	if methods, twoFA := read(status(alice, bob.Subject)); !reflect.DeepEqual(methods, want(false, false, true)) || twoFA != false {
		t.Errorf("bob = %v 2FA %v", methods, twoFA)
	}
	if methods, twoFA := read(status(alice, alice.Subject)); !reflect.DeepEqual(methods, want(true, true, false)) || twoFA != true {
		t.Errorf("alice = %v 2FA %v", methods, twoFA)
	}
	// By the userId the members list shows, Render's user.id.
	var bobUserID string
	for _, m := range e.ok(alice, fmt.Sprintf(`{ organizationMembers(organizationId: %q) { subject userId } }`, orgID))["organizationMembers"].([]any) {
		if m.(map[string]any)["subject"] == bob.Subject {
			bobUserID, _ = m.(map[string]any)["userId"].(string)
		}
	}
	if !strings.HasPrefix(bobUserID, "usr-") {
		t.Fatalf("bob's userId = %q", bobUserID)
	}
	if methods, _ := read(status(alice, bobUserID)); !reflect.DeepEqual(methods, want(false, false, true)) {
		t.Errorf("bob by userId = %v", methods)
	}

	// The one refusal, whoever asks about whom.
	for name, out := range map[string]map[string]any{
		"a Member asking":                   status(bob, alice.Subject),
		"an Owner asking about an outsider": status(alice, carol.Subject),
		"an Owner asking about nobody":      status(alice, "usr-nobody"),
	} {
		if code := graphqlCode(out); code != "FORBIDDEN" {
			t.Errorf("%s answered %q, want FORBIDDEN", name, code)
		}
	}
	if code := graphqlCode(status(carol, bob.Subject)); code != core.CodeOrganizationNotFound {
		t.Errorf("someone outside the org answered %q", code)
	}
	statuses.failing = bob.Subject
	out := status(alice, bob.Subject)
	if errs, _ := out["errors"].([]any); len(errs) == 0 {
		t.Errorf("an unreadable account answered %v, want an error", out["data"])
	}

	// A Member cannot list, mint or revoke the org's audit-log keys. Minting
	// and revoking refuse them as orgUserAuthStatus does, not by telling them
	// the org they are in was not found (production answered that, w1/m657
	// t007); someone outside the org is told it was not found. This server
	// has no key store, so the list only has to refuse.
	list := fmt.Sprintf(`{ organization(organizationId: %q) { organizationAPITokens { id } } }`, orgID)
	if errs, _ := e.gql(bob, list)["errors"].([]any); len(errs) == 0 {
		t.Errorf("a Member ran %s", list)
	}
	for _, q := range []string{
		fmt.Sprintf(`mutation { provisionOrganizationAuditLogToken(organizationId: %q, label: "x") { token } }`, orgID),
		fmt.Sprintf(`mutation { revokeOrganizationAPIToken(organizationId: %q, tokenId: "x") }`, orgID),
	} {
		if code := graphqlCode(e.gql(bob, q)); code != "FORBIDDEN" {
			t.Errorf("a Member running %s answered %q, want FORBIDDEN", q, code)
		}
		if code := graphqlCode(e.gql(carol, q)); code != core.CodeOrganizationNotFound {
			t.Errorf("someone outside the org running %s answered %q", q, code)
		}
	}
}
