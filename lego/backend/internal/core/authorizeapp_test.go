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

// authorizeapp_test.go covers w6/m17: AuthorizeApp collapses the old
// Authorize(relation) + GetApp(relation, name) pair into one seam that
// authorizes a named App against its OWN workspace, not the caller's default
// one. The case that matters most is TestAuthorizeAppFixesTheCallerDefault
// WorkspaceIntersectionBug below — the exact w6/013 scenario.

// TestAuthorizeAppFixesTheCallerDefaultWorkspaceIntersectionBug is the w6/013
// reproduction, at the core.Base level: bob's DEFAULT workspace (tea-team) is
// one he was invited into as a viewer; he separately owns tea-mine (admin)
// where his App "mine-web" actually lives. The old two-gate design checked
// can_operate against bob's default (tea-team) FIRST, via a standalone
// Authorize call, and denied him there before GetApp ever got a chance to
// look at the App's real workspace — a false negative purely from checking
// the wrong workspace. AuthorizeApp authorizes against the App's own
// workspace instead, so this must succeed.
func TestAuthorizeAppFixesTheCallerDefaultWorkspaceIntersectionBug(t *testing.T) {
	cl := fakeAppClient(sampleApp("mine-web", "tea-mine"))
	b := &Base{
		Client:    cl,
		Namespace: "default",
		// bob's oldest (default) membership is tea-team; he also belongs to
		// tea-mine, which the App actually lives in.
		Workspace: multiWorkspace{"bob": {"tea-team", "tea-mine"}},
		// bob is a viewer of tea-team (can_operate denied there) but admin of
		// tea-mine (can_operate allowed there).
		Authz: denyWorkspaceChecker{relation: RelCanOperate, object: WorkspaceObject("tea-team")},
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanOperate, "mine-web"); err != nil {
		t.Errorf("AuthorizeApp(can_operate) on bob's OWN service: got %v, want it served — "+
			"the App's own workspace (tea-mine) must decide, not bob's default (tea-team)", err)
	}
}

func TestAuthorizeAppSameWorkspaceSucceeds(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-a"))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("same-workspace AuthorizeApp: %v", err)
	}
}

func TestAuthorizeAppResolvesTypedServiceID(t *testing.T) {
	app := sampleApp(CRName("tea-a", "web"), "tea-a")
	app.Labels[LabelAppID] = "srv-c185th5c2rvvnhbfiltg"
	app.Labels[LabelServiceName] = "web"
	b := &Base{
		Client: fakeAppClient(app), Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{},
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	got, err := b.AuthorizeApp(ctx, RelCanOperate, "srv-c185th5c2rvvnhbfiltg")
	if err != nil {
		t.Fatalf("AuthorizeApp by typed id: %v", err)
	}
	if got.Name != CRName("tea-a", "web") {
		t.Fatalf("typed id resolved App %q, want tenant-prefixed CR", got.Name)
	}
}

func TestAuthorizeAppResolvesTypedPublicID(t *testing.T) {
	a := sampleApp("tea-a-web", "tea-a")
	a.Labels[LabelServiceName] = "Customer API"
	a.Labels[LabelAppID] = "srv-d9example"
	b := &Base{
		Client: fakeAppClient(a), Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{},
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	got, err := b.AuthorizeApp(ctx, RelCanOperate, "srv-d9example")
	if err != nil {
		t.Fatalf("AuthorizeApp by typed id: %v", err)
	}
	if got.Name != "tea-a-web" {
		t.Fatalf("resolved App = %q, want tea-a-web", got.Name)
	}
}

func TestAuthorizeAppTypedIDAuditsCanonicalPublicName(t *testing.T) {
	a := sampleApp("tea-a-web", "tea-a")
	a.Labels[LabelServiceName] = "Customer API"
	a.Labels[LabelAppID] = "srv-d9example"
	sink := &recordingSink{}
	b := &Base{
		Client: fakeAppClient(a), Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{}, Audit: sink,
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanOperate, "srv-d9example"); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 || sink.events[0].Target != ServiceTarget("Customer API") {
		t.Fatalf("audit events = %#v, want canonical public-name target", sink.events)
	}
}

func TestAuthorizeAppTypedPublicIDStillEnforcesOwningWorkspace(t *testing.T) {
	a := sampleApp("tea-b-web", "tea-b")
	a.Labels[LabelAppID] = "srv-d9other"
	b := &Base{
		Client: fakeAppClient(a), Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: fakeDenyChecker{},
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "srv-d9other"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-workspace typed id: got %v, want ErrForbidden", err)
	}
}

func TestAuthorizeAppCrossWorkspaceStillChecksTheRelationThere(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-b"))
	b := &Base{
		Client: cl, Namespace: "default", Workspace: aliceResolver(),
		Authz: denyWorkspaceChecker{relation: RelCanCreate, object: WorkspaceObject("tea-b")},
	}
	ctx := aliceCtx()

	if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("AuthorizeApp(can_view) on her viewable App: got %v, want served", err)
	}
	if _, err := b.AuthorizeApp(ctx, RelCanCreate, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("AuthorizeApp(can_create) where she lacks it: got %v, want ErrForbidden (no cross-workspace escalation)", err)
	}
}

func TestAuthorizeAppNotFoundIsForbiddenBeforeNotFoundForADeniedCaller(t *testing.T) {
	// 403-before-404: an unauthorized caller must not learn a name doesn't
	// exist by probing it.
	cl := fakeAppClient()
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: fakeDenyChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "ghost"); !errors.Is(err, ErrForbidden) {
		t.Errorf("AuthorizeApp on a missing App, denied caller: got %v, want ErrForbidden", err)
	}
}

func TestAuthorizeAppNotFoundIsNotFoundForAnAuthorizedCaller(t *testing.T) {
	cl := fakeAppClient()
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("AuthorizeApp on a missing App, authorized caller: got %v, want ErrNotFound", err)
	}
}

func TestAuthorizeAppUnlabeledAppIsForbiddenToEveryWorkspace(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", ""))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("AuthorizeApp on an unlabeled App: got %v, want ErrForbidden", err)
	}
}

func TestAuthorizeAppStoreOffIgnoresLabelsUnchanged(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-b"))
	b := &Base{Client: cl, Namespace: "default", Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("store-off AuthorizeApp: %v", err)
	}
}

func TestAuthorizeAppNamedWorkspaceNonMemberIsForbidden(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-a"))
	b := &Base{Client: cl, Namespace: "default", Workspace: aliceResolver(), Authz: &fakeAllowChecker{}}
	ctx := WithWorkspace(aliceCtx(), "tea-c") // alice is not a member of tea-c

	if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("AuthorizeApp naming a workspace alice does not belong to: got %v, want ErrForbidden", err)
	}
}

// TestAuthorizeAppAuditsOnceAgainstTheResourceWorkspace: a write-relation call
// records exactly one event, against the App's own workspace (not the
// caller's default), target = ServiceTarget(name) — "authorize there once,
// audit there once" (w6/013's design).
func TestAuthorizeAppAuditsOnceAgainstTheResourceWorkspace(t *testing.T) {
	sink := &recordingSink{}
	cl := fakeAppClient(sampleApp("mine-web", "tea-mine"))
	b := &Base{
		Client: cl, Namespace: "default",
		Workspace: multiWorkspace{"bob": {"tea-team", "tea-mine"}},
		Authz:     &fakeAllowChecker{},
		Audit:     sink,
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})

	if _, err := b.AuthorizeApp(ctx, RelCanOperate, "mine-web"); err != nil {
		t.Fatalf("AuthorizeApp: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("recorded %d audit events, want 1", len(sink.events))
	}
	e := sink.events[0]
	if want := WorkspaceObject("tea-mine"); e.Resource != want {
		t.Errorf("audit resource = %q, want %q (the App's own workspace, not bob's default tea-team)", e.Resource, want)
	}
	if want := ServiceTarget("mine-web"); e.Target != want {
		t.Errorf("audit target = %q, want %q", e.Target, want)
	}
	if e.Outcome != AuditAllowed {
		t.Errorf("audit outcome = %q, want %q", e.Outcome, AuditAllowed)
	}
}
