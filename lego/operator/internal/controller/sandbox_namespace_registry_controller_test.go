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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

func snapshotRegistryTestEnv(t *testing.T, objs ...client.Object) (*SandboxNamespaceRegistryReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	objs = append(objs, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{"htpasswd": []byte("bex-builder:$2a$10$somehash\n")},
	})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &SandboxNamespaceRegistryReconciler{
		Client: c,
		Scheme: scheme,
		Registry: &registry.Creds{
			Client:       c,
			ZotNamespace: "bex-registry",
			HTPasswdName: "zot-htpasswd",
			ConfigName:   "zot-config",
			Registry:     "zot.bex-registry.svc:5000",
		},
	}, c
}

// TestSandboxNamespaceRegistryProvisionsAndRevokes drives the reconciler
// through the namespace lifecycle: provision on reconcile, revoke once the
// namespace is gone.
func TestSandboxNamespaceRegistryProvisionsAndRevokes(t *testing.T) {
	ctx := context.Background()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "tea-x-sandbox",
		Labels: map[string]string{"app.bex.co/regime": "sandbox"},
	}}
	r, c := snapshotRegistryTestEnv(t, ns)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "tea-x-sandbox"}}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var pullSec corev1.Secret
	if err := c.Get(ctx, client.ObjectKey{Namespace: "tea-x-sandbox", Name: registry.SnapshotPullSecretName}, &pullSec); err != nil {
		t.Fatalf("pull secret not provisioned: %v", err)
	}

	// Namespace deleted → the label-filtered watch delivers the event and the
	// reconciler revokes the Zot identity.
	if err := c.Delete(ctx, ns); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile after delete: %v", err)
	}
	var ht corev1.Secret
	if err := c.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-htpasswd"}, &ht); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ht.Data["htpasswd"]), "snap-tea-x-sandbox:") {
		t.Fatal("htpasswd entry should be revoked after namespace deletion")
	}
}
