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
	"fmt"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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

func TestAuthorizeAppTypedIDAuditsCanonicalCRName(t *testing.T) {
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
	if len(sink.events) != 1 || sink.events[0].Target != ServiceTarget("tea-a-web") {
		t.Fatalf("audit events = %#v, want namespace-unique CR-name target", sink.events)
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
	// Placed in the caller's own `<ws>` namespace (a plausible hand-apply
	// target) so the by-name lookup reaches it and exercises AuthorizeLabeled's
	// no-label rule, not GetApp's namespace resolution.
	app := sampleApp("web", "")
	app.Namespace = "tea-a"
	cl := fakeAppClient(app)
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

// collidingApps returns n Apps, each in its own workspace (tea-b, tea-c, …)
// but sharing LabelServiceName "web" — the AuthorizeApp name-collision
// fallback fixture shared by TestAuthorizeAppFallbackCapsPerCandidateDeniedAudits
// and TestAuthorizeAppCollisionFallbackResolvesActingWorkspaceOnce below.
func collidingApps(n int) []*appv1alpha1.App {
	apps := make([]*appv1alpha1.App, n)
	for i := range apps {
		a := sampleApp(fmt.Sprintf("tea-%c-web", 'b'+i), fmt.Sprintf("tea-%c", 'b'+i))
		a.Labels[LabelServiceName] = "web"
		apps[i] = a
	}
	return apps
}

// appObjects adapts a []*appv1alpha1.App to the []client.Object
// fakeAppClient's builder wants — the conversion every collidingApps call
// site otherwise repeats inline.
func appObjects(apps []*appv1alpha1.App) []client.Object {
	objs := make([]client.Object, len(apps))
	for i := range apps {
		objs[i] = apps[i]
	}
	return objs
}

// TestAuthorizeAppFallbackCapsPerCandidateDeniedAudits is w4/015: the
// LabelServiceName collision-fallback loop audits at most
// maxFallbackCandidateAudits rejected candidates individually, then records
// ONE aggregate denial row against the caller's own workspace — so a denied
// read of a pathologically common name can't serialize an unbounded run of
// synchronous audit writes (each bounded by auditRecordTimeout) or amplify
// one request into one insert per colliding tenant. Below the cap, behavior
// is unchanged: one row per rejected candidate, no aggregate row.
func TestAuthorizeAppFallbackCapsPerCandidateDeniedAudits(t *testing.T) {
	newBase := func(sink *recordingSink, apps []*appv1alpha1.App) *Base {
		return &Base{
			Client: fakeAppClient(appObjects(apps)...), Namespace: "default",
			Workspace: fakeWorkspace{"identity-a": "tea-a"},
			Authz:     fakeDenyChecker{}, Audit: sink,
		}
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	t.Run("below the cap every candidate keeps its own row", func(t *testing.T) {
		sink := &recordingSink{}
		b := newBase(sink, collidingApps(2))
		if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if len(sink.events) != 2 {
			t.Fatalf("recorded %d events, want 2 (one per rejected candidate, no aggregate)", len(sink.events))
		}
	})

	t.Run("past the cap the remainder collapses into one aggregate row", func(t *testing.T) {
		sink := &recordingSink{}
		b := newBase(sink, collidingApps(maxFallbackCandidateAudits+4))
		if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if want := maxFallbackCandidateAudits + 1; len(sink.events) != want {
			t.Fatalf("recorded %d events, want %d (%d per-candidate + 1 aggregate)",
				len(sink.events), want, maxFallbackCandidateAudits)
		}
		agg := sink.events[len(sink.events)-1]
		if agg.Resource != WorkspaceObject("tea-a") {
			t.Errorf("aggregate row Resource = %q, want the CALLER's workspace (%q)", agg.Resource, WorkspaceObject("tea-a"))
		}
		if agg.Target != ServiceTarget("web") || agg.Outcome != AuditDenied {
			t.Errorf("aggregate row = %+v, want a denied service:web row", agg)
		}
	})

	t.Run("an allowed candidate past the cap is still served", func(t *testing.T) {
		apps := collidingApps(maxFallbackCandidateAudits + 2)
		last := apps[len(apps)-1]
		sink := &recordingSink{}
		b := newBase(sink, apps)
		// The caller is a member of the LAST candidate's workspace (viewer
		// there) and denied everywhere else.
		b.Workspace = multiWorkspace{"identity-a": {"tea-a", last.Labels[LabelTenant]}}
		b.Authz = denyAllButChecker{allowed: WorkspaceObject(last.Labels[LabelTenant])}
		got, err := b.AuthorizeApp(ctx, RelCanView, "web")
		if err != nil {
			t.Fatalf("AuthorizeApp: %v (a candidate past the cap must still be checked and served)", err)
		}
		if got.Name != last.Name {
			t.Fatalf("served %q, want %q", got.Name, last.Name)
		}
	})
}

// countingWorkspaceResolver wraps a WorkspaceResolver and counts Tenant()
// calls — w4/m30's collision-fallback assertion that the acting workspace
// resolves exactly once for N colliding candidates, not once per candidate
// (resolveWorkspace calls Tenant on every invocation, so a call count directly
// measures the store round trips resourceWorkspaceFor's hoist eliminates).
type countingWorkspaceResolver struct {
	WorkspaceResolver
	tenantCalls *int
}

func (w countingWorkspaceResolver) Tenant(ctx context.Context, id Identity) (string, bool) {
	*w.tenantCalls++
	return w.WorkspaceResolver.Tenant(ctx, id)
}

// TestAuthorizeAppCollisionFallbackResolvesActingWorkspaceOnce is w4/m30/t003:
// N candidates sharing LabelServiceName each used to trigger their own
// resourceWorkspace call, which re-ran resolveWorkspace (and its
// Workspace.Tenant store query) once per candidate even though the acting
// workspace is ctx-invariant across the whole loop. AuthorizeApp now resolves
// it once at the top and threads it through resourceWorkspaceFor, so N
// colliding candidates must cost exactly one Tenant() call — asserted here
// with a call-counting Workspace double, on both the denied (exhausts every
// candidate) and allowed (short-circuits partway through) paths.
func TestAuthorizeAppCollisionFallbackResolvesActingWorkspaceOnce(t *testing.T) {
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
		if _, err := b.AuthorizeApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
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
		got, err := b.AuthorizeApp(ctx, RelCanView, "web")
		if err != nil {
			t.Fatalf("AuthorizeApp: %v", err)
		}
		if got.Name != allowed.Name {
			t.Fatalf("served %q, want %q", got.Name, allowed.Name)
		}
		if calls != 1 {
			t.Errorf("Workspace.Tenant called %d times, want exactly 1", calls)
		}
	})
}

// denyAllButChecker denies every workspace object except one — the
// "authorized only for the last colliding candidate" fixture above.
type denyAllButChecker struct{ allowed string }

func (c denyAllButChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object == c.allowed, nil
}

// strayDuplicatePair returns two App CRs sharing one typed id and one
// metadata.name: the canonical projection in its workspace's own namespace and
// a stray twin in the old shared namespace (see AppInOwnWorkspaceNamespace).
func strayDuplicatePair() (canonical, stray *appv1alpha1.App) {
	canonical = sampleApp("tea-a-web", "tea-a")
	canonical.Namespace = "tea-a"
	canonical.Labels[LabelAppID] = "srv-d9dup"
	stray = sampleApp("tea-a-web", "tea-a") // sampleApp pins Namespace to "default"
	stray.Labels[LabelAppID] = "srv-d9dup"
	return canonical, stray
}

// TestAuthorizeAppPrefersCanonicalNamespaceOverStrayDuplicate: when two CRs
// carry the same LabelAppID, the one living in its own workspace's namespace
// must win regardless of list order — a stray duplicate must never shadow the
// live projection.
func TestAuthorizeAppPrefersCanonicalNamespaceOverStrayDuplicate(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})
	canonical, stray := strayDuplicatePair()
	for name, objs := range map[string][]client.Object{
		"stray listed first": {stray, canonical},
		"stray listed last":  {canonical, stray},
	} {
		t.Run(name, func(t *testing.T) {
			b := &Base{
				Client: fakeAppClient(objs...), Namespace: "default",
				Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{},
			}
			got, err := b.AuthorizeApp(ctx, RelCanView, "srv-d9dup")
			if err != nil {
				t.Fatalf("AuthorizeApp: %v", err)
			}
			if got.Namespace != "tea-a" {
				t.Fatalf("served the CR in namespace %q, want the canonical workspace namespace tea-a", got.Namespace)
			}
		})
	}
}

// TestGetAppPrefersCanonicalNamespaceOverStrayDuplicate: same invariant on the
// GetApp seam every read feature (logs, metrics, secrets, …) shares.
func TestGetAppPrefersCanonicalNamespaceOverStrayDuplicate(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})
	canonical, stray := strayDuplicatePair()
	b := &Base{
		Client: fakeAppClient(stray, canonical), Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"}, Authz: &fakeAllowChecker{},
	}
	got, err := b.GetApp(ctx, RelCanView, "srv-d9dup")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if got.Namespace != "tea-a" {
		t.Fatalf("served the CR in namespace %q, want tea-a", got.Namespace)
	}
}

// TestAppNamespaceByNamePrefersCanonicalNamespace: the metrics resource-series
// funnel resolves an App's namespace from its metadata.name alone; with a
// stray duplicate present it must resolve to the workspace's own namespace —
// the one Prometheus's cAdvisor series actually carry — not first match.
func TestAppNamespaceByNamePrefersCanonicalNamespace(t *testing.T) {
	canonical, stray := strayDuplicatePair()
	b := &Base{Client: fakeAppClient(stray, canonical), Namespace: "bex-system"}
	if got := b.AppNamespaceByName(context.Background(), "tea-a-web"); got != "tea-a" {
		t.Fatalf("AppNamespaceByName = %q, want tea-a", got)
	}
	// A lone stray (no canonical twin) still resolves — its namespace is where
	// its pods, if any, live; better than the b.Namespace empty-series fallback.
	b = &Base{Client: fakeAppClient(stray), Namespace: "bex-system"}
	if got := b.AppNamespaceByName(context.Background(), "tea-a-web"); got != "default" {
		t.Fatalf("lone stray AppNamespaceByName = %q, want default", got)
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
