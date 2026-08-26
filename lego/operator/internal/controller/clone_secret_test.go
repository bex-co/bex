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

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCopyCloneSecretAcrossNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-clone", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"token": []byte("ghs_abc")},
	}
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(src).Build()
	r := &AppReconciler{Client: cachedClient, BuildClient: buildClient}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "uid-web"}}

	// Copy default/web-clone -> bex-system/web-clone (the build namespace).
	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "web-clone"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	var dst corev1.Secret
	if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "web-clone"}, &dst); err != nil {
		t.Fatalf("dst not created: %v", err)
	}
	if string(dst.Data["token"]) != "ghs_abc" {
		t.Errorf("copied token = %q, want ghs_abc", dst.Data["token"])
	}
	if dst.Type != corev1.SecretTypeOpaque {
		t.Errorf("copied type = %v", dst.Type)
	}
	if dst.Labels["app.bex.co/app-uid"] != "uid-web" || dst.Labels["app.bex.co/component"] != "copied-secret" {
		t.Fatalf("copied Secret missing canonical artifact labels: %v", dst.Labels)
	}

	// Idempotent + refreshes: change the source, copy again, dst updates.
	src.Data["token"] = []byte("ghs_fresh")
	if err := buildClient.Update(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "web-clone"); err != nil {
		t.Fatal(err)
	}
	_ = buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "web-clone"}, &dst)
	if string(dst.Data["token"]) != "ghs_fresh" {
		t.Errorf("refresh failed: token = %q, want ghs_fresh", dst.Data["token"])
	}
}

// TestCopyCloneSecretRefusesProtectedSource is the codex-security 2026-08 F7
// relocation guard: a source carrying LabelProtectedFromTenantMount (the shared
// S3 backup credential the operator projects into every datastore namespace)
// must not be copied into the shared build namespace under any circumstances —
// the denylist exists precisely to keep those Secrets out of tenant-named
// positions, and the old copier relocated them while REPLACING their labels,
// stripping the very marker other checks rely on.
func TestCopyCloneSecretRefusesProtectedSource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	protected := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bex-tenant-postgres",
			Namespace: "default",
			Labels:    map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount},
		},
		Data: map[string][]byte{"AWS_SECRET_ACCESS_KEY": []byte("shared")},
	}
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(protected).Build()
	r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "uid-web"}}

	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "bex-tenant-postgres"); err == nil {
		t.Fatal("copyCloneSecret ACCEPTED a protected source — the denylist is bypassable via spec.cloneSecret")
	}
	var leaked corev1.Secret
	if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "bex-tenant-postgres"}, &leaked); err == nil {
		t.Fatal("protected Secret was relocated into the shared build namespace")
	}
}

// TestCopyCloneSecretRelocatesOwnProtectedBuildSecrets is the production shape
// TestCopyCloneSecretAcrossNamespaces above never had: bex-api stamps
// LabelProtectedFromTenantMount on every clone / registry-pull Secret it mints
// (clonesecret.go, registrycreds/pullsecret.go), and production runs with
// BEX_BUILD_NAMESPACE set, so EVERY GitHub-connected App reaches this copier
// with a labeled source. Refusing it (w6/m97) failed the build one step past
// the guard, with the same total outage under a different status reason. The
// relocated copy must keep the marker.
func TestCopyCloneSecretRelocatesOwnProtectedBuildSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	protected := func(name string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tea-a", Labels: map[string]string{
				execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount,
			}},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"token": []byte("ghs_abc")},
		}
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"}}

	for _, name := range []string{
		appv1alpha1.CloneSecretName(app.Name),
		appv1alpha1.ExternalRegistryPullSecretName(app.Name),
	} {
		t.Run("relocates "+name, func(t *testing.T) {
			buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(protected(name)).Build()
			r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
			if err := r.copyCloneSecret(context.Background(), app, "tea-a", "bex-build", name); err != nil {
				t.Fatalf("copier refused this App's own build-plane Secret — every connected build fails: %v", err)
			}
			var dst corev1.Secret
			if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &dst); err != nil {
				t.Fatalf("dst not created: %v", err)
			}
			if string(dst.Data["token"]) != "ghs_abc" {
				t.Errorf("copied token = %q, want ghs_abc", dst.Data["token"])
			}
			// F7's original point: the marker must survive the relocation, or
			// the copy becomes mountable by any App in the build namespace.
			if dst.Labels[execution.LabelProtectedFromTenantMount] != execution.ProtectedFromTenantMount {
				t.Fatalf("relocated Secret lost its protected marker: %v", dst.Labels)
			}
		})
	}

	// The carve-out is this App's own pair only: another App's clone Secret,
	// with an identically-shaped name, is still unrelocatable.
	t.Run("refuses another App's protected clone secret", func(t *testing.T) {
		other := appv1alpha1.CloneSecretName("other")
		buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(protected(other)).Build()
		r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
		if err := r.copyCloneSecret(context.Background(), app, "tea-a", "bex-build", other); err == nil {
			t.Fatal("copier relocated another App's protected clone Secret into the shared build namespace")
		}
		var leaked corev1.Secret
		if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: other}, &leaked); err == nil {
			t.Fatal("another App's protected Secret reached the build namespace")
		}
	})
}
