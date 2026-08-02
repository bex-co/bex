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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TestCopyCloneSecretRefusesForeignDestination is the codex-security #14
// regression: a tenant-controlled spec field names the build-namespace
// destination, so a same-named Secret owned by another App (or the platform) may
// already occupy it. The mirror must refuse to overwrite a foreign owner rather
// than clobber its type/data/labels under the operator's CRUD authority.
func TestCopyCloneSecretRefusesForeignDestination(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-clone", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"token": []byte("attacker-bytes")},
	}
	// A foreign-owned destination already occupies the name in the build ns.
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-clone", Namespace: "bex-system", UID: "uid-victim",
			Labels: map[string]string{"app.bex.co/app": "victim", "app.bex.co/app-uid": "uid-victim"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"token": []byte("victim-secret")},
	}
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(src, foreign).Build()
	r := &AppReconciler{
		Client:      fake.NewClientBuilder().WithScheme(scheme).Build(),
		BuildClient: buildClient,
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "uid-web"}}

	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "web-clone"); err == nil {
		t.Fatalf("copyCloneSecret must refuse to overwrite a foreign-owned destination, got nil error")
	}
	// The victim's bytes AND labels must be byte-for-byte unchanged.
	var dst corev1.Secret
	if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "web-clone"}, &dst); err != nil {
		t.Fatalf("foreign destination vanished: %v", err)
	}
	if string(dst.Data["token"]) != "victim-secret" {
		t.Fatalf("foreign destination data was clobbered: token = %q, want victim-secret", dst.Data["token"])
	}
	if dst.Labels["app.bex.co/app"] != "victim" || dst.Labels["app.bex.co/app-uid"] != "uid-victim" {
		t.Fatalf("foreign destination labels were rewritten: %v", dst.Labels)
	}
}

// TestDeleteOwnedSecretLeavesForeignSecretIntact is the codex-security #5
// regression: the finalizer used to delete every App-named build-namespace Secret
// by name. deleteOwnedSecret now deletes only a Secret this App owns (the
// artifactLabels UID); a same-named foreign/platform Secret survives, while the
// App's own Secret is deleted and a missing one is a clean no-op.
func TestDeleteOwnedSecretLeavesForeignSecretIntact(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-clone", Namespace: "bex-build", UID: "uid-victim",
			Labels: map[string]string{"app.bex.co/app": "victim", "app.bex.co/app-uid": "uid-victim"},
		},
	}
	owned := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-env", Namespace: "bex-build", UID: "uid-web",
			Labels: map[string]string{"app.bex.co/app": "web", "app.bex.co/app-uid": "uid-web"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign, owned).Build()
	web := execution.ArtifactIdentity{Name: "web", UID: "uid-web", Namespace: "default"}
	ctx := context.Background()

	// A foreign secret is left intact (done=true — nothing to wait on).
	if done, err := deleteOwnedSecret(ctx, cl, "bex-build", "web-clone", web); err != nil || !done {
		t.Fatalf("deleteOwnedSecret(foreign) = done=%v err=%v; want done=true, nil", done, err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-build", Name: "web-clone"}, &corev1.Secret{}); err != nil {
		t.Fatalf("foreign secret should survive the finalizer, got %v", err)
	}
	// An owned secret is deleted (done=false — delete issued).
	if done, err := deleteOwnedSecret(ctx, cl, "bex-build", "web-env", web); err != nil || done {
		t.Fatalf("deleteOwnedSecret(own) = done=%v err=%v; want done=false, nil", done, err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-build", Name: "web-env"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("own secret should be deleted, got %v", err)
	}
	// A missing secret is a clean no-op (done=true).
	if done, err := deleteOwnedSecret(ctx, cl, "bex-build", "never-existed", web); err != nil || !done {
		t.Fatalf("deleteOwnedSecret(missing) = done=%v err=%v; want done=true, nil", done, err)
	}
}
