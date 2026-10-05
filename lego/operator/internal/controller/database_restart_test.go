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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// restartFixture is a served, healthy single-instance Database whose user just
// asked for a restart: spec.restartedAt is set, CNPG still reports the instance
// ready, and the instance Pod carries podRestartedAt.
func restartFixture(t *testing.T, restartedAt time.Time, podRestartedAt string) (*DatabaseReconciler, client.Client, reconcile.Request) {
	t.Helper()
	r, cl, req := newMajorUpgradeReconciler(t, "Cluster in healthy state")
	ctx := context.Background()

	var db appv1alpha1.Database
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	db.Spec.Version = "16"
	db.Spec.RestartedAt = restartedAt.UTC().Format(time.RFC3339)
	if err := cl.Update(ctx, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.Provisioned = true
	if err := cl.Status().Update(ctx, &db); err != nil {
		t.Fatal(err)
	}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(ctx, req.NamespacedName, cluster); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cluster.Object, int64(1), "status", "readyInstances"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(ctx, cluster); err != nil {
		t.Fatal(err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name + "-1", Namespace: req.Namespace,
			Labels:      map[string]string{"cnpg.io/cluster": req.Name, "cnpg.io/podRole": "instance"},
			Annotations: map[string]string{restartAnnotation: podRestartedAt},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	if err := cl.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	r.Now = func() time.Time { return restartedAt.Add(5 * time.Second) }
	return r, cl, req
}

func readyCondition(t *testing.T, cl client.Client, req reconcile.Request) (appv1alpha1.Database, *metav1.Condition) {
	t.Helper()
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	return db, meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionReady)
}

// w4/m137 t010: right after a restart request CNPG still counts the old
// instance ready, so the Database was marked Ready for the restart's own
// generation and the API read `available` before the restart began. It must
// stay non-Ready until the instance CNPG brings back carries the requested
// restart, and only then become Ready.
func TestDatabaseRestartStaysNotReadyUntilCNPGRestartsTheInstance(t *testing.T) {
	ctx := context.Background()
	restartedAt := time.Date(2026, 10, 5, 3, 33, 52, 0, time.UTC)
	r, cl, req := restartFixture(t, restartedAt, "2026-10-04T00:00:00Z")

	res, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	db, ready := readyCondition(t, cl, req)
	if db.Status.Phase == appv1alpha1.DBPhaseReady || ready == nil || ready.Status == metav1.ConditionTrue {
		t.Fatalf("before CNPG restarts the instance: phase %q ready %+v, want not Ready", db.Status.Phase, ready)
	}
	if ready.Reason != "Restarting" || !db.Status.Provisioned {
		t.Fatalf("reason %q provisioned %v, want Restarting on a served database", ready.Reason, db.Status.Provisioned)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > 3*time.Second {
		t.Fatalf("requeue %v, want a close poll of the rollout", res.RequeueAfter)
	}

	// CNPG brings the instance back stamped with the requested restart.
	var pod corev1.Pod
	if err := cl.Get(ctx, types.NamespacedName{Name: req.Name + "-1", Namespace: req.Namespace}, &pod); err != nil {
		t.Fatal(err)
	}
	pod.Annotations[restartAnnotation] = db.Spec.RestartedAt
	if err := cl.Update(ctx, &pod); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	db, ready = readyCondition(t, cl, req)
	if db.Status.Phase != appv1alpha1.DBPhaseReady || ready == nil || ready.Status != metav1.ConditionTrue || ready.ObservedGeneration != db.Generation {
		t.Fatalf("after the restart: phase %q ready %+v, want Ready for generation %d", db.Status.Phase, ready, db.Generation)
	}
}

// A restarted instance that is not yet serving does not count.
func TestDatabaseRestartWaitsForTheRestartedInstanceToBeReady(t *testing.T) {
	ctx := context.Background()
	restartedAt := time.Date(2026, 10, 5, 3, 33, 52, 0, time.UTC)
	r, cl, req := restartFixture(t, restartedAt, restartedAt.Format(time.RFC3339))
	var pod corev1.Pod
	if err := cl.Get(ctx, types.NamespacedName{Name: req.Name + "-1", Namespace: req.Namespace}, &pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
	if err := cl.Status().Update(ctx, &pod); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if db, ready := readyCondition(t, cl, req); db.Status.Phase == appv1alpha1.DBPhaseReady || ready.Reason != "Restarting" {
		t.Fatalf("restarted instance not ready: phase %q ready %+v, want Restarting", db.Status.Phase, ready)
	}
}

// A CNPG that never acts on the annotation must not wedge the database:
// past restartRolloutBound the restart stops holding Ready back.
func TestDatabaseRestartHoldIsBounded(t *testing.T) {
	ctx := context.Background()
	restartedAt := time.Date(2026, 10, 5, 3, 33, 52, 0, time.UTC)
	r, cl, req := restartFixture(t, restartedAt, "2026-10-04T00:00:00Z")
	r.Now = func() time.Time { return restartedAt.Add(restartRolloutBound + time.Second) }
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if db, ready := readyCondition(t, cl, req); db.Status.Phase != appv1alpha1.DBPhaseReady || ready.Status != metav1.ConditionTrue {
		t.Fatalf("past the bound: phase %q ready %+v, want Ready", db.Status.Phase, ready)
	}
}
