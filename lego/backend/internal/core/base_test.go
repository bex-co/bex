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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeWorkspace is a map-backed WorkspaceResolver for tests: identities not in
// the map resolve ok=false, exactly like a store-on caller with no tenant.
type fakeWorkspace map[string]string // Identity.Subject -> tenantID

func (f fakeWorkspace) Tenant(_ context.Context, id Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

// IsMember: a map-backed caller belongs to exactly the one workspace it
// resolves to — the single-membership case every pre-w6/m14 test is written
// against. Multi-membership callers use a richer fake (see the m14 tests).
func (f fakeWorkspace) IsMember(_ context.Context, id Identity, tenantID string) (bool, error) {
	tid, ok := f[id.Subject]
	return ok && tid == tenantID, nil
}

// fakeAllowChecker allows everything but records the object each Check saw —
// what the tests assert the workspace resolution produced.
type fakeAllowChecker struct{ lastObject string }

func (f *fakeAllowChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	f.lastObject = object
	return true, nil
}

func fakeAppClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestAuthorizeTargetsCallerWorkspace(t *testing.T) {
	chk := &fakeAllowChecker{}
	b := &Base{Authz: chk, Workspace: fakeWorkspace{"identity-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if chk.lastObject != "workspace:tea-a" {
		t.Errorf("checked object = %q, want workspace:tea-a", chk.lastObject)
	}
}

func TestAuthorizeUnresolvedCallerFallsBackToDefaultWorkspace(t *testing.T) {
	chk := &fakeAllowChecker{}
	// The resolver is wired (store on) but this subject has no tenant — an
	// unbound machine key, or the platform bootstrap.
	b := &Base{Authz: chk, Workspace: fakeWorkspace{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bex-bootstrap", Method: "oauth2"})

	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if chk.lastObject != DefaultWorkspace {
		t.Errorf("checked object = %q, want %s", chk.lastObject, DefaultWorkspace)
	}
}

func TestAuthorizeStoreOffTargetsDefaultWorkspaceUnchanged(t *testing.T) {
	chk := &fakeAllowChecker{}
	// Workspace is nil: the legacy store-off mode. Behavior must be
	// byte-identical to before tenant onboarding existed, regardless of subject.
	b := &Base{Authz: chk}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if chk.lastObject != DefaultWorkspace {
		t.Errorf("checked object = %q, want %s", chk.lastObject, DefaultWorkspace)
	}
}

func sampleApp(name, tenantID string) *appv1alpha1.App {
	a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	if tenantID != "" {
		a.Labels = map[string]string{LabelTenant: tenantID}
	}
	return a
}

func TestGetAppCrossTenantIsForbidden(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-b"))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.GetApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("cross-tenant GetApp: got %v, want ErrForbidden", err)
	}
}

func TestGetAppSameTenantSucceeds(t *testing.T) {
	cl := fakeAppClient(sampleApp("web", "tea-a"))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.GetApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("same-tenant GetApp: %v", err)
	}
}

func TestGetAppNoIdentitySkipsTenantGate(t *testing.T) {
	// The git webhook's HMAC-authenticated redeploy carries no core.Identity —
	// it must not be blocked by a tenant gate it has no subject to check.
	cl := fakeAppClient(sampleApp("web", "tea-a"))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}}

	if _, err := b.GetApp(context.Background(), RelCanView, "web"); err != nil {
		t.Errorf("no-identity GetApp: %v", err)
	}
}

func TestGetAppStoreOffIgnoresLabelsUnchanged(t *testing.T) {
	// Workspace nil: the legacy store-off mode. A caller may fetch any App
	// regardless of its (possibly stale/absent) tenant label — byte-identical
	// to before tenant onboarding existed.
	cl := fakeAppClient(sampleApp("web", "tea-b"))
	b := &Base{Client: cl, Namespace: "default"}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.GetApp(ctx, RelCanView, "web"); err != nil {
		t.Errorf("store-off GetApp: %v", err)
	}
}

func TestGetAppUnlabeledAppIsForbiddenToEveryTenant(t *testing.T) {
	// An App with no tenant label (e.g. hand-applied, or a pre-m9 CR) resolves
	// its label to "", which never matches a real tenant id — forbidden, not a
	// silent leak to whichever caller asks first.
	cl := fakeAppClient(sampleApp("web", ""))
	b := &Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	if _, err := b.GetApp(ctx, RelCanView, "web"); !errors.Is(err, ErrForbidden) {
		t.Errorf("unlabeled App GetApp: got %v, want ErrForbidden", err)
	}
}

func TestTenantHelperMirrorsWorkspaceResolver(t *testing.T) {
	b := &Base{Workspace: fakeWorkspace{"identity-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})

	tid, ok := b.Tenant(ctx)
	if !ok || tid != "tea-a" {
		t.Errorf("Tenant = %q,%v want tea-a,true", tid, ok)
	}

	if _, ok := (&Base{}).Tenant(ctx); ok {
		t.Error("nil Workspace: Tenant must report ok=false")
	}
	if _, ok := b.Tenant(context.Background()); ok {
		t.Error("no identity in context: Tenant must report ok=false")
	}
}
