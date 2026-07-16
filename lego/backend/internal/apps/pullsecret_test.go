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

package apps

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakePullSecrets is a stub PullSecretSource.
type fakePullSecrets struct {
	name           string
	ok             bool
	err            error
	calls          int
	lastImage      string
	lastWorkspace  string
	lastID         *string
	credentialName string
}

func (f *fakePullSecrets) RegistryCredentialName(_ context.Context, _, _ string) (string, bool) {
	return f.credentialName, f.credentialName != ""
}

func (f *fakePullSecrets) MaterializePullSecret(_ context.Context, workspaceID string, _ *appv1alpha1.App, image string, credentialID *string) (string, bool, error) {
	f.calls++
	f.lastWorkspace = workspaceID
	f.lastImage = image
	f.lastID = cloneStringPtr(credentialID)
	return f.name, f.ok, f.err
}

func rcService(rc PullSecretSource, apps ...*appv1alpha1.App) (*Service, client.Client) {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, RegistryCreds: rc}, cl
}

func TestCreateImageWithMatchingCredentialSetsExternalRegistryPullSecret(t *testing.T) {
	rc := &fakePullSecrets{name: "web-registry-pull", ok: true}
	svc, cl := rcService(rc)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "ghcr.io/acme/private:1.0"}); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.ExternalRegistryPullSecret != "web-registry-pull" {
		t.Errorf("spec.externalRegistryPullSecret = %q, want web-registry-pull", app.Spec.ExternalRegistryPullSecret)
	}
	if rc.lastImage != "ghcr.io/acme/private:1.0" {
		t.Errorf("materialize called with image %q", rc.lastImage)
	}
}

func TestCreateImageWithNoMatchingCredentialLeavesFieldEmpty(t *testing.T) {
	rc := &fakePullSecrets{ok: false} // no stored credential for this host
	svc, cl := rcService(rc)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "docker.io/library/nginx:1.25"}); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.ExternalRegistryPullSecret != "" {
		t.Errorf("no credential should leave the field empty, got %q", app.Spec.ExternalRegistryPullSecret)
	}
}

func TestCreateRepoBackedAppNeverCallsMaterialize(t *testing.T) {
	// A build-from-git App's image lands in the internal Zot registry, not an
	// external host — ensureExternalRegistryPullSecret must not even call the
	// seam (there's no Spec.Image to resolve a host from at create time).
	rc := &fakePullSecrets{name: "should-not-be-set", ok: true}
	svc, cl := rcService(rc)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/octo/app"}); err != nil {
		t.Fatal(err)
	}
	if rc.calls != 0 {
		t.Errorf("MaterializePullSecret called %d times for a repo-backed create, want 0", rc.calls)
	}
	app := getApp(t, cl, "web")
	if app.Spec.ExternalRegistryPullSecret != "" {
		t.Errorf("repo-backed App must not get an external pull secret, got %q", app.Spec.ExternalRegistryPullSecret)
	}
}

func TestMaterializeFailureSurfacesAsCreateError(t *testing.T) {
	// A credential DID match but materializing it failed — must fail the
	// create, never silently deploy without the pull secret it needs.
	rc := &fakePullSecrets{err: errors.New("openbao down")}
	svc, cl := rcService(rc)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "ghcr.io/acme/private:1.0"}); err == nil {
		t.Fatal("materialize failure must surface as a create error")
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err == nil {
		t.Error("app should not be created on materialize failure")
	}
}

func TestRedeployReResolvesExternalRegistryPullSecret(t *testing.T) {
	// A redeploy of an existing image-backed App re-checks for a matching
	// credential — picking up one stored after create, or dropping one whose
	// credential has since been deleted.
	existing := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/acme/private:1.0", AutoDeploy: true, ExternalRegistryPullSecret: "stale-secret"},
	}
	rc := &fakePullSecrets{ok: false} // credential since deleted
	svc, _ := rcService(rc, existing)

	if _, err := svc.redeploy(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if rc.calls != 1 {
		t.Errorf("redeploy called MaterializePullSecret %d times, want 1", rc.calls)
	}
}

func TestDeleteRemovesExternalRegistryPullSecret(t *testing.T) {
	// The registry-pull Secret carries no ownerRef (it can be materialized
	// before the App has a UID, during the App's own Create call) — Delete
	// must clean it up explicitly, mirroring the clone-secret Secret.
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/acme/private:1.0", ExternalRegistryPullSecret: "web-registry-pull"},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-registry-pull", Namespace: "default"}}
	svc, cl := rcService(&fakePullSecrets{}, app)
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	var got corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-registry-pull"}, &got); err == nil {
		t.Error("delete must remove the registry-pull secret")
	}
}

func TestDeleteWithNoRegistryPullSecretIsANoOp(t *testing.T) {
	// A public-image (or repo-backed) App never had a registry-pull secret to
	// begin with — delete must not error just because there's nothing to clean up.
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
	svc, _ := rcService(&fakePullSecrets{}, app)

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("delete with no registry-pull secret: %v", err)
	}
}
