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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/245: a public database's PgBouncer rolls when the certificate it serves
// changes — enabling the pooler re-issues it with the pooler's name, and a
// renewal replaces it — and only then.
func TestPoolerRollsWhenItsServedCertificateChanges(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-a", Namespace: "tea-a", UID: "db-uid"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "1c-2g", Pooler: true},
	}
	tls := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: databasePublicTLSName(db), Namespace: db.Namespace},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: []byte("cert-issued-at-creation"), corev1.TLSPrivateKeyKey: []byte("k")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, tls).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	cluster := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	served := func() string {
		t.Helper()
		if _, err := r.reconcilePooler(ctx, db, cluster); err != nil {
			t.Fatal(err)
		}
		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(cnpgPoolerGVK)
		if err := cl.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: db.Name + "-pooler"}, live); err != nil {
			t.Fatal(err)
		}
		got, _, _ := unstructured.NestedString(live.Object, "spec", "template", "metadata", "annotations", poolerCertificateAnnotation)
		return got
	}

	// A private database's Cluster serves CNPG's own certificate: nothing to stamp.
	if got := served(); got != "" {
		t.Fatalf("private database's Pooler carries %q, want no certificate annotation", got)
	}

	_ = unstructured.SetNestedField(cluster.Object, databasePublicTLSName(db), "spec", "certificates", "serverTLSSecret")
	first := served()
	if first == "" {
		t.Fatal("a public database's Pooler carries no certificate digest")
	}
	if again := served(); again != first {
		t.Fatalf("an unrelated reconcile changed the digest %q → %q (it would roll PgBouncer)", first, again)
	}

	// cert-manager re-issues with the pooler's name (or renews).
	tls.Data[corev1.TLSCertKey] = []byte("cert-reissued-with-the-pool-name")
	if err := cl.Update(ctx, tls); err != nil {
		t.Fatal(err)
	}
	if reissued := served(); reissued == "" || reissued == first {
		t.Fatalf("after a re-issue the digest is %q (was %q), want a new one so PgBouncer rolls", reissued, first)
	}
}
