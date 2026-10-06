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

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func conflictOn(resource, name string) error {
	return apierrors.NewConflict(schema.GroupResource{Resource: resource}, name, errors.New("changed"))
}

// assertNotFailed checks the stored App carries no failure: bex-api closes a
// release's open deploy when its phase is Failed.
func assertNotFailed(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	if live := liveApp(t, cl, nn); live.Status.Phase == appv1alpha1.PhaseFailed {
		t.Fatalf("a write conflict was stored as a failure: phase Failed, Ready %+v", findReadyCondition(&live))
	}
}

// reconcileUntilConflicted runs passes until one hits the armed conflict, and
// returns that pass's error. A pass before it must succeed.
func reconcileUntilConflicted(t *testing.T, r *AppReconciler, nn types.NamespacedName, conflicts *int) error {
	t.Helper()
	for pass := range 5 {
		_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
		if *conflicts > 0 {
			return err
		}
		if err != nil {
			t.Fatalf("pass %d, before the conflict: %v", pass, err)
		}
	}
	t.Fatal("setup: no pass made the write that conflicts")
	return nil
}

// TestAConfigSnapshotWriteConflictRequeuesTheDeploy (w5/095): recording a
// release's config snapshot writes the App's status, and a conflict there is
// a lost race. The pass returns it and the next one records the snapshot. It
// used to be stored as phase Failed, which closed the release's open deploy:
// on the cron path and on a web service's.
func TestAConfigSnapshotWriteConflictRequeuesTheDeploy(t *testing.T) {
	for name, app := range map[string]func() *appv1alpha1.App{
		"cron job": func() *appv1alpha1.App {
			app := activeApp("tea-snapshot-cron")
			app.Spec.Type, app.Spec.Tier, app.Spec.Expose = appv1alpha1.TypeCronJob, "", false
			app.Spec.Schedule, app.Spec.Command = "* * * * *", "echo hi"
			return app
		},
		"web service": func() *appv1alpha1.App { return activeApp("tea-snapshot-web") },
	} {
		t.Run(name, func(t *testing.T) {
			conflicts := 0
			r, cl, nn := lifecycleFixture(t, app(), withInterceptor(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if written, ok := obj.(*appv1alpha1.App); ok && sub == "status" && conflicts == 0 && written.Status.ConfigSnapshotGeneration != 0 {
						conflicts++
						return conflictOn("apps", obj.GetName())
					}
					return c.SubResource(sub).Update(ctx, obj, opts...)
				},
			}))

			if err := reconcileUntilConflicted(t, r, nn, &conflicts); !apierrors.IsConflict(err) {
				t.Fatalf("the conflicted pass returned %v, want the conflict, so it requeues", err)
			}
			assertNotFailed(t, cl, nn)
			reconcileOnce(t, r, nn)
			if liveApp(t, cl, nn).Status.ConfigSnapshotGeneration == 0 {
				t.Fatal("the next pass did not record the release's config snapshot")
			}
			assertNotFailed(t, cl, nn)
		})
	}
}

// TestADeploymentWriteConflictRequeuesTheRollout (w5/095): the operator reads
// the Deployment from its cache, so updating it can lose a race with the
// Deployment controller's own writes. Release 2's rollout then retries on the
// next pass instead of failing.
func TestADeploymentWriteConflictRequeuesTheRollout(t *testing.T) {
	armed, conflicts := false, 0
	r, cl, nn := lifecycleFixture(t, activeApp("tea-deployment-conflict"), withInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*appsv1.Deployment); ok && armed && conflicts == 0 {
				conflicts++
				return conflictOn("deployments", obj.GetName())
			}
			return c.Update(ctx, obj, opts...)
		},
	}))
	serveReleaseOne(t, r, cl, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	armed = true

	if err := reconcileUntilConflicted(t, r, nn, &conflicts); !apierrors.IsConflict(err) {
		t.Fatalf("the conflicted pass returned %v, want the conflict", err)
	}
	assertNotFailed(t, cl, nn)
	reconcileOnce(t, r, nn)
	if got := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0].Image; got != "nginx:2" {
		t.Fatalf("the next pass left the Deployment on %q, want nginx:2", got)
	}
	assertNotFailed(t, cl, nn)
}

// TestASelectedConfigWriteConflictRequeuesTheDeploy (w5/095): a release that
// selects an earlier release's configuration records the selection in the
// App's status before planning. A conflict there retries too.
func TestASelectedConfigWriteConflictRequeuesTheDeploy(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	stored := liveApp(t, r.Client, nn)
	stored.Labels = map[string]string{labelWorkspace: app.Namespace} // a canonical tenant App
	if err := r.Update(ctx, &stored); err != nil {
		t.Fatal(err)
	}
	conflicts := 0
	r.Client = interceptor.NewClient(r.Client.(client.WithWatch), interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if written, ok := obj.(*appv1alpha1.App); ok && sub == "status" && conflicts == 0 && written.Status.ConfigSnapshotGeneration == 3 {
				conflicts++
				return conflictOn("apps", obj.GetName())
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	})

	if err := reconcileUntilConflicted(t, r, nn, &conflicts); !apierrors.IsConflict(err) {
		t.Fatalf("the conflicted pass returned %v, want the conflict", err)
	}
	assertNotFailed(t, r.Client, nn)
}

// TestFailRecordsAnythingButAConflict (w5/095): fail records every failure
// but a write conflict, whatever the reason. A conflict recorded as a build
// failure would stand as terminal: no pass retries a release with one.
func TestFailRecordsAnythingButAConflict(t *testing.T) {
	ctx := context.Background()
	r, cl, nn := lifecycleFixture(t, activeApp("tea-fail"))
	reconcileOnce(t, r, nn)

	for _, reason := range []string{"DeployFailed", "ServiceFailed", appv1alpha1.ReasonBuildFailedUserError} {
		app := liveApp(t, cl, nn)
		if _, err := r.fail(ctx, &app, reason, conflictOn("deployments", app.Name)); !apierrors.IsConflict(err) {
			t.Fatalf("fail(%s, conflict) = %v, want the conflict back", reason, err)
		}
		assertNotFailed(t, cl, nn)
		if live := liveApp(t, cl, nn); meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild) != nil {
			t.Fatalf("fail(%s, conflict) stored a Build verdict", reason)
		}
	}

	app := liveApp(t, cl, nn)
	_, _ = r.fail(ctx, &app, "DeployFailed", errors.New("image pull secret missing"))
	live := liveApp(t, cl, nn)
	if ready := findReadyCondition(&live); live.Status.Phase != appv1alpha1.PhaseFailed || ready == nil || ready.Reason != "DeployFailed" {
		t.Fatalf("a real failure was not stored: phase %q, Ready %+v", live.Status.Phase, ready)
	}
}

// TestAFailedSnapshotRecordKeepsTheServedSnapshot (w5/095): when recording a
// new release's config snapshot fails, the pass rolls its in-memory status
// back to the snapshot the stored status still records. The failure written
// after it used to store generation 0, dropping the served release's snapshot
// for the mutable names.
func TestAFailedSnapshotRecordKeepsTheServedSnapshot(t *testing.T) {
	failed := false
	r, cl, nn := lifecycleFixture(t, activeApp("tea-snapshot-rollback"), withInterceptor(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if written, ok := obj.(*appv1alpha1.App); ok && sub == "status" && !failed && written.Status.ConfigSnapshotGeneration == 2 {
				failed = true
				return errors.New("the API server is unavailable")
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	}))
	serveReleaseOne(t, r, cl, nn)
	if got := liveApp(t, cl, nn).Status.ConfigSnapshotGeneration; got != 1 {
		t.Fatalf("setup: release 1 recorded snapshot %d, want 1", got)
	}
	deployImageAt(t, cl, nn, "nginx:2", 2)

	for pass := 0; !failed; pass++ {
		if pass == 5 {
			t.Fatal("setup: no pass recorded release 2's snapshot")
		}
		_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	}
	if got := liveApp(t, cl, nn).Status.ConfigSnapshotGeneration; got != 1 {
		t.Fatalf("after the failed record the stored snapshot is %d, want the served release's 1", got)
	}
}
