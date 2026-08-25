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
	"github.com/bex-co/bex/lego/operator/internal/publish"
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
		// Round-11 #1: the pending-projection annotations are runtime Secret
		// references exactly like the spec fields (runtimeEnvSecret projects
		// pending-env-secret into envFrom; secretFileMounts projects
		// pending-files-secret into the /etc/secrets volume) — the validator
		// must refuse them too.
		"pending-env annotation": func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.PendingEnvSecretAnnotation: "bex-tenant-postgres"}
		},
		"pending-files annotation": func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.PendingFilesSecretAnnotation: "bex-tenant-postgres"}
		},
		// codex-security 2026-08 F7: the two remaining tenant-settable
		// Secret-name fields. copyCloneSecret resolves spec.cloneSecret by name
		// and relocates the Secret into the shared build namespace (stripping its
		// labels on the old code); the kubelet resolves
		// spec.externalRegistryPullSecret by name. Both belong in the same
		// denylist as the mount fields.
		"cloneSecret": func(a *appv1alpha1.App) { a.Spec.CloneSecret = "bex-tenant-postgres" },
		"externalRegistryPullSecret": func(a *appv1alpha1.App) {
			a.Spec.ExternalRegistryPullSecret = "bex-tenant-postgres"
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

// TestRejectConfiguredOperationalSecretNames is the round-11 #1 guard for the
// out-of-band operational Secrets: the shared registry pull/push credentials,
// build-namespace pull credential, tenant signing key, and static-site publish
// credential are created by scripts, so nothing stamps the protected label on
// them. The operator knows their configured names and refuses them by name —
// no Secret lookup needed, so the denial holds even before the Secret exists.
func TestRejectConfiguredOperationalSecretNames(t *testing.T) {
	scheme := protectedSecretScheme(t)
	buildClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &AppReconciler{
		Client:              buildClient,
		BuildClient:         buildClient,
		RegistryPullSecret:  "zot-shared-pull",
		RegistryPushSecret:  "bex-registry-push",
		TenantSignKeySecret: "bex-tenant-sign",
		StaticStore:         publish.Store{Secret: "bex-static-publish-s3"},
	}
	app := func(field func(*appv1alpha1.App)) *appv1alpha1.App {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web"}}
		field(a)
		return a
	}
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"shared pull secret": func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "zot-shared-pull" },
		"push secret":        func(a *appv1alpha1.App) { a.Spec.EnvFromSecrets = []string{"bex-registry-push"} },
		"tenant signing key": func(a *appv1alpha1.App) {
			a.Spec.FilesFromSecrets = []string{"bex-tenant-sign"}
		},
		// Round-18: in a co-located install (BEX_BUILD_NAMESPACE unset) the
		// static publisher credential sits in the App's own namespace, so a
		// tenant App could mount it for bucket-wide read/write/delete across
		// every tenant's static content.
		"static publish credential": func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "AWS_SECRET_ACCESS_KEY", ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "bex-static-publish-s3", Key: "AWS_SECRET_ACCESS_KEY"}}}}
		},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if err := r.rejectProtectedSecretRefs(context.Background(), app(mutate)); err == nil {
				t.Fatalf("%s reference was ACCEPTED — operational credential exfil", name)
			}
		})
	}

	// Round-18 sweep: the static publish credential must be refused through
	// EVERY projection field the validator covers, not just one.
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"envFromSecret":  func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bex-static-publish-s3" },
		"envFromSecrets": func(a *appv1alpha1.App) { a.Spec.EnvFromSecrets = []string{"bex-static-publish-s3"} },
		"filesFromSecrets": func(a *appv1alpha1.App) {
			a.Spec.FilesFromSecrets = []string{"bex-static-publish-s3"}
		},
		"env secretKeyRef": func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "X", ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "bex-static-publish-s3", Key: "AWS_SECRET_ACCESS_KEY"}}}}
		},
		"pending-env annotation": func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.PendingEnvSecretAnnotation: "bex-static-publish-s3"}
		},
		"pending-files annotation": func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.PendingFilesSecretAnnotation: "bex-static-publish-s3"}
		},
	} {
		t.Run("rejects static publish credential via "+name, func(t *testing.T) {
			if err := r.rejectProtectedSecretRefs(context.Background(), app(mutate)); err == nil {
				t.Fatalf("static publish credential via %s was ACCEPTED — cross-tenant static-content exfil", name)
			}
		})
	}
}
