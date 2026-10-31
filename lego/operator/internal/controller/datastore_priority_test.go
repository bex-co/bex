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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m247: datastore Pods run under the datastore PriorityClass so the
// workspace compute quota (scoped NotIn that class) does not charge them, and
// existing stores adopt it only when their Pods roll anyway.

func TestNewDatastoresCarryTheDatastorePriorityClass(t *testing.T) {
	plan, ok := tiers.Postgres.ByID("1c-2g")
	if !ok {
		t.Fatal("no 1c-2g plan")
	}
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 10, highAvailability: true})
	if got := spec["priorityClassName"]; got != appv1alpha1.DatastorePriorityClassName {
		t.Errorf("Cluster priorityClassName = %v", got)
	}
	pooler := &unstructured.Unstructured{Object: map[string]any{"spec": poolerSpec("db")}}
	if got, _, _ := unstructured.NestedString(pooler.Object, "spec", "template", "spec", "priorityClassName"); got != appv1alpha1.DatastorePriorityClassName {
		t.Errorf("Pooler template priorityClassName = %q", got)
	}
	kv := isolatedKV("")
	kvPlan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, kvPlan, storage, "kv.example.test", false, false))
	if got := sts.Spec.Template.Spec.PriorityClassName; got != appv1alpha1.DatastorePriorityClassName {
		t.Errorf("Valkey template priorityClassName = %q", got)
	}
}

func TestExistingKeyValueAdoptsThePriorityClassOnlyWhenItRolls(t *testing.T) {
	kv := isolatedKV("")
	plan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, false))
	// A store created before the class existed.
	sts.CreationTimestamp = metav1.Now()
	sts.Spec.Template.Spec.PriorityClassName = ""
	settled := sts.Spec.Template.DeepCopy()

	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, false))
	if got := sts.Spec.Template.Spec.PriorityClassName; got != "" {
		t.Fatalf("an unchanged store was rolled to adopt the class: %q", got)
	}
	// An isolation toggle is relabelled in place; it must not carry the class in.
	kv.Labels[labelNetworkIsolation] = "env-a"
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, false))
	if !templateEqualIgnoringIsolation(settled, &sts.Spec.Template) {
		t.Fatalf("an isolation toggle rolled the store: %+v", sts.Spec.Template.Spec.PriorityClassName)
	}
	// A plan change rolls the Pod anyway: the class rides along and then stays.
	kv.Spec.Plan = "starter"
	plan, storage = resolveKVPlan(kv.Spec)
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, false))
	if got := sts.Spec.Template.Spec.PriorityClassName; got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("a rolling store did not adopt the class: %q", got)
	}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, false))
	if got := sts.Spec.Template.Spec.PriorityClassName; got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("an adopted class was dropped: %q", got)
	}
}

func TestExistingClusterAdoptsThePriorityClassOnlyWhenItRolls(t *testing.T) {
	desired := func(cpu string, instances int64, image string) map[string]any {
		spec := map[string]any{
			"instances": instances,
			"resources": map[string]any{
				"requests": map[string]any{"cpu": cpu, "memory": "2Gi"},
				"limits":   map[string]any{"cpu": cpu, "memory": "2Gi"},
			},
			"priorityClassName": appv1alpha1.DatastorePriorityClassName,
		}
		if image != "" {
			spec["imageName"] = image
		}
		return spec
	}
	live := func(mutate func(*unstructured.Unstructured)) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"instances": int64(1),
			"imageName": "pg:16",
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "1", "memory": "2Gi"},
				"limits":   map[string]any{"cpu": "1", "memory": "2Gi"},
			},
		}}}
		u.SetResourceVersion("7")
		if mutate != nil {
			mutate(u)
		}
		return u
	}
	for _, tc := range []struct {
		name        string
		live        *unstructured.Unstructured
		desired     map[string]any
		suspended   bool
		restartedAt string
		want        bool
	}{
		{"new cluster", &unstructured.Unstructured{Object: map[string]any{}}, desired("1", 1, ""), false, "", true},
		{"unchanged existing cluster", live(nil), desired("1", 1, "pg:16"), false, "", false},
		{"same plan spelled in millicores", live(nil), desired("1000m", 1, ""), false, "", false},
		{"instance removed", live(func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, int64(2), "spec", "instances")
		}), desired("1", 1, ""), false, "", false},
		{"plan resize", live(nil), desired("2", 1, ""), false, "", true},
		{"HA standby added", live(nil), desired("1", 2, ""), false, "", true},
		{"image update", live(nil), desired("1", 1, "pg:17"), false, "", true},
		{"restart requested", live(nil), desired("1", 1, ""), false, "2026-10-30T00:00:00Z", true},
		{"restart already applied", live(func(u *unstructured.Unstructured) {
			u.SetAnnotations(map[string]string{restartAnnotation: "2026-10-30T00:00:00Z"})
		}), desired("1", 1, ""), false, "2026-10-30T00:00:00Z", false},
		{"suspended", live(nil), desired("1", 1, ""), true, "", true},
		{"resuming from hibernation", live(func(u *unstructured.Unstructured) {
			u.SetAnnotations(map[string]string{hibernationAnnotation: "on"})
		}), desired("1", 1, ""), false, "", true},
		{"already adopted", live(func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, appv1alpha1.DatastorePriorityClassName, "spec", "priorityClassName")
		}), desired("1", 1, ""), false, "", true},
	} {
		if got := clusterAdoptsDatastorePriority(tc.live, tc.desired, tc.suspended, tc.restartedAt); got != tc.want {
			t.Errorf("%s: adopts = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestExistingPoolerFollowsItsClustersPriorityClass(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	existing.SetResourceVersion("3")
	classless := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	adopted := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"priorityClassName": appv1alpha1.DatastorePriorityClassName}}}
	if !poolerAdoptsDatastorePriority(&unstructured.Unstructured{Object: map[string]any{}}, classless) {
		t.Error("a new Pooler must carry the class")
	}
	if poolerAdoptsDatastorePriority(existing, classless) {
		t.Error("an existing Pooler was rolled while its Cluster's Pods were not")
	}
	if !poolerAdoptsDatastorePriority(existing, adopted) {
		t.Error("an existing Pooler did not follow its Cluster's adoption")
	}
	withClass := existing.DeepCopy()
	_ = unstructured.SetNestedField(withClass.Object, poolerTemplate(), "spec", "template")
	if !poolerAdoptsDatastorePriority(withClass, classless) {
		t.Error("an adopted Pooler class was dropped")
	}
}

// w2/057: the datastores' maintenance Pods (Postgres export, export expiry and
// backup purge; Key Value backup and purge) carry the class too, so a
// workspace whose services fill its compute quota still backs up and exports.
func TestDatastoreMaintenanceJobsCarryTheDatastorePriorityClass(t *testing.T) {
	db := exportTestDatabase(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	dbs := &DatabaseReconciler{Backup: testStore}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-paid-kv", Namespace: "default", UID: "paid-kv-uid"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	kvs := &KeyValueReconciler{Backup: testKeyValueBackupStore}
	for name, pod := range map[string]corev1.PodSpec{
		"postgres export":        exportJob(db, db.Spec.Exports[0], testStore).Spec.Template.Spec,
		"postgres export expiry": exportCleanupJob(db, appv1alpha1.DatabaseExportStatus{ID: db.Spec.Exports[0].ID, ObjectKey: "s3://b/k"}, testStore).Spec.Template.Spec,
		"postgres backup purge":  dbs.dbBackupPurgeJob(db).Spec.Template.Spec,
		"key value backup":       kvs.keyValueBackupCronJobSpec(kv, starterValkeyTier(), "red-paid-kv-platform").JobTemplate.Spec.Template.Spec,
		"key value backup purge": kvs.keyValueBackupPurgeJob(kv).Spec.Template.Spec,
	} {
		if pod.PriorityClassName != appv1alpha1.DatastorePriorityClassName {
			t.Errorf("%s Pod priorityClassName = %q, want %q", name, pod.PriorityClassName, appv1alpha1.DatastorePriorityClassName)
		}
	}
}

// The rule above, at its call site: reconcileCluster must not add the class to
// a live Cluster that predates it on an unchanged reconcile (that would roll
// every instance), and must add it when the reconcile adds an instance.
func TestReconcileClusterHoldsThePriorityClassUntilTheClusterRolls(t *testing.T) {
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
		Spec:       appv1alpha1.DatabaseSpec{Plan: "1c-2g"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	plan, _ := resolvePlan(db.Spec)
	params := clusterParams{plan: plan, storageGB: 10, namespace: db.Namespace}
	key := client.ObjectKey{Namespace: db.Namespace, Name: db.Name}
	reconcile := func(p clusterParams) *unstructured.Unstructured {
		t.Helper()
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		cluster.SetName(db.Name)
		cluster.SetNamespace(db.Namespace)
		if err := cl.Get(ctx, key, cluster); err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		if err := r.reconcileCluster(ctx, db, cluster, p); err != nil {
			t.Fatal(err)
		}
		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(cnpgClusterGVK)
		if err := cl.Get(ctx, key, live); err != nil {
			t.Fatal(err)
		}
		return live
	}
	class := func(u *unstructured.Unstructured) string {
		got, _, _ := unstructured.NestedString(u.Object, "spec", "priorityClassName")
		return got
	}

	if got := class(reconcile(params)); got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("a new Cluster carries class %q", got)
	}

	// A Cluster created before the class existed.
	live := reconcile(params)
	unstructured.RemoveNestedField(live.Object, "spec", "priorityClassName")
	if err := cl.Update(ctx, live); err != nil {
		t.Fatal(err)
	}
	if got := class(reconcile(params)); got != "" {
		t.Fatalf("an unchanged Cluster was given the class (rolling every instance): %q", got)
	}

	// Enabling HA adds the standby, so CNPG rolls the instances anyway.
	params.highAvailability = true
	if got := class(reconcile(params)); got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("a Cluster gaining an instance did not adopt the class: %q", got)
	}
}
