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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func staticDeletionApp(name string) *appv1alpha1.App {
	app := deletionApp(name)
	app.Spec.Type = appv1alpha1.TypeStaticSite
	return app
}

func staticPurgeCreds(ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "static-creds", Namespace: ns},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("test-access"),
			"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
		},
	}
}

// TestStaticAppDeletionSurfacesStalledConditionOnOverrun proves the w3/m81
// bound: a static-content purge that has not converged past the finalization
// window is stamped DeletionStalled (the actionable operator signal), its
// requeue backs off from settleRequeue to childHealthRequeue, and the finalizer
// is RETAINED so no object prefix is silently orphaned. A negligible
// FinalizerOverrunAfter makes the still-running purge an overrun immediately, so
// the failure path is asserted without a multi-minute wall-clock wait.
func TestStaticAppDeletionSurfacesStalledConditionOnOverrun(t *testing.T) {
	app := staticDeletionApp("static-stalled")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, staticPurgeCreds(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	store := publish.Store{Bucket: "static", Endpoint: "https://s3.example", Secret: "static-creds"}
	r := &AppReconciler{
		Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes,
		StaticStore: store, FinalizerOverrunAfter: time.Nanosecond,
	}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	// The purge Job is created but not yet complete, so cleanup is pending; the
	// overrun backs the requeue off to childHealthRequeue.
	if result.RequeueAfter != childHealthRequeue {
		t.Fatalf("stalled requeue = %v, want childHealthRequeue %v", result.RequeueAfter, childHealthRequeue)
	}

	var current appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(&current, finalizer) {
		t.Fatal("overrun released the finalizer; a stalled purge must retain it so no object prefix is silently orphaned")
	}
	cond := meta.FindStatusCondition(current.Status.Conditions, conditionDeletionStalled)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("DeletionStalled condition = %+v, want present and True", cond)
	}
	if cond.Reason != reasonCleanupExceededDeadline {
		t.Fatalf("DeletionStalled reason = %q, want %q", cond.Reason, reasonCleanupExceededDeadline)
	}
}

// TestStaticAppDeletionWithinBoundStaysQuiet is the control: an ordinary
// in-progress finalization (default 15-minute window, DeletionTimestamp ~now) is
// NOT an overrun — it keeps the tight settleRequeue and stamps no DeletionStalled
// condition, so the signal only ever fires for a genuine hang.
func TestStaticAppDeletionWithinBoundStaysQuiet(t *testing.T) {
	app := staticDeletionApp("static-normal")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, staticPurgeCreds(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	store := publish.Store{Bucket: "static", Endpoint: "https://s3.example", Secret: "static-creds"}
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, StaticStore: store}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != settleRequeue {
		t.Fatalf("in-window requeue = %v, want settleRequeue %v", result.RequeueAfter, settleRequeue)
	}
	var current appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if cond := meta.FindStatusCondition(current.Status.Conditions, conditionDeletionStalled); cond != nil {
		t.Fatalf("healthy in-progress deletion stamped DeletionStalled: %+v", cond)
	}
	if !controllerutil.ContainsFinalizer(&current, finalizer) {
		t.Fatal("in-progress finalization must retain the finalizer")
	}
}

// TestStaticAppDeletionOverrunStillFinalizesOnCompletion proves the overrun
// signal is not a dead end: once the purge Job completes, the finalizer is
// removed and the App is deleted, exactly as a healthy teardown — the stall was
// observability, not a terminal state.
func TestStaticAppDeletionOverrunStillFinalizesOnCompletion(t *testing.T) {
	app := staticDeletionApp("static-recovers")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, staticPurgeCreds(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	store := publish.Store{Bucket: "static", Endpoint: "https://s3.example", Secret: "static-creds"}
	r := &AppReconciler{
		Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes,
		StaticStore: store, FinalizerOverrunAfter: time.Nanosecond,
	}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	// First pass: purge Job created, stalled condition stamped, finalizer held.
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	desired := publish.PurgeJob(app.Name, string(app.UID), "", app.Namespace, store, app.Namespace, "", "")
	var job batchv1.Job
	if err := cl.Get(context.Background(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &job); err != nil {
		t.Fatalf("purge Job not created: %v", err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(context.Background(), &job); err != nil {
		t.Fatal(err)
	}

	// Subsequent passes observe completion, tear the Job down, and remove the
	// finalizer — the App is gone despite having been flagged stalled.
	for range 3 {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); err == nil {
		t.Fatal("App survived proven purge completion after an overrun flag")
	}
}
