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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Every managed Postgres gets a CNPG Database child adopting its application
// database — the default or custom name and owner a create or restore wrote —
// with vector ensured present and the database retained if the child goes
// (w8/m84). Its CNPG verdict becomes the ExtensionsReady condition.
func TestDatabaseApplicationDatabaseChild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spec   appv1alpha1.DatabaseSpec
		dbname string
		owner  string
	}{
		{name: "default identity", spec: appv1alpha1.DatabaseSpec{Name: "orders"}},
		{name: "custom identity", spec: appv1alpha1.DatabaseSpec{Name: "orders", DatabaseName: "orders_prod", DatabaseUser: "orders_app"}, dbname: "orders_prod", owner: "orders_app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = appv1alpha1.AddToScheme(scheme)
			scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(cnpgDatabaseGVK, &unstructured.Unstructured{})
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-vector", Namespace: "default", UID: "uid-dpg-vector"}, Spec: tc.spec}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{}).Build()
			r := &DatabaseReconciler{Client: cl, Scheme: scheme}
			nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
			for range 2 {
				_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
			}
			wantName, wantOwner := tc.dbname, tc.owner
			if wantName == "" {
				wantName, wantOwner = tc.spec.EffectiveDatabaseName(db.Name), tc.spec.EffectiveDatabaseUser(db.Name)
			}
			child := &unstructured.Unstructured{}
			child.SetGroupVersionKind(cnpgDatabaseGVK)
			if err := cl.Get(context.Background(), types.NamespacedName{Name: "dpg-vector-app", Namespace: "default"}, child); err != nil {
				t.Fatalf("CNPG Database child: %v", err)
			}
			spec, _, _ := unstructured.NestedMap(child.Object, "spec")
			cluster, _, _ := unstructured.NestedString(spec, "cluster", "name")
			extensions, _, _ := unstructured.NestedSlice(spec, "extensions")
			if cluster != db.Name || spec["name"] != wantName || spec["owner"] != wantOwner || spec["ensure"] != "present" ||
				spec["databaseReclaimPolicy"] != "retain" || len(extensions) != 1 {
				t.Fatalf("child spec = %#v, want cluster %s, database %s owned by %s, ensured, retained, one extension", spec, db.Name, wantName, wantOwner)
			}
			if ext := extensions[0].(map[string]any); ext["name"] != "vector" || ext["ensure"] != "present" || ext["version"] != nil {
				t.Errorf("extension = %#v, want vector ensured present with no pinned version", ext)
			}
			if owners := child.GetOwnerReferences(); len(owners) != 1 || owners[0].Name != db.Name || owners[0].Kind != "Database" {
				t.Errorf("owners = %+v, want the bex Database", owners)
			}
		})
	}
}

func TestExtensionsCondition(t *testing.T) {
	child := func(generation int64, status map[string]any) *unstructured.Unstructured {
		o := &unstructured.Unstructured{Object: map[string]any{}}
		o.SetGeneration(generation)
		if status != nil {
			o.Object["status"] = status
		}
		return o
	}
	for _, tc := range []struct {
		name   string
		child  *unstructured.Unstructured
		status metav1.ConditionStatus
		reason string
	}{
		{"not yet reconciled", child(1, nil), metav1.ConditionUnknown, "Pending"},
		{"stale verdict", child(2, map[string]any{"applied": true, "observedGeneration": int64(1)}), metav1.ConditionUnknown, "Pending"},
		{"applied", child(2, map[string]any{"applied": true, "observedGeneration": int64(2)}), metav1.ConditionTrue, "Applied"},
		{"failed", child(2, map[string]any{"applied": false, "observedGeneration": int64(1), "message": "extension \"vector\" is not available"}), metav1.ConditionFalse, "ApplyFailed"},
		{"failed before any success", child(1, map[string]any{"applied": false, "message": "extension \"vector\" is not available"}), metav1.ConditionFalse, "ApplyFailed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var conditions []metav1.Condition
			meta.SetStatusCondition(&conditions, extensionsCondition(tc.child, 7))
			got := meta.FindStatusCondition(conditions, conditionExtensionsReady)
			if got.Status != tc.status || got.Reason != tc.reason || got.ObservedGeneration != 7 {
				t.Errorf("condition = %+v, want %s/%s at generation 7", got, tc.status, tc.reason)
			}
			if tc.reason == "ApplyFailed" && got.Message != `extension "vector" is not available` {
				t.Errorf("message = %q, want CNPG's", got.Message)
			}
		})
	}
}

// A database PostgreSQL itself owns (a legacy template1 name, w4/m170) gets no
// CNPG child, which CNPG would refuse on every reconcile, and a child that
// cannot be written degrades only ExtensionsReady: the database still
// reconciles.
func TestApplicationDatabaseChildNeverWedgesTheDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		spec         appv1alpha1.DatabaseSpec
		failWrite    bool
	}{
		{name: "reserved database name", reason: "ReservedDatabaseName", spec: appv1alpha1.DatabaseSpec{Name: "legacy", DatabaseName: "template1"}},
		{name: "child write refused", reason: "ApplyError", spec: appv1alpha1.DatabaseSpec{Name: "orders"}, failWrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = appv1alpha1.AddToScheme(scheme)
			scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(cnpgDatabaseGVK, &unstructured.Unstructured{})
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-guard", Namespace: "default", UID: "uid-dpg-guard"}, Spec: tc.spec}
			refuse := interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if u, ok := obj.(*unstructured.Unstructured); ok && tc.failWrite && u.GroupVersionKind() == cnpgDatabaseGVK {
					return errors.New("admission webhook denied the request")
				}
				return c.Create(ctx, obj, opts...)
			}}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{}).
				WithInterceptorFuncs(refuse).Build()
			r := &DatabaseReconciler{Client: cl, Scheme: scheme}
			nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
			for range 2 {
				_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
			}
			child := &unstructured.Unstructured{}
			child.SetGroupVersionKind(cnpgDatabaseGVK)
			if err := cl.Get(context.Background(), types.NamespacedName{Name: "dpg-guard-app", Namespace: "default"}, child); err == nil {
				t.Fatalf("a CNPG child exists: %v", child.Object)
			}
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			if err := cl.Get(context.Background(), nn, cluster); err != nil {
				t.Fatalf("the Cluster is no longer reconciled: %v", err)
			}
			var got appv1alpha1.Database
			if err := cl.Get(context.Background(), nn, &got); err != nil {
				t.Fatal(err)
			}
			condition := meta.FindStatusCondition(got.Status.Conditions, conditionExtensionsReady)
			if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != tc.reason {
				t.Errorf("ExtensionsReady = %+v, want False/%s", condition, tc.reason)
			}
			if got.Status.Phase == appv1alpha1.DBPhaseFailed {
				t.Errorf("phase = Failed; the child must not fail the database")
			}
		})
	}
}
