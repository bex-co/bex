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

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPredeployPullSecrets pins the w9/012 pre-deploy sibling of the publish
// Job's pull-secret fix: the pre-deploy Job runs in the build namespace, where
// the App-namespace tenant pull secrets are unreachable. Same namespace ⇒
// byte-identical to imagePullSecrets(); separate namespace ⇒ the build-ns
// credential for platform-registry images, and a foreign image's external
// pull secret is relocated beside the Job.
func TestPredeployPullSecrets(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	names := func(refs []corev1.LocalObjectReference) []string {
		out := make([]string, 0, len(refs))
		for _, r := range refs {
			out = append(out, r.Name)
		}
		return out
	}

	t.Run("same namespace is byte-identical to imagePullSecrets", func(t *testing.T) {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "default"}}
		r := &AppReconciler{Registry: "zot.svc:5000", PerAppRegistry: &registry.Creds{}}
		got, err := r.predeployPullSecrets(ctx, app, "zot.svc:5000/myapp:gen-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != registry.PullSecretName("myapp") {
			t.Errorf("same-ns secrets = %v, want [reg-pull-myapp]", names(got))
		}
	})

	t.Run("separate namespace uses the build-ns credential", func(t *testing.T) {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "default"}}
		r := &AppReconciler{
			Registry: "zot.svc:5000", BuildNamespace: "bex-system",
			PerAppRegistry: &registry.Creds{}, RegistryBuildPullSecret: "bex-registry-pull",
		}
		got, err := r.predeployPullSecrets(ctx, app, "zot.svc:5000/myapp:gen-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "bex-registry-pull" {
			t.Errorf("separate-ns secrets = %v, want [bex-registry-pull]", names(got))
		}
	})

	t.Run("separate namespace relocates a foreign image's external pull secret", func(t *testing.T) {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "default"}}
		app.Spec.ExternalRegistryPullSecret = "ext-pull"
		src := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ext-pull", Namespace: "default"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(src).Build()
		r := &AppReconciler{
			Client: cl, Registry: "zot.svc:5000", BuildNamespace: "bex-system",
		}
		got, err := r.predeployPullSecrets(ctx, app, "ghcr.io/acme/tool:v1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "ext-pull" {
			t.Fatalf("foreign-image secrets = %v, want [ext-pull]", names(got))
		}
		var copied corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-system", Name: "ext-pull"}, &copied); err != nil {
			t.Fatalf("external pull secret not relocated to the build namespace: %v", err)
		}
		if copied.Type != corev1.SecretTypeDockerConfigJson {
			t.Errorf("relocated secret type = %v, want kubernetes.io/dockerconfigjson", copied.Type)
		}
	})
}

// TestMirrorPreDeploySecrets pins the env/secret-file mirroring half of the
// w9/012 pre-deploy fix: with a separate build namespace, the Secrets the
// pre-deploy pod references (spec.envFromSecret[s], spec.filesFromSecrets)
// are copied beside the Job; a Secret absent from the App namespace is
// skipped, not an error; same-namespace is a no-op.
func TestMirrorPreDeploySecrets(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "default"}}
	app.Spec.EnvFromSecret = "myapp-env"
	app.Spec.EnvFromSecrets = []string{"extra-env", "absent-env"}
	app.Spec.FilesFromSecrets = []string{"certs"}

	objs := []client.Object{
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "myapp-env", Namespace: "default"},
			Data: map[string][]byte{"DATABASE_URL": []byte("postgres://…")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extra-env", Namespace: "default"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "certs", Namespace: "default"}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	r := &AppReconciler{Client: cl, BuildNamespace: "bex-system"}

	if err := r.mirrorPreDeploySecrets(ctx, app, "bex-system"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"myapp-env", "extra-env", "certs"} {
		var s corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-system", Name: name}, &s); err != nil {
			t.Errorf("secret %s not mirrored into the build namespace: %v", name, err)
		}
	}
	var s corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-system", Name: "absent-env"}, &s); err == nil {
		t.Error("absent-env should not have been conjured in the build namespace")
	}

	// Same namespace: nothing to do, no error even with no client access.
	if err := (&AppReconciler{}).mirrorPreDeploySecrets(ctx, app, "default"); err != nil {
		t.Errorf("same-namespace mirror = %v, want nil", err)
	}
}
