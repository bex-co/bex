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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeCloneTokens is a stub CloneTokenSource.
type fakeCloneTokens struct {
	token string
	ok    bool
	err   error
	calls int
}

func (f *fakeCloneTokens) CloneToken(_ context.Context, _ string) (string, bool, error) {
	f.calls++
	return f.token, f.ok, f.err
}

func ghService(gh CloneTokenSource, apps ...*appv1alpha1.App) (*Service, client.Client) {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, GitHub: gh}, cl
}

func cloneSecretValue(t *testing.T, cl client.Client, name string) (string, bool) {
	t.Helper()
	var sec corev1.Secret
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &sec)
	if apierrors.IsNotFound(err) {
		return "", false
	}
	if err != nil {
		t.Fatalf("get secret %s: %v", name, err)
	}
	// The fake client surfaces StringData writes as-is (it does not fold them
	// into Data), so read whichever the write populated.
	if v, ok := sec.StringData["token"]; ok {
		return v, true
	}
	return string(sec.Data["token"]), true
}

func TestCreateConnectedRepoWritesCloneSecret(t *testing.T) {
	gh := &fakeCloneTokens{token: "ghs_new", ok: true}
	svc, cl := ghService(gh)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/octo/app"}); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.CloneSecret != "web-clone" {
		t.Fatalf("spec.cloneSecret = %q, want web-clone", app.Spec.CloneSecret)
	}
	if v, ok := cloneSecretValue(t, cl, "web-clone"); !ok || v != "ghs_new" {
		t.Fatalf("clone secret token = %q ok=%v", v, ok)
	}
}

func TestCreatePublicRepoNoCloneSecret(t *testing.T) {
	gh := &fakeCloneTokens{ok: false} // not a connected repo
	svc, cl := ghService(gh)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/pub/lic"}); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.CloneSecret != "" {
		t.Errorf("public repo must not set cloneSecret, got %q", app.Spec.CloneSecret)
	}
	if _, ok := cloneSecretValue(t, cl, "web-clone"); ok {
		t.Error("public repo must not write a clone secret")
	}
}

func TestRedeployRefreshesCloneSecret(t *testing.T) {
	// Existing app already has a clone secret; a push redeploy mints a new token.
	existing := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/octo/app", AutoDeploy: true, CloneSecret: "web-clone"},
	}
	stale := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-clone", Namespace: "default"},
		StringData: map[string]string{"token": "ghs_stale"},
	}
	gh := &fakeCloneTokens{token: "ghs_fresh", ok: true}
	svc, cl := ghService(gh, existing)
	if err := cl.Create(context.Background(), stale); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.redeploy(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if gh.calls != 1 {
		t.Errorf("redeploy called CloneToken %d times, want 1", gh.calls)
	}
	if v, ok := cloneSecretValue(t, cl, "web-clone"); !ok || v != "ghs_fresh" {
		t.Fatalf("secret not refreshed: %q ok=%v", v, ok)
	}
	app := getApp(t, cl, "web")
	if app.Spec.CloneSecret != "web-clone" || app.Spec.RestartedAt == "" {
		t.Errorf("redeploy spec = %+v", app.Spec)
	}
}

func TestDeployMintFailureSurfaces(t *testing.T) {
	// A GitHub failure on a connected repo must fail the deploy, never silently
	// fall back to a public clone.
	gh := &fakeCloneTokens{err: errors.New("github down")}
	svc, cl := ghService(gh)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/octo/app"}); err == nil {
		t.Fatal("mint failure must surface as a create error")
	}
	// And nothing half-written: no App CR created.
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); !apierrors.IsNotFound(err) {
		t.Errorf("app should not be created on mint failure, got err=%v", err)
	}
}

func TestDeleteRemovesCloneSecret(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/octo/app", CloneSecret: "web-clone"},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-clone", Namespace: "default"}, StringData: map[string]string{"token": "x"}}
	svc, cl := ghService(&fakeCloneTokens{}, app)
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cloneSecretValue(t, cl, "web-clone"); ok {
		t.Error("delete must remove the clone secret")
	}
}
