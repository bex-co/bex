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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func protectedSecretScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	return scheme
}

// TestRejectProtectedSecretRefs is the codex F1 guard: a tenant App may not wire
// an operator-managed protected Secret (the shared S3 backup credential) into any
// kubelet-projected reference. A reference to an ordinary Secret still passes.
func TestRejectProtectedSecretRefs(t *testing.T) {
	scheme := protectedSecretScheme(t)
	protected := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bex-tenant-postgres", Namespace: "tea-a",
			Labels: map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount},
		},
		Data: map[string][]byte{"AWS_SECRET_ACCESS_KEY": []byte("shared")},
	}
	ordinary := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "tea-a"},
		Data:       map[string][]byte{"FOO": []byte("bar")},
	}
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(protected, ordinary).Build()
	r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
	base := func() *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"}}
	}

	// Every projection field that names the protected Secret must be refused.
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"envFromSecret":  func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bex-tenant-postgres" },
		"envFromSecrets": func(a *appv1alpha1.App) { a.Spec.EnvFromSecrets = []string{"bex-tenant-postgres"} },
		"filesFromSecrets": func(a *appv1alpha1.App) {
			a.Spec.FilesFromSecrets = []string{"bex-tenant-postgres"}
		},
		"env secretKeyRef": func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "X", ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "bex-tenant-postgres", Key: "AWS_SECRET_ACCESS_KEY"}}}}
		},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			app := base()
			mutate(app)
			if err := r.rejectProtectedSecretRefs(context.Background(), app); err == nil {
				t.Fatalf("%s referencing a protected Secret was ACCEPTED — backup credential exfil", name)
			}
		})
	}

	// An ordinary Secret reference, and no references at all, both pass.
	okApp := base()
	okApp.Spec.EnvFromSecret = "web-env"
	if err := r.rejectProtectedSecretRefs(context.Background(), okApp); err != nil {
		t.Fatalf("ordinary secret reference was refused: %v", err)
	}
	if err := r.rejectProtectedSecretRefs(context.Background(), base()); err != nil {
		t.Fatalf("App with no secret references was refused: %v", err)
	}
}

// TestMirrorPreDeploySecretsFailsClosedOnForeignCollision is the codex F4 guard:
// when a tenant-referenced pre-deploy Secret is absent in the App namespace, the
// mirror must not silently proceed if a same-named FOREIGN Secret already exists
// in the shared build namespace — the pre-deploy Job would otherwise mount that
// (platform) Secret. Absent in both namespaces stays the tolerated optional case.
func TestMirrorPreDeploySecretsFailsClosedOnForeignCollision(t *testing.T) {
	scheme := protectedSecretScheme(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"},
		Spec:       appv1alpha1.AppSpec{EnvFromSecret: "migrate-env"},
	}

	// Foreign (platform) Secret occupies the name in the build namespace, and the
	// App-namespace source is absent => must fail closed.
	foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "migrate-env", Namespace: "bex-build"}}
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()
	r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
	if err := r.mirrorPreDeploySecrets(context.Background(), app, "bex-build"); err == nil {
		t.Fatal("missing tenant source with a foreign build-namespace Secret was ACCEPTED — platform secret mount")
	}

	// Absent in BOTH namespaces => benign optional case, no error.
	empty := fake.NewClientBuilder().WithScheme(scheme).Build()
	r2 := &AppReconciler{Client: empty, BuildClient: empty}
	if err := r2.mirrorPreDeploySecrets(context.Background(), app, "bex-build"); err != nil {
		t.Fatalf("absent-in-both optional reference was refused: %v", err)
	}
}
