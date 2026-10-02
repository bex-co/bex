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
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKeyValueTLSObligationPrecedesIssuance(t *testing.T) {
	for _, failStamp := range []bool{false, true} {
		t.Run(map[bool]string{false: "issued", true: "stamp-fails"}[failStamp], func(t *testing.T) {
			scheme := keyValueBackupTestScheme(t)
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-public", Namespace: "default", UID: "kv-lifetime"}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", Public: true}}
			issued := false
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).WithStatusSubresource(kv).WithInterceptorFuncs(interceptor.Funcs{
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*appv1alpha1.KeyValue); ok && failStamp {
						return errors.New("finalizer update unavailable")
					}
					return c.Update(ctx, obj, opts...)
				},
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if obj.GetObjectKind().GroupVersionKind() == certManagerCertificateGVK {
						var persisted appv1alpha1.KeyValue
						if err := c.Get(ctx, client.ObjectKeyFromObject(kv), &persisted); err != nil {
							t.Fatal(err)
						}
						if !controllerutil.ContainsFinalizer(&persisted, kvTLSFinalizer) || persisted.Annotations[annotKVTLSIdentity] == "" {
							t.Fatal("TLS issuance preceded durable cleanup obligation")
						}
						issued = true
						// Stop here: workload reconciliation is covered independently.
						return errors.New("stop after observing issuance")
					}
					return c.Create(ctx, obj, opts...)
				},
			}).Build()
			r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "issuer"}
			_, _ = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(kv)})
			if issued == failStamp {
				t.Fatalf("Certificate issued=%v with failed stamp=%v", issued, failStamp)
			}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(kv), kv); err != nil {
				t.Fatal(err)
			}
			if controllerutil.ContainsFinalizer(kv, kvFinalizer) {
				t.Fatal("Free TLS preparation added backup purge obligation")
			}
		})
	}
}

func TestKeyValueTLSProvenanceAndHistoricalPublicObligation(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-history", Namespace: "default", UID: "kv-lifetime"}, Spec: appv1alpha1.KeyValueSpec{Public: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "issuer"}
	if err := r.ensureKeyValueTLSFinalizer(ctx, kv); err != nil {
		t.Fatal(err)
	}
	version := kv.ResourceVersion
	if err := r.ensureKeyValueTLSFinalizer(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if kv.ResourceVersion != version {
		t.Fatal("unchanged TLS obligation rewrote the KeyValue")
	}
	if _, err := r.reconcileKeyValueTLS(ctx, kv, true, keyValueTLSName(kv)); err != nil {
		t.Fatal(err)
	}
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: keyValueTLSName(kv)}, cert); err != nil {
		t.Fatal(err)
	}
	uid, _, _ := unstructured.NestedString(cert.Object, "spec", "secretTemplate", "labels", execution.LabelKeyValueUID)
	if uid != string(kv.UID) || !metav1.IsControlledBy(cert, kv) {
		t.Fatal("issued Certificate omitted lifetime provenance")
	}
	cert.SetUID("cert-lifetime")
	if err := cl.Update(ctx, cert); err != nil {
		t.Fatal(err)
	}
	history := kv.Annotations[annotKVTLSIdentity]
	kv.Spec.Public = false
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatal(err)
	}
	r.KvDomain = ""
	r.ClusterIssuer = ""
	if err := r.ensureKeyValueTLSFinalizer(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(kv, kvTLSFinalizer) || kv.Annotations[annotKVTLSIdentity] != history {
		t.Fatal("public withdrawal lost cleanup identity")
	}
	if _, err := r.reconcileKeyValueTLS(ctx, kv, false, keyValueTLSName(kv)); err != nil {
		t.Fatal(err)
	}
	identity, err := r.keyValueTLSIdentity(kv)
	if err != nil || identity.CertificateUID != "cert-lifetime" {
		t.Fatalf("withdrawal lost Certificate lifetime: %+v %v", identity, err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(cert), cert); !apierrors.IsNotFound(err) {
		t.Fatalf("owned Certificate remained: %v", err)
	}
}
