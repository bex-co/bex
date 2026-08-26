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

// TestAllowsOwnBuildPlaneSecretRefs covers the w6/m97 carve-out: bex-api mints
// an App's clone token and external-registry pull credential under names
// derived from that App's own name, stamps the protected label on them so no
// OTHER App can mount them, and writes those names into spec.cloneSecret /
// spec.externalRegistryPullSecret itself — so the guard refusing them failed
// every GitHub-connected deploy on the platform before any build ran. The
// carve-out is exact self-name equality on exactly those two fields; every
// sibling case in this test proves it is no wider than that.
func TestAllowsOwnBuildPlaneSecretRefs(t *testing.T) {
	scheme := protectedSecretScheme(t)
	protectedSecret := func(name string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "tea-a",
			Labels: map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount},
		}}
	}
	webClone := appv1alpha1.CloneSecretName("web")               // "web-clone"
	webPull := appv1alpha1.ExternalRegistryPullSecretName("web") // "web-registry-pull"
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		protectedSecret(webClone),
		protectedSecret(webPull),
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "tea-a"}},
	).Build()
	r := &AppReconciler{Client: buildClient, BuildClient: buildClient}
	app := func(name string, mutate func(*appv1alpha1.App)) *appv1alpha1.App {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tea-a"}}
		mutate(a)
		return a
	}

	for name, mutate := range map[string]func(*appv1alpha1.App){
		"own clone secret": func(a *appv1alpha1.App) { a.Spec.CloneSecret = webClone },
		"own registry-pull secret": func(a *appv1alpha1.App) {
			a.Spec.ExternalRegistryPullSecret = webPull
		},
		"both own build-plane secrets": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webClone
			a.Spec.ExternalRegistryPullSecret = webPull
		},
		// The two carve-out fields coexisting with a legitimate ordinary
		// reference: the rest of the set is still validated as usual.
		"own secrets alongside an ordinary env secret": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webClone
			a.Spec.ExternalRegistryPullSecret = webPull
			a.Spec.EnvFromSecret = "web-env"
		},
	} {
		t.Run("accepts "+name, func(t *testing.T) {
			if err := r.rejectProtectedSecretRefs(context.Background(), app("web", mutate)); err != nil {
				t.Fatalf("App's own build-plane Secret was REFUSED — every connected deploy fails: %v", err)
			}
		})
	}

	// The carve-out is self-name equality, not a "<x>-clone looks fine" suffix
	// rule: these names match the convention exactly, but belong to App "web",
	// and App "other" naming them is still exfiltration.
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"another App's clone secret via cloneSecret": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webClone
		},
		"another App's pull secret via externalRegistryPullSecret": func(a *appv1alpha1.App) {
			a.Spec.ExternalRegistryPullSecret = webPull
		},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			if err := r.rejectProtectedSecretRefs(context.Background(), app("other", mutate)); err == nil {
				t.Fatalf("%s was ACCEPTED — clone-token/registry-credential exfil across Apps", name)
			}
		})
	}

	// Each field is exempt only for ITS OWN derived name: naming the clone
	// Secret through externalRegistryPullSecret (or vice versa) is not a
	// self-reference, and must not be laundered into acceptance by the other
	// field legitimately naming the same Secret. Both declaration orders,
	// because exemption is a property of the reference, not of the name.
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"clone secret named through externalRegistryPullSecret": func(a *appv1alpha1.App) {
			a.Spec.ExternalRegistryPullSecret = webClone
		},
		"pull secret named through cloneSecret": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webPull
		},
		"clone secret named through BOTH fields": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webClone
			a.Spec.ExternalRegistryPullSecret = webClone
		},
		"pull secret named through BOTH fields": func(a *appv1alpha1.App) {
			a.Spec.CloneSecret = webPull
			a.Spec.ExternalRegistryPullSecret = webPull
		},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			if err := r.rejectProtectedSecretRefs(context.Background(), app("web", mutate)); err == nil {
				t.Fatalf("%s was ACCEPTED — the wrong field's self-name is not a self-reference", name)
			}
		})
	}

	// The mount fields keep their full strictness against the App's OWN build
	// -plane Secrets: spec.cloneSecret is read by the operator's copier at
	// build time, but envFrom/files/secretKeyRef/pending would hand the same
	// GitHub installation token and registry password to tenant code at
	// runtime. Every field the validator covers, both self-names.
	for _, self := range []string{webClone, webPull} {
		for field, mutate := range map[string]func(*appv1alpha1.App, string){
			"envFromSecret":  func(a *appv1alpha1.App, n string) { a.Spec.EnvFromSecret = n },
			"envFromSecrets": func(a *appv1alpha1.App, n string) { a.Spec.EnvFromSecrets = []string{n} },
			"filesFromSecrets": func(a *appv1alpha1.App, n string) {
				a.Spec.FilesFromSecrets = []string{n}
			},
			"env secretKeyRef": func(a *appv1alpha1.App, n string) {
				a.Spec.Env = []appv1alpha1.EnvVar{{Name: "TOKEN", ValueFrom: &appv1alpha1.EnvVarSource{
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: n, Key: "token"}}}}
			},
			"pending-env annotation": func(a *appv1alpha1.App, n string) {
				a.Annotations = map[string]string{appv1alpha1.PendingEnvSecretAnnotation: n}
			},
			"pending-files annotation": func(a *appv1alpha1.App, n string) {
				a.Annotations = map[string]string{appv1alpha1.PendingFilesSecretAnnotation: n}
			},
			// The build-plane field AND a mount field naming the same
			// self-secret: the mount reference must still sink it, or the
			// carve-out becomes a laundering channel for the token.
			"cloneSecret + envFromSecret": func(a *appv1alpha1.App, n string) {
				a.Spec.CloneSecret = n
				a.Spec.EnvFromSecret = n
			},
			"externalRegistryPullSecret + filesFromSecrets": func(a *appv1alpha1.App, n string) {
				a.Spec.ExternalRegistryPullSecret = n
				a.Spec.FilesFromSecrets = []string{n}
			},
		} {
			t.Run("refuses own "+self+" mounted via "+field, func(t *testing.T) {
				a := app("web", func(a *appv1alpha1.App) { mutate(a, self) })
				if err := r.rejectProtectedSecretRefs(context.Background(), a); err == nil {
					t.Fatalf("App's own %s via %s was ACCEPTED — tenant code would read the credential at runtime", self, field)
				}
			})
		}
	}

	// The out-of-band operational denylist runs over the full reference set,
	// carve-out included: a configured operational Secret that happens to
	// collide with an App's self-name is still refused by name.
	t.Run("refuses a configured operational Secret colliding with the self-name", func(t *testing.T) {
		rc := &AppReconciler{Client: buildClient, BuildClient: buildClient, RegistryPushSecret: webClone}
		a := app("web", func(a *appv1alpha1.App) { a.Spec.CloneSecret = webClone })
		if err := rc.rejectProtectedSecretRefs(context.Background(), a); err == nil {
			t.Fatal("configured operational Secret named via cloneSecret was ACCEPTED")
		}
	})
}
