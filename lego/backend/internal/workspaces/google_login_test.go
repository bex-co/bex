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
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// google_login_test.go covers the setting half of w7/m465
// (docs/ADR012-auth.md §9b): "require Google login" beside the 2FA setting,
// with the same admin, plan, no-op and default-workspace rules, its own
// lock-out guard (the acting admin must be signed in with Google, with the
// Google account of their email) and Render's ChangeTeamAllowedLoginMethodsEvent.
// The guard is the real twofactor.Gate.

// googleAccounts is the gate's Google lookup: subject -> has the matching
// Google account; absent = not a Kratos identity.
type googleAccounts map[string]bool

func (g googleAccounts) GoogleAccountMatches(_ context.Context, subject string) (bool, bool, error) {
	matches, human := g[subject]
	return matches, human, nil
}

type googleFixture struct {
	*twoFactorFixture
	gate *twofactor.Gate
}

// newGoogleFixture: admin-1 has the matching Google account, admin-2 a
// personal one.
func newGoogleFixture(t *testing.T) *googleFixture {
	t.Helper()
	f := newTwoFactorFixture(t)
	gate := twofactor.New(twofactor.Config{
		Settings: storeSettings{f.store}, Enrollment: f.identities,
		Google: googleAccounts{"admin-1": true, "admin-2": false, "dev-1": true},
	})
	f.svc.GoogleLogin = gate
	return &googleFixture{twoFactorFixture: f, gate: gate}
}

func asGoogle(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession, Human: true, SignIn: core.SignInGoogle})
}

func asPassword(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession, Human: true, SignIn: core.SignInPassword})
}

func (a *auditCapture) googleRows(outcome core.AuditOutcome) []core.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []core.AuditEvent
	for _, e := range a.events {
		if e.Verb == core.AuditVerbSetRequireGoogleLogin && e.Outcome == outcome {
			out = append(out, e)
		}
	}
	return out
}

func TestSetRequireGoogleLogin_AnAdminTogglesItAndItIsAudited(t *testing.T) {
	f := newGoogleFixture(t)
	ctx := asGoogle("admin-1")

	w, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true)
	if err != nil || !w.RequireGoogleLogin || w.RequireTwoFactor {
		t.Fatalf("enable = (%+v, %v), want Google login alone", w, err)
	}
	if list, err := f.svc.List(ctx); err != nil || len(list) != 1 || !list[0].RequireGoogleLogin {
		t.Fatalf("List = (%+v, %v), want the flag on the listed workspace", list, err)
	}
	if len(f.changed) != 1 || f.changed[0] != f.workspace {
		t.Fatalf("cache invalidations = %v", f.changed)
	}
	rows := f.audit.googleRows(core.AuditAllowed)
	if len(rows) != 1 || rows[0].GoogleLoginRequiredTo == nil || !*rows[0].GoogleLoginRequiredTo ||
		rows[0].TwoFactorRequiredTo != nil || rows[0].Caller != "admin-1" || rows[0].Resource != core.WorkspaceObject(f.workspace) {
		t.Fatalf("audit rows = %+v, want one allowed row recording enabled=true", rows)
	}
	// A no-op records nothing.
	if _, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if rows := f.audit.googleRows(core.AuditAllowed); len(rows) != 1 || len(f.changed) != 1 {
		t.Fatalf("a no-op wrote %d rows / %v invalidations", len(rows), f.changed)
	}
	// Turning it off needs no Google sign-in: the admin is enforcing it now,
	// so the gate itself is what admits them.
	if w, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, false); err != nil || w.RequireGoogleLogin {
		t.Fatalf("disable = (%+v, %v)", w, err)
	}
	if rows := f.audit.googleRows(core.AuditAllowed); len(rows) != 2 || *rows[1].GoogleLoginRequiredTo {
		t.Fatalf("audit rows = %+v, want a second row recording enabled=false", rows)
	}
}

func TestSetRequireGoogleLogin_OnlyAnAdminMayChangeIt(t *testing.T) {
	f := newGoogleFixture(t)
	for _, subject := range []string{"dev-1", "viewer-1", "stranger"} {
		if _, err := f.svc.SetRequireGoogleLogin(asGoogle(subject), f.workspace, true); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s enable = %v, want ErrForbidden", subject, err)
		}
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); got.RequireGoogleLogin {
		t.Fatal("a refused mutation changed the setting")
	}
	if denied := f.audit.googleRows(core.AuditDenied); len(denied) != 3 {
		t.Fatalf("denied rows = %d, want one per refused caller under the Google-login verb", len(denied))
	}
}

// The lock-out guard is the gate's own check: an admin signed in with a
// password, or with a personal Google account, cannot turn it on.
func TestSetRequireGoogleLogin_AnAdminNotSignedInWithGoogleCannotLockThemselvesOut(t *testing.T) {
	f := newGoogleFixture(t)
	for name, ctx := range map[string]context.Context{
		"password sign-in":         asPassword("admin-1"),
		"personal Gmail sign-in":   asGoogle("admin-2"),
		"an API key with no stamp": ctxAsAPIKey("key-1"),
	} {
		if name == "an API key with no stamp" {
			// A key with no creator stamp cannot be attributed: no Keys lookup
			// is wired, so the guard cannot establish it and fails closed.
			f.svc.Base.Authz = roleChecker{"admin-1": "admin", "admin-2": "admin", "key-1": "admin"}
			if _, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true); !errors.Is(err, core.ErrAuthzUnavailable) {
				t.Fatalf("%s = %v, want ErrAuthzUnavailable", name, err)
			}
			continue
		}
		if _, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginAdminNotSignedIn) {
			t.Fatalf("%s = %v, want the coded 409 %s", name, err, core.CodeWorkspaceGoogleLoginAdminNotSignedIn)
		}
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); got.RequireGoogleLogin {
		t.Fatal("the guard let the setting change")
	}
	if rows := f.audit.googleRows(core.AuditAllowed); len(rows) != 0 || len(f.changed) != 0 {
		t.Fatalf("a refused enable recorded %v / invalidated %v", rows, f.changed)
	}
}

func TestSetRequireGoogleLogin_EnforcingItNeedsPro(t *testing.T) {
	f := newGoogleFixture(t)
	ctx := asGoogle("admin-1")
	f.store.tenants[f.workspace] = store.Tenant{ID: f.workspace, Name: "acme", Plan: store.PlanHobby}
	_, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true)
	var coded *core.CodedError
	if !isCoded(err, core.ErrBadRequest, core.PlanFeatureRequiredCode) || !errors.As(err, &coded) ||
		!reflect.DeepEqual(coded.Params, map[string]any{"plan": "hobby", "feature": "googleLoginEnforcement", "requiredPlan": "pro"}) ||
		err.Error() != "bad request: Google login enforcement requires the Pro plan or higher; this workspace is on Hobby" {
		t.Fatalf("enable on Hobby = %v, want the coded PLAN_FEATURE_REQUIRED refusal naming Google login", err)
	}
	// A Hobby workspace that already enforces it keeps it and can turn it off.
	f.store.tenants[f.workspace] = store.Tenant{ID: f.workspace, Name: "acme", Plan: store.PlanHobby, RequireGoogleLogin: true}
	if w, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, true); err != nil || !w.RequireGoogleLogin {
		t.Fatalf("re-sending on Hobby = (%+v, %v), want the no-op", w, err)
	}
	if w, err := f.svc.SetRequireGoogleLogin(ctx, f.workspace, false); err != nil || w.RequireGoogleLogin {
		t.Fatalf("turning it off on Hobby = (%+v, %v)", w, err)
	}
}

func TestSetRequireGoogleLogin_UnavailableWithoutALookupAndOnTheDefaultWorkspace(t *testing.T) {
	f := newGoogleFixture(t)
	f.svc.GoogleLogin = nil // no Kratos admin reader
	if _, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), f.workspace, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginUnavailable) {
		t.Fatalf("enable without a lookup = %v, want the coded 409", err)
	}
	f.store.tenants[f.workspace] = store.Tenant{ID: f.workspace, Name: "acme", Plan: "pro", RequireGoogleLogin: true}
	if w, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), f.workspace, false); err != nil || w.RequireGoogleLogin {
		t.Fatalf("disable without a lookup = (%+v, %v), want allowed", w, err)
	}

	f.svc.GoogleLogin = f.gate
	f.store.tenants[core.DefaultTenant] = store.Tenant{ID: core.DefaultTenant, Name: core.DefaultTenant, Plan: "pro"}
	f.svc.Base.Authz = roleChecker{"admin-1": "admin"}
	if _, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), core.DefaultTenant, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceGoogleLoginUnavailable) {
		t.Fatalf("enable on the default workspace = %v, want the coded 409", err)
	}
}

// Once it is on, the gate refuses a member signed in with a password on the
// workspace's own reads, and the exempt list still shows them the setting.
func TestGoogleLoginBlocksAPasswordMemberButTheExemptListShowsWhy(t *testing.T) {
	f := newGoogleFixture(t)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	gate := twofactor.New(twofactor.Config{
		Settings: storeSettings{f.store}, Enrollment: f.identities,
		Google: googleAccounts{"admin-1": true}, Now: func() time.Time { return now },
	})
	f.svc.Base.TwoFactor = gate
	f.svc.GoogleLogin = gate
	f.svc.TwoFactorChanged = gate.Invalidate
	if _, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GetWorkspace(asPassword("admin-1"), f.workspace); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceGoogleLoginRequired) {
		t.Fatalf("GetWorkspace signed in with a password = %v, want the coded 403", err)
	}
	list, err := f.svc.List(asPassword("admin-1"))
	if err != nil || len(list) != 1 || !list[0].RequireGoogleLogin || list[0].IPAllowList != nil {
		t.Fatalf("the exempt List = (%+v, %v), want the setting shown and the IP rules withheld", list, err)
	}
	// The admin can still turn it off once signed in with Google.
	if _, err := f.svc.SetRequireGoogleLogin(asPassword("admin-1"), f.workspace, false); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceGoogleLoginRequired) {
		t.Fatalf("disable signed in with a password = %v, want the gate's 403", err)
	}
	if w, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), f.workspace, false); err != nil || w.RequireGoogleLogin {
		t.Fatalf("disable signed in with Google = (%+v, %v)", w, err)
	}
}
