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

package core

import (
	"context"
	"errors"
	"testing"
)

// workspace_test.go covers w6/m14: a caller belongs to SEVERAL workspaces, so
// "which workspace does this request act in" stops being a lookup and becomes a
// decision — the explicit one they named (membership-checked), else their
// default. The cases that matter are the two ways that decision can go wrong:
// a request landing in a workspace the caller may not touch (confused deputy),
// and a request being refused in one they own (the w6/m11 live 403).

// multiWorkspace is a WorkspaceResolver for a caller with MANY memberships:
// subject -> its workspaces, oldest first (the store's ORDER BY membership
// created_at). Tenant answers the oldest — the default workspace; IsMember
// answers for any of them.
type multiWorkspace map[string][]string

func (m multiWorkspace) Tenant(_ context.Context, id Identity) (string, bool) {
	ws := m[id.Subject]
	if len(ws) == 0 {
		return "", false
	}
	return ws[0], true
}

func (m multiWorkspace) IsMember(_ context.Context, id Identity, tenantID string) (bool, error) {
	for _, w := range m[id.Subject] {
		if w == tenantID {
			return true, nil
		}
	}
	return false, nil
}

// brokenWorkspace is a resolver whose membership store is down — the fail-closed
// case (an outage must not read as "not a member", and must never fall back to
// the caller's own workspace).
type brokenWorkspace struct{ multiWorkspace }

func (brokenWorkspace) IsMember(context.Context, Identity, string) (bool, error) {
	return false, errors.New("membership store unreachable")
}

// denyWorkspaceChecker denies one relation on one workspace object and allows
// everything else — a caller who is, say, admin of A but only a viewer of B.
type denyWorkspaceChecker struct{ relation, object string }

func (c denyWorkspaceChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == c.relation && object == c.object), nil
}

// alice belongs to two workspaces: tea-a (older, her default) and tea-b.
func aliceCtx() context.Context {
	return WithIdentity(context.Background(), Identity{Subject: "alice", Method: "session"})
}

func aliceResolver() multiWorkspace {
	return multiWorkspace{"alice": {"tea-a", "tea-b"}}
}

func TestAuthorizeUsesDefaultWorkspaceWhenNoneNamed(t *testing.T) {
	chk := &fakeAllowChecker{}
	b := &Base{Workspace: aliceResolver(), Authz: chk}

	if err := b.Authorize(aliceCtx(), RelCanCreate); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if want := WorkspaceObject("tea-a"); chk.lastObject != want {
		t.Errorf("checked object = %q, want %q (the caller's default = oldest membership)", chk.lastObject, want)
	}
}

func TestAuthorizeUsesNamedWorkspaceForAMember(t *testing.T) {
	chk := &fakeAllowChecker{}
	b := &Base{Workspace: aliceResolver(), Authz: chk}
	ctx := WithWorkspace(aliceCtx(), "tea-b")

	if err := b.Authorize(ctx, RelCanCreate); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if want := WorkspaceObject("tea-b"); chk.lastObject != want {
		t.Errorf("checked object = %q, want %q — naming a workspace must move the check to it", chk.lastObject, want)
	}
	if tid, ok := b.Tenant(ctx); !ok || tid != "tea-b" {
		t.Errorf("Tenant = (%q, %v), want (tea-b, true) — a create must be stamped with the workspace it was authorized against", tid, ok)
	}
}

func TestNamedWorkspaceForNonMemberIsForbiddenNotRedirected(t *testing.T) {
	// The confused-deputy case: alice is not in tea-c. The request must be
	// REFUSED — not quietly served against tea-a, which would create her
	// resource in the wrong workspace while she believed it landed in tea-c.
	chk := &fakeAllowChecker{}
	b := &Base{Workspace: aliceResolver(), Authz: chk}
	ctx := WithWorkspace(aliceCtx(), "tea-c")

	if err := b.Authorize(ctx, RelCanCreate); !errors.Is(err, ErrForbidden) {
		t.Errorf("Authorize with a non-member workspace: got %v, want ErrForbidden", err)
	}
	if chk.lastObject != "" {
		t.Errorf("OpenFGA was consulted (object %q) — the membership refusal must short-circuit, never fall back to another workspace", chk.lastObject)
	}
	if tid, ok := b.Tenant(ctx); ok {
		t.Errorf("Tenant = (%q, true), want ok=false — a refused workspace must not resolve to the caller's own", tid)
	}
}

func TestNamedWorkspaceFailsClosedWhenMembershipStoreIsDown(t *testing.T) {
	b := &Base{Workspace: brokenWorkspace{aliceResolver()}, Authz: &fakeAllowChecker{}}
	ctx := WithWorkspace(aliceCtx(), "tea-b")

	err := b.Authorize(ctx, RelCanCreate)
	if !errors.Is(err, ErrAuthzUnavailable) {
		t.Errorf("Authorize with an unreachable membership store: got %v, want ErrAuthzUnavailable (fail closed)", err)
	}
}

// TestGetAppInAnotherOwnedWorkspaceIsNotForbidden is the w6/m11 regression: an
// owner of two workspaces was 403'd on their OWN App whenever the implicit
// resolution picked their other workspace. Fails before w6/m14.
func TestGetAppInAnotherOwnedWorkspaceIsNotForbidden(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-b")) // the App lives in her SECOND workspace
	b := &Base{Client: cl, Namespace: "default", Workspace: aliceResolver(), Authz: &fakeAllowChecker{}}

	// No workspace named: resolution picks tea-a (her default) — the App is in tea-b.
	if _, err := b.GetApp(aliceCtx(), RelCanView, "web"); err != nil {
		t.Errorf("GetApp on an App in the caller's OTHER workspace: got %v, want it served — the App's own workspace decides, not the caller's default", err)
	}
}

// TestGetAppInAnotherWorkspaceStillChecksTheRelationThere is what makes the
// regression above safe to fix. Roles are PER workspace: an admin of tea-a may
// be only a viewer of tea-b. Reaching an App by name must not carry tea-a's
// permissions into tea-b.
func TestGetAppInAnotherWorkspaceStillChecksTheRelationThere(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-b"))
	b := &Base{
		Client: cl, Namespace: "default", Workspace: aliceResolver(),
		// alice may not create/delete in tea-b (she is a viewer there).
		Authz: denyWorkspaceChecker{relation: RelCanCreate, object: WorkspaceObject("tea-b")},
	}
	ctx := aliceCtx()

	// A read verb's relation holds in tea-b => served.
	if _, err := b.GetApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("GetApp(can_view) on her viewable App: got %v, want it served", err)
	}
	// A destructive verb's relation does NOT hold in tea-b => refused, even
	// though she IS a member of tea-b and IS an admin of her default workspace.
	if _, err := b.GetApp(ctx, RelCanCreate, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("GetApp(can_create) on an App in a workspace where she lacks it: got %v, want ErrForbidden (no cross-workspace privilege escalation)", err)
	}
}

func TestGetAppInAWorkspaceTheCallerDoesNotBelongToIsForbidden(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-stranger"))
	b := &Base{
		Client: cl, Namespace: "default", Workspace: aliceResolver(),
		Authz: &fakeAllowChecker{}, // even with authorization wide open
	}

	if _, err := b.GetApp(aliceCtx(), RelCanView, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("GetApp on a stranger's App: got %v, want ErrForbidden — membership is the floor, whatever OpenFGA says", err)
	}
}

func TestGetAppNamedWorkspaceDoesNotHideTheCallersOtherApps(t *testing.T) {
	// The App is in tea-a; the request explicitly acts in tea-b. tea-a is still
	// hers, so the relation holds there and the App is served — the named
	// workspace scopes where a CREATE lands (Tenant), it does not hide Apps she
	// may otherwise reach by name.
	cl := fakeAppClient(sampleApp("web", "tea-a"))
	b := &Base{Client: cl, Namespace: "default", Workspace: aliceResolver(), Authz: &fakeAllowChecker{}}
	ctx := WithWorkspace(aliceCtx(), "tea-b")

	if _, err := b.GetApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("GetApp: got %v, want served (the App is in another workspace of hers)", err)
	}
}

func TestWithWorkspaceEmptyIsAbsent(t *testing.T) {
	// An absent ownerId ("") must mean "my default workspace", never "workspace
	// with the empty id" — which would match an unlabeled App.
	ctx := WithWorkspace(context.Background(), "")
	if id, ok := WorkspaceFrom(ctx); ok {
		t.Errorf("WorkspaceFrom after WithWorkspace(\"\") = (%q, true), want ok=false", id)
	}
}

// TestAuditRecordsTheWorkspaceTheCallerTriedToReach: a denied cross-workspace
// write is recorded against the workspace it TARGETED, not the caller's own —
// otherwise the audit log would show an attempt that never happened in a
// workspace, and hide the one that did.
func TestAuditRecordsTheWorkspaceTheCallerTriedToReach(t *testing.T) {
	sink := &recordingSink{}
	b := &Base{Workspace: aliceResolver(), Authz: &fakeAllowChecker{}, Audit: sink}

	if err := b.Authorize(WithWorkspace(aliceCtx(), "tea-c"), RelCanCreate); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Authorize: got %v, want ErrForbidden", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1 (a denial is still a write attempt)", len(sink.events))
	}
	e := sink.events[0]
	if want := WorkspaceObject("tea-c"); e.Resource != want {
		t.Errorf("audit resource = %q, want %q", e.Resource, want)
	}
	if e.Outcome != AuditDenied {
		t.Errorf("audit outcome = %q, want %q", e.Outcome, AuditDenied)
	}
}

type recordingSink struct{ events []AuditEvent }

func (s *recordingSink) Record(_ context.Context, e AuditEvent) error {
	s.events = append(s.events, e)
	return nil
}

// TestGetAppCollisionFallbackResolvesActingWorkspaceOnce is w4/027: GetApp's
// LabelServiceName fallback loop used to call AuthorizeLabeled for each
// candidate, and AuthorizeLabeled re-ran resolveWorkspace (and its
// Workspace.Tenant store query) once per candidate even though the acting
// workspace is ctx-invariant across the whole loop. GetApp now resolves once
// and propagates the cached context, so N colliding candidates cost exactly one
// Tenant() call.
func TestGetAppCollisionFallbackResolvesActingWorkspaceOnce(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	t.Run("denied — every candidate checked", func(t *testing.T) {
		apps := collidingApps(5)
		calls := 0
		b := &Base{
			Client:    fakeAppClient(appObjects(apps)...),
			Namespace: "default",
			Workspace: countingWorkspaceResolver{fakeWorkspace{"identity-a": "tea-a"}, &calls},
			Authz:     fakeDenyChecker{},
		}
		if _, err := b.GetApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if calls != 1 {
			t.Errorf("Workspace.Tenant called %d times for %d colliding candidates, want exactly 1", calls, len(apps))
		}
	})

	t.Run("allowed partway through — short-circuits, still exactly one resolution", func(t *testing.T) {
		apps := collidingApps(5)
		allowed := apps[2]
		calls := 0
		b := &Base{
			Client:    fakeAppClient(appObjects(apps)...),
			Namespace: "default",
			Workspace: countingWorkspaceResolver{multiWorkspace{"identity-a": {"tea-a", allowed.Labels[LabelTenant]}}, &calls},
			Authz:     denyAllButChecker{allowed: WorkspaceObject(allowed.Labels[LabelTenant])},
		}
		got, err := b.GetApp(ctx, RelCanView, "web")
		if err != nil {
			t.Fatalf("GetApp: %v", err)
		}
		if got.Name != allowed.Name {
			t.Fatalf("served %q, want %q", got.Name, allowed.Name)
		}
		if calls != 1 {
			t.Errorf("Workspace.Tenant called %d times, want exactly 1", calls)
		}
	})
}
