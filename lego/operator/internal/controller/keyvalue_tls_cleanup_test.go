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
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestKeyValueTLSDeletionLifetime(t *testing.T) {
	for _, test := range []struct {
		name                                      string
		legacy, foreign, old, private, backupOnly bool
	}{
		{name: "legacy backup finalizer only", legacy: true, backupOnly: true},
		{name: "issued ownerless"}, {name: "legacy ownerless", legacy: true},
		{name: "formerly public", legacy: true, private: true},
		{name: "foreign lifetime", foreign: true}, {name: "legacy previous lifetime", legacy: true, old: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := keyValueBackupTestScheme(t)
			created := metav1.NewTime(time.Now().Add(-time.Hour))
			deleted := metav1.NewTime(time.Now())
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-tls-lifecycle", Namespace: defaultAppsNamespace, UID: "current-kv", CreationTimestamp: created, DeletionTimestamp: &deleted, Finalizers: []string{"app.bex.co/kv-tls-cleanup"}}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", Public: !test.private}}
			if test.backupOnly {
				kv.Finalizers = []string{kvFinalizer}
				kv.Spec.Plan = "starter"
				kv.Annotations = map[string]string{appv1alpha1.AnnotationPreserveKeyValueBackups: kvPreserveBackupsTrue}
			}
			name := kv.Name + "-kv-tls"
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: kv.Namespace, UID: "issued-secret", CreationTimestamp: metav1.NewTime(created.Add(time.Minute)), Labels: map[string]string{execution.LabelKeyValueUID: string(kv.UID)}}, Type: corev1.SecretTypeTLS}
			if test.legacy {
				secret.Labels = nil
				secret.Annotations = map[string]string{"cert-manager.io/certificate-name": name, "cert-manager.io/alt-names": kv.Name + ".kv.example.test", "cert-manager.io/issuer-name": "issuer", "cert-manager.io/issuer-kind": "ClusterIssuer"}
			}
			if test.old {
				secret.CreationTimestamp = metav1.NewTime(created.Add(-time.Minute))
			}
			if test.foreign {
				secret.Labels[execution.LabelKeyValueUID] = "previous-kv"
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv).WithObjects(kv).Build()
			// Secrets are deliberately absent from the cached client, just as tenant Secrets are in production.
			secrets := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
			request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kv)}
			for range 5 {
				r := &KeyValueReconciler{Client: cl, SecretClient: secrets, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "issuer", Backup: testKeyValueBackupStore}
				_, err := r.Reconcile(ctx, request)
				if test.foreign || test.old {
					if err == nil {
						t.Fatal("unproven Secret cleanup must fail closed")
					}
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			err := secrets.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})
			if test.foreign || test.old {
				if err != nil {
					t.Fatalf("foreign Secret lost: %v", err)
				}
				current := &appv1alpha1.KeyValue{}
				if err := cl.Get(ctx, request.NamespacedName, current); err != nil || len(current.Finalizers) == 0 {
					t.Fatalf("cleanup obligation released: %v", err)
				}
			} else {
				if !apierrors.IsNotFound(err) {
					t.Fatalf("issued TLS Secret remains after KeyValue deletion: %v", err)
				}
				if err := cl.Get(ctx, request.NamespacedName, &appv1alpha1.KeyValue{}); !apierrors.IsNotFound(err) {
					t.Fatalf("finalizer not released after observed absence: %v", err)
				}
			}
			jobs := &batchv1.JobList{}
			if err := cl.List(ctx, jobs); err != nil || len(jobs.Items) != 0 {
				t.Fatalf("Free TLS deletion created backup purge Jobs: %v, %d", err, len(jobs.Items))
			}
		})
	}
}

func TestKeyValueTLSDeletionWaitsForCertificate(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	deleted := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-ordered", Namespace: defaultAppsNamespace, UID: "kv-current", DeletionTimestamp: &deleted, Finalizers: []string{"app.bex.co/kv-tls-cleanup"}}}
	cert := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"secretName": kv.Name + "-kv-tls", "dnsNames": []any{kv.Name + ".kv.example.test"}, "issuerRef": map[string]any{"name": "issuer", "kind": "ClusterIssuer"}}}}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	cert.SetName(kv.Name + "-kv-tls")
	cert.SetNamespace(kv.Namespace)
	cert.SetUID("certificate-current")
	cert.SetFinalizers([]string{"test.cert-manager/issuance"})
	cert.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(kv, appv1alpha1.SchemeGroupVersion.WithKind("KeyValue"))})
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cert.GetName(), Namespace: kv.Namespace, UID: "secret-current", Labels: map[string]string{execution.LabelKeyValueUID: string(kv.UID)}}, Type: corev1.SecretTypeTLS}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv).WithObjects(kv, cert).Build()
	secrets := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}}
	run := func() {
		t.Helper()
		r := &KeyValueReconciler{Client: cl, APIReader: cl, SecretClient: secrets, Scheme: scheme}
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	for range 4 {
		run()
		if err := secrets.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); err != nil {
			t.Fatalf("Secret removed while Certificate can still issue: %v", err)
		}
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		t.Fatal(err)
	}
	if cert.GetDeletionTimestamp().IsZero() {
		t.Fatal("Certificate was never quiesced")
	}
	cert.SetFinalizers(nil)
	if err := cl.Update(ctx, cert); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		run()
	}
	if err := secrets.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Secret remains after Certificate absence: %v", err)
	}
	if err := cl.Get(ctx, req.NamespacedName, &appv1alpha1.KeyValue{}); !apierrors.IsNotFound(err) {
		t.Fatalf("KeyValue still finalizing: %v", err)
	}
}

func TestKeyValueTLSProducerRejectsExistingForeignSecret(t *testing.T) {
	for _, variant := range []string{"different UID", "legacy older lifetime", "foreign owner"} {
		t.Run(variant, func(t *testing.T) {
			ctx := context.Background()
			scheme := keyValueBackupTestScheme(t)
			created := metav1.NewTime(time.Now().Add(-time.Hour))
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-producer", Namespace: defaultAppsNamespace, UID: "current", CreationTimestamp: created}}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-kv-tls", Namespace: kv.Namespace, UID: "foreign-secret", CreationTimestamp: metav1.NewTime(created.Add(time.Minute)), Labels: map[string]string{execution.LabelKeyValueUID: string(kv.UID)}}, Type: corev1.SecretTypeTLS}
			switch variant {
			case "different UID":
				secret.Labels[execution.LabelKeyValueUID] = "previous"
			case "legacy older lifetime":
				secret.Labels = nil
				secret.CreationTimestamp = metav1.NewTime(created.Add(-time.Minute))
				secret.Annotations = map[string]string{"cert-manager.io/certificate-name": secret.Name, "cert-manager.io/alt-names": kv.Name + ".kv.example.test", "cert-manager.io/issuer-name": "issuer", "cert-manager.io/issuer-kind": "ClusterIssuer"}
			case "foreign owner":
				secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Secret", Name: "foreign", UID: "foreign-owner"}}
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
			secrets := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
			r := &KeyValueReconciler{Client: cl, SecretClient: secrets, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "issuer"}
			if _, err := r.reconcileKeyValueTLS(ctx, kv, true, secret.Name); err == nil {
				t.Fatal("issuance allowed relabeling a foreign TLS Secret as current lifetime")
			}
			cert := &unstructured.Unstructured{}
			cert.SetGroupVersionKind(certManagerCertificateGVK)
			if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), cert); !apierrors.IsNotFound(err) {
				t.Fatalf("Certificate able to claim foreign Secret was published: %v", err)
			}
		})
	}
}

type keyValueTLSForbiddenDeleteClient struct {
	client.Client
	deny bool
}

func (c *keyValueTLSForbiddenDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.deny {
		return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, obj.GetName(), nil)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestKeyValueTLSDeletionRetriesForbiddenSecret(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	deleted := metav1.NewTime(time.Now().Add(-time.Hour))
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-forbidden", Namespace: defaultAppsNamespace, UID: "current", DeletionTimestamp: &deleted, Finalizers: []string{"app.bex.co/kv-tls-cleanup"}}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-kv-tls", Namespace: kv.Namespace, UID: "issued", Labels: map[string]string{execution.LabelKeyValueUID: string(kv.UID)}}, Type: corev1.SecretTypeTLS}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv).WithObjects(kv).Build()
	secrets := &keyValueTLSForbiddenDeleteClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build(), deny: true}
	req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kv)}
	r := &KeyValueReconciler{Client: cl, SecretClient: secrets, Scheme: scheme, FinalizerOverrunAfter: time.Second}
	if _, err := r.Reconcile(ctx, req); !apierrors.IsForbidden(err) {
		t.Fatalf("want retriable Forbidden, got %v", err)
	}
	current := &appv1alpha1.KeyValue{}
	if err := cl.Get(ctx, req.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Finalizers) != 1 {
		t.Fatal("Forbidden lost cleanup obligation")
	}
	stalled := false
	for _, condition := range current.Status.Conditions {
		if condition.Type == conditionDeletionStalled && condition.Status == metav1.ConditionTrue {
			stalled = true
		}
	}
	if !stalled {
		t.Fatal("overdue Forbidden not surfaced as DeletionStalled")
	}
	secrets.deny = false
	for range 3 {
		r = &KeyValueReconciler{Client: cl, SecretClient: secrets, Scheme: scheme}
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if err := secrets.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("retry left Secret: %v", err)
	}
	if err := cl.Get(ctx, req.NamespacedName, current); !apierrors.IsNotFound(err) {
		t.Fatalf("retry failed to release lifetime: %v", err)
	}
}

func keyValueTLSDeletionScenario(t *testing.T, scenario string) (*KeyValueReconciler, *appv1alpha1.KeyValue, *corev1.Secret, *bool) {
	t.Helper()
	scheme := keyValueBackupTestScheme(t)
	deleted := metav1.Now()
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "kv-matrix", Namespace: defaultAppsNamespace, UID: "current", DeletionTimestamp: &deleted, Finalizers: []string{kvTLSFinalizer}}}
	if scenario == "noncanonical namespace" {
		kv.Namespace = "foreign-workspace"
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: keyValueTLSName(kv), Namespace: kv.Namespace, UID: "original", Labels: map[string]string{execution.LabelKeyValueUID: string(kv.UID)}}, Type: corev1.SecretTypeTLS}
	var objects, secretObjects []client.Object
	objects = append(objects, kv)
	if scenario != "never issued private" {
		secretObjects = append(secretObjects, secret)
	}
	if scenario == "namespace terminating" {
		kv.Finalizers = append(kv.Finalizers, kvFinalizer)
		secretObjects = append(secretObjects, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: kv.Namespace, DeletionTimestamp: &deleted, Finalizers: []string{"test/namespace-controller"}}})
	}
	if scenario == "foreign Certificate" {
		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(certManagerCertificateGVK)
		cert.SetName(secret.Name)
		cert.SetNamespace(kv.Namespace)
		cert.SetUID("foreign-cert")
		cert.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, UID: "previous"}}, appv1alpha1.SchemeGroupVersion.WithKind("KeyValue"))})
		objects = append(objects, cert)
	}
	if scenario == "delayed Secret deletion" {
		secret.Finalizers = []string{"test/secret-retention"}
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv).WithObjects(objects...).Build()
	interleaved := new(bool)
	secrets := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secretObjects...).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok && scenario == "Secret read forbidden" {
				return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, key.Name, nil)
			}
			return c.Get(ctx, key, obj, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if !*interleaved && (scenario == "replacement UID" || scenario == "replacement resourceVersion") {
				*interleaved = true
				current := &corev1.Secret{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
					return err
				}
				if scenario == "replacement UID" {
					if err := c.Delete(ctx, current); err != nil {
						return err
					}
					current.UID = "replacement"
					current.ResourceVersion = ""
					current.Labels[execution.LabelKeyValueUID] = "next-lifetime"
					if err := c.Create(ctx, current); err != nil {
						return err
					}
				} else {
					current.Labels[execution.LabelKeyValueUID] = "next-lifetime"
					if err := c.Update(ctx, current); err != nil {
						return err
					}
				}
			}
			return c.Delete(ctx, obj, opts...)
		},
	}).Build()
	r := &KeyValueReconciler{Client: cl, SecretClient: secrets, Scheme: scheme, Backup: testKeyValueBackupStore}
	return r, kv, secret, interleaved
}

func TestKeyValueTLSDeletionRemainingMatrix(t *testing.T) {
	for _, scenario := range []string{"never issued private", "namespace terminating", "noncanonical namespace", "foreign Certificate", "Secret read forbidden", "replacement UID", "replacement resourceVersion", "delayed Secret deletion"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			r, kv, secret, interleaved := keyValueTLSDeletionScenario(t, scenario)
			cl, secrets := r.Client, r.SecretClient
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kv)}
			_, err := r.Reconcile(ctx, req)
			wantError := scenario == "noncanonical namespace" || scenario == "foreign Certificate" || scenario == "Secret read forbidden" || scenario == "replacement UID" || scenario == "replacement resourceVersion"
			if (err != nil) != wantError {
				t.Fatalf("reconcile error=%v, wantError=%v", err, wantError)
			}
			current := &appv1alpha1.KeyValue{}
			getErr := cl.Get(ctx, req.NamespacedName, current)
			if scenario == "never issued private" {
				if !apierrors.IsNotFound(getErr) {
					t.Fatalf("empty private lifetime retained: %v", getErr)
				}
				return
			}
			if getErr != nil {
				t.Fatal(getErr)
			}
			if scenario == "namespace terminating" {
				if slices.Contains(current.Finalizers, kvTLSFinalizer) || !slices.Contains(current.Finalizers, kvFinalizer) {
					t.Fatalf("namespace termination lost backup duty: %v", current.Finalizers)
				}
			} else if !slices.Contains(current.Finalizers, kvTLSFinalizer) {
				t.Fatal("released TLS duty before observed cleanup")
			}
			if scenario == "Secret read forbidden" {
				return
			}
			remaining := &corev1.Secret{}
			if err := secrets.Get(ctx, client.ObjectKeyFromObject(secret), remaining); err != nil {
				t.Fatalf("Secret lost: %v", err)
			}
			if scenario == "replacement UID" || scenario == "replacement resourceVersion" {
				if !*interleaved || remaining.Labels[execution.LabelKeyValueUID] != "next-lifetime" {
					t.Fatal("replacement was not preserved")
				}
				if _, err := r.Reconcile(ctx, req); err == nil {
					t.Fatal("retry adopted foreign replacement")
				}
			}
			if scenario == "delayed Secret deletion" {
				if remaining.DeletionTimestamp.IsZero() {
					t.Fatal("Secret deletion not requested")
				}
				if _, err := r.Reconcile(ctx, req); err != nil {
					t.Fatal(err)
				}
				if err := cl.Get(ctx, req.NamespacedName, current); err != nil {
					t.Fatalf("pending Secret deletion released parent: %v", err)
				}
				remaining.Finalizers = nil
				if err := secrets.Update(ctx, remaining); err != nil {
					t.Fatal(err)
				}
				if _, err := r.Reconcile(ctx, req); err != nil {
					t.Fatal(err)
				}
				if err := cl.Get(ctx, req.NamespacedName, current); !apierrors.IsNotFound(err) {
					t.Fatalf("observed absence did not finish: %v", err)
				}
			}
		})
	}
}
