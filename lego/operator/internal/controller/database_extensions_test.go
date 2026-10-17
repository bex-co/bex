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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		{name: "reserved database name", reason: reasonReservedDatabaseName, spec: appv1alpha1.DatabaseSpec{Name: "legacy", DatabaseName: "template1"}},
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
				t.Errorf("phase = Failed; the child must not fail a database whose cluster is not yet healthy")
			}
		})
	}
}

// setApplicationDatabaseStatus stands in for CloudNativePG: it writes status
// onto db's application Database child.
func setApplicationDatabaseStatus(ctx context.Context, cl client.Client, db *appv1alpha1.Database, status func(generation int64) map[string]any) error {
	child := &unstructured.Unstructured{}
	child.SetGroupVersionKind(cnpgDatabaseGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: applicationDatabaseName(db)}, child); err != nil {
		return err
	}
	if err := unstructured.SetNestedMap(child.Object, status(child.GetGeneration()), "status"); err != nil {
		return err
	}
	if err := cl.Status().Update(ctx, child); err == nil {
		return nil
	}
	return cl.Update(ctx, child)
}

// markApplicationDatabaseApplied reports db's application Database child
// applied for its current generation, which a new database needs before its
// first Ready (w4/m223).
func markApplicationDatabaseApplied(ctx context.Context, cl client.Client, db *appv1alpha1.Database) error {
	return setApplicationDatabaseStatus(ctx, cl, db, func(generation int64) map[string]any {
		return map[string]any{"applied": true, "observedGeneration": generation}
	})
}

// healthyNewDatabase is a database that has never served, reconciled once,
// whose CNPG cluster now reports a healthy instance. A new database's first
// Ready waits for its managed extensions (w4/m223, a868: it used to arrive
// 8-18s before vector, failing a migration run at the first Available).
func healthyNewDatabase(t *testing.T, spec appv1alpha1.DatabaseSpec, funcs ...interceptor.Funcs) (*DatabaseReconciler, client.Client, reconcile.Request, *appv1alpha1.Database) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	for _, gvk := range []schema.GroupVersionKind{cnpgClusterGVK, cnpgDatabaseGVK, cnpgBackupGVK, cnpgScheduledBackupGVK, cnpgPoolerGVK} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-first", Namespace: "default", UID: "uid-dpg-first", Finalizers: []string{dbFinalizer}},
		Spec:       spec,
	}
	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).WithStatusSubresource(&appv1alpha1.Database{})
	for _, f := range funcs {
		builder = builder.WithInterceptorFuncs(f)
	}
	cl := builder.Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	reconcileDB(t, r, cl, req)
	setClusterStatus(t, cl, req, "Cluster in healthy state", 1, 0)
	return r, cl, req, db
}

// readyReason is the Ready condition as "Status/Reason".
func readyReason(db appv1alpha1.Database) string {
	if c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionReady); c != nil {
		return string(c.Status) + "/" + c.Reason
	}
	return ""
}

func vectorUnavailable(int64) map[string]any {
	return map[string]any{"applied": false, "message": `extension "vector" is not available`}
}

func TestFirstReadyWaitsForPendingExtensions(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		status func(int64) map[string]any
	}{
		{"pending child", nil},
		{"verdict for another generation", func(g int64) map[string]any {
			return map[string]any{"applied": true, "observedGeneration": g + 7}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, cl, req, db := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "orders"})
			if tc.status != nil {
				if err := setApplicationDatabaseStatus(ctx, cl, db, tc.status); err != nil {
					t.Fatal(err)
				}
			}
			got := reconcileDB(t, r, cl, req)
			if got.Status.Phase != appv1alpha1.DBPhaseProvisioning || got.Status.Provisioned ||
				readyReason(got) != "False/"+reasonInitializingExtensions {
				t.Fatalf("phase %q provisioned %v Ready %s, want Provisioning, not provisioned, False/%s",
					got.Status.Phase, got.Status.Provisioned, readyReason(got), reasonInitializingExtensions)
			}
			if err := markApplicationDatabaseApplied(ctx, cl, db); err != nil {
				t.Fatal(err)
			}
			got = reconcileDB(t, r, cl, req)
			if got.Status.Phase != appv1alpha1.DBPhaseReady || !got.Status.Provisioned || readyReason(got) != "True/"+appv1alpha1.ReasonProvisioned {
				t.Fatalf("after apply: phase %q provisioned %v Ready %s, want first Ready", got.Status.Phase, got.Status.Provisioned, readyReason(got))
			}
		})
	}
}

func TestInitialExtensionFailureIsNamedAndRetried(t *testing.T) {
	ctx := context.Background()
	r, cl, req, db := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "orders"})
	if err := setApplicationDatabaseStatus(ctx, cl, db, vectorUnavailable); err != nil {
		t.Fatal(err)
	}
	result, err := r.Reconcile(ctx, req)
	if err != nil || result.RequeueAfter <= 0 {
		t.Fatalf("reconcile = %+v, %v; want a retry", result, err)
	}
	got := reconcileDB(t, r, cl, req)
	if got.Status.Phase != appv1alpha1.DBPhaseFailed || got.Status.Provisioned ||
		readyReason(got) != "False/"+appv1alpha1.ReasonExtensionsFailed {
		t.Fatalf("phase %q provisioned %v Ready %s, want Failed/%s", got.Status.Phase, got.Status.Provisioned, readyReason(got), appv1alpha1.ReasonExtensionsFailed)
	}
	if err := markApplicationDatabaseApplied(ctx, cl, db); err != nil {
		t.Fatal(err)
	}
	if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady {
		t.Fatalf("after a successful retry phase = %q, want Ready", got.Status.Phase)
	}
}

func TestServedDatabaseKeepsServingThroughALaterExtensionFailure(t *testing.T) {
	ctx := context.Background()
	r, cl, req, db := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "orders"})
	if err := markApplicationDatabaseApplied(ctx, cl, db); err != nil {
		t.Fatal(err)
	}
	if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady {
		t.Fatalf("first Ready phase = %q", got.Status.Phase)
	}
	if err := setApplicationDatabaseStatus(ctx, cl, db, vectorUnavailable); err != nil {
		t.Fatal(err)
	}
	got := reconcileDB(t, r, cl, req)
	extensions := meta.FindStatusCondition(got.Status.Conditions, conditionExtensionsReady)
	if got.Status.Phase != appv1alpha1.DBPhaseReady || readyReason(got) != "True/"+appv1alpha1.ReasonProvisioned ||
		extensions == nil || extensions.Reason != reasonExtensionsApplyFailed {
		t.Fatalf("phase %q Ready %s ExtensionsReady %+v, want still Ready with the failure on ExtensionsReady only",
			got.Status.Phase, readyReason(got), extensions)
	}
}

func TestReservedDatabaseNameKeepsItsLegacyFirstReady(t *testing.T) {
	r, cl, req, _ := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "legacy", DatabaseName: "template1"})
	if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady || !got.Status.Provisioned {
		t.Fatalf("reserved-name phase %q provisioned %v, want Ready", got.Status.Phase, got.Status.Provisioned)
	}
}

func TestSuspendingANeverServedDatabaseKeepsItsFirstReadyPending(t *testing.T) {
	ctx := context.Background()
	r, cl, req, db := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "orders"})
	setSuspended := func(suspended bool) {
		var current appv1alpha1.Database
		if err := cl.Get(ctx, req.NamespacedName, &current); err != nil {
			t.Fatal(err)
		}
		current.Spec.Suspended = suspended
		if err := cl.Update(ctx, &current); err != nil {
			t.Fatal(err)
		}
	}
	setSuspended(true)
	if got := reconcileDB(t, r, cl, req); got.Status.Provisioned {
		t.Fatal("suspension set provisioned on a database that never served")
	}
	setSuspended(false)
	setClusterStatus(t, cl, req, "Cluster in healthy state", 1, 0)
	if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseProvisioning || got.Status.Provisioned {
		t.Fatalf("resumed before extensions: phase %q provisioned %v, want Provisioning", got.Status.Phase, got.Status.Provisioned)
	}
	if err := markApplicationDatabaseApplied(ctx, cl, db); err != nil {
		t.Fatal(err)
	}
	if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady || !got.Status.Provisioned {
		t.Fatalf("resumed and applied: phase %q provisioned %v, want Ready", got.Status.Phase, got.Status.Provisioned)
	}
}

// A refused child write names the failure on a healthy new database instead of
// leaving it creating forever; a conflict or busy API server is only retried.
func TestRefusedChildWriteOnAHealthyNewDatabase(t *testing.T) {
	for _, tc := range []struct {
		name      string
		refusal   error
		wantPhase appv1alpha1.DatabasePhase
		wantReady string
	}{
		{"admission refusal", errors.New("admission webhook denied the request"),
			appv1alpha1.DBPhaseFailed, "False/" + appv1alpha1.ReasonExtensionsFailed},
		{"conflict", apierrors.NewConflict(schema.GroupResource{Group: "postgresql.cnpg.io", Resource: "databases"}, "dpg-first-app", errors.New("modified")),
			appv1alpha1.DBPhaseProvisioning, "False/" + reasonInitializingExtensions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refuse := interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if u, ok := obj.(*unstructured.Unstructured); ok && u.GroupVersionKind() == cnpgDatabaseGVK {
					return tc.refusal
				}
				return c.Create(ctx, obj, opts...)
			}}
			r, cl, req, _ := healthyNewDatabase(t, appv1alpha1.DatabaseSpec{Name: "orders"}, refuse)
			got := reconcileDB(t, r, cl, req)
			if got.Status.Phase != tc.wantPhase || readyReason(got) != tc.wantReady || got.Status.Provisioned {
				t.Fatalf("phase %q Ready %s provisioned %v, want %q %s", got.Status.Phase, readyReason(got), got.Status.Provisioned, tc.wantPhase, tc.wantReady)
			}
		})
	}
}
