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

	"github.com/prometheus/client_golang/prometheus/testutil"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// completeOwnBuild marks the build Job the App dispatched for its release
// complete, as the build plane does once the image is pushed.
func completeOwnBuild(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	app := liveApp(t, cl, nn)
	job := &batchv1.Job{}
	key := client.ObjectKey{Namespace: nn.Namespace, Name: build.JobName(nn.Name, releaseBuildRevision(&app))}
	if err := cl.Get(context.Background(), key, job); err != nil {
		t.Fatalf("setup: the release's build was never dispatched: %v", err)
	}
	now := metav1.Now()
	job.Status.StartTime, job.Status.CompletionTime = &now, &now
	job.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}
	if err := cl.Status().Update(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

// buildingReleaseTwo serves a prebuilt release 1, then dispatches release 2's
// build from source into the workspace's one free build slot. It returns
// release 1's image.
func buildingReleaseTwo(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	r.MaxConcurrentBuilds = 1
	_, prior := serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	reconcileOnce(t, r, nn)
	return prior
}

// failOwnBuild marks the App's release build Job failed by the tenant's build,
// the classification a Job's PodFailurePolicy reason carries.
func failOwnBuild(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	app := liveApp(t, cl, nn)
	job := &batchv1.Job{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: nn.Namespace, Name: build.JobName(nn.Name, releaseBuildRevision(&app))}, job); err != nil {
		t.Fatalf("setup: the release's build was never dispatched: %v", err)
	}
	now := metav1.Now()
	job.Status.StartTime = &now
	job.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy, LastTransitionTime: now},
	}
	if err := cl.Status().Update(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

// TestAFinishedBuildIsSettledWhileOtherBuildsHoldEverySlot (w5/108): the build
// caps gate a new dispatch, and a finished build Job is no longer active, so
// the pass that found the build finished counted it out and parked the
// release as BuildQueued until another build freed a slot. It is observed
// instead: a built release rolls out, and a failed build is recorded.
func TestAFinishedBuildIsSettledWhileOtherBuildsHoldEverySlot(t *testing.T) {
	for name, finish := range map[string]func(*testing.T, client.Client, types.NamespacedName){
		"succeeded": completeOwnBuild,
		"failed":    failOwnBuild,
	} {
		t.Run(name, func(t *testing.T) {
			app := activeApp("tea-finished-build")
			app.UID = "uid-web" // the fake client assigns none; build admission is keyed on it
			r, cl, nn := lifecycleFixture(t, app)
			prior := buildingReleaseTwo(t, r, cl, nn)
			finish(t, cl, nn)
			if err := cl.Create(context.Background(), buildSlotTaken(nn.Namespace)); err != nil {
				t.Fatal(err)
			}

			_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}) // a failed build answers its error
			live := liveApp(t, cl, nn)
			if ready := findReadyCondition(&live); ready != nil && ready.Reason == reasonBuildQueued {
				t.Fatalf("the finished build waits for a build slot: %s", ready.Message)
			}
			if name == "failed" {
				if verdict := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild); verdict == nil ||
					!appv1alpha1.IsBuildFailureReason(verdict.Reason) || verdict.ObservedGeneration != 2 {
					t.Fatalf("Build condition = %+v, want release 2's failure recorded", verdict)
				}
				return
			}
			built := live.Status.ArtifactImage
			if got := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0].Image; built == prior || got != built {
				t.Fatalf("the pod template runs %q, want release 2's built image, not release 1's %q", got, prior)
			}
		})
	}
}

// TestABuildSuccessPassStoresItsArtifactFirst (w5/108): the pass that finds a
// build succeeded used to keep the artifact in memory until its first status
// write, the release's config snapshot. When that write lost a race, the next
// pass found a finished build again and metered it a second time. The pass now
// stores the artifact first, so the next one reuses it, and meters only once
// that write holds, so a lost race on the artifact write itself counts the
// build once too.
func TestABuildSuccessPassStoresItsArtifactFirst(t *testing.T) {
	for name, conflictsOn := range map[string]func(written *appv1alpha1.App, releaseOne string) bool{
		"the snapshot write": func(written *appv1alpha1.App, _ string) bool {
			return written.Status.ConfigSnapshotGeneration == 2
		},
		"the artifact write": func(written *appv1alpha1.App, releaseOne string) bool {
			return written.Status.ArtifactImage != releaseOne
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := activeApp("tea-build-record")
			app.UID = "uid-web"
			armed, releaseOne, conflicts := false, "", 0
			r, cl, nn := lifecycleFixture(t, app, withInterceptor(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if written, ok := obj.(*appv1alpha1.App); ok && sub == "status" && armed && conflicts == 0 && conflictsOn(written, releaseOne) {
						conflicts++
						return conflictOn("apps", obj.GetName())
					}
					return c.SubResource(sub).Update(ctx, obj, opts...)
				},
			}))
			releaseOne = buildingReleaseTwo(t, r, cl, nn)
			completeOwnBuild(t, cl, nn)
			succeeded := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeSucceeded))
			armed = true

			if err := reconcileUntilConflicted(t, r, nn, &conflicts); !apierrors.IsConflict(err) {
				t.Fatalf("the success pass returned %v, want the conflict", err)
			}
			reconcileOnce(t, r, nn)

			if got := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeSucceeded)) - succeeded; got != 1 {
				t.Fatalf("the build was metered %v times, want once", got)
			}
			built := liveApp(t, cl, nn).Status.ArtifactImage
			if got := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0].Image; built == releaseOne || got != built {
				t.Fatalf("the pod template runs %q, want release 2's built image %q", got, built)
			}
		})
	}
}

// firstReadStale serves a pass's first App read from stale, as an informer
// that has not yet observed the previous pass's write, and every later read
// from the store, as one that caught up mid-pass.
type firstReadStale struct {
	client.Client
	stale *appv1alpha1.App
}

func (c *firstReadStale) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if app, ok := obj.(*appv1alpha1.App); ok && c.stale != nil {
		c.stale.DeepCopyInto(app)
		c.stale = nil
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// TestAPassBehindTheArtifactWriteMetersNothing (w5/108): a pass stores the
// artifact, then its snapshot write conflicts, and the requeued pass reads the
// App from a cache that has not seen the artifact yet. It enters the success
// branch again, and once its cache catches up a write skipped as unchanged
// would let it meter the build a second time. The artifact write conflicts
// instead.
func TestAPassBehindTheArtifactWriteMetersNothing(t *testing.T) {
	app := activeApp("tea-build-stale")
	app.UID = "uid-web"
	armed, conflicts := false, 0
	r, cl, nn := lifecycleFixture(t, app, withInterceptor(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if written, ok := obj.(*appv1alpha1.App); ok && sub == "status" && armed && conflicts == 0 && written.Status.ConfigSnapshotGeneration == 2 {
				conflicts++
				return conflictOn("apps", obj.GetName())
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	}))
	buildingReleaseTwo(t, r, cl, nn)
	completeOwnBuild(t, cl, nn)
	stale := liveApp(t, cl, nn)
	succeeded := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeSucceeded))
	armed = true
	if err := reconcileUntilConflicted(t, r, nn, &conflicts); !apierrors.IsConflict(err) {
		t.Fatalf("the success pass returned %v, want its snapshot write's conflict", err)
	}

	store := r.Client
	r.Client = &firstReadStale{Client: store, stale: &stale}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); !apierrors.IsConflict(err) {
		t.Fatalf("the pass behind the artifact write returned %v, want its artifact write's conflict", err)
	}
	r.Client = store
	reconcileOnce(t, r, nn)

	if got := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeSucceeded)) - succeeded; got != 1 {
		t.Fatalf("the build was metered %v times, want once", got)
	}
}

// TestAStoredBuildPrunesEarlierReleasesKpackArtifacts (w5/129): every
// buildpack release builds its own kpack Image. The pass that stores release
// 2's artifact deletes the Image release 1 left and keeps its own, so leftover
// Images stop growing with release history.
func TestAStoredBuildPrunesEarlierReleasesKpackArtifacts(t *testing.T) {
	ctx := context.Background()
	app := activeApp("tea-pruned-build")
	app.UID = "uid-web" // the fake client assigns none; build artifacts are keyed on it
	r, cl, nn := lifecycleFixture(t, app)
	serveReleaseOne(t, r, cl, nn)
	earlier := releaseKpackImage(r, nn, 1)
	markKpackImageBuilt(earlier)
	if err := cl.Create(ctx, earlier); err != nil {
		t.Fatal(err)
	}
	releaseTwoFromSource(t, cl, nn)
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Builder = build.BuilderBuildpack })
	reconcileOnce(t, r, nn)
	own := dispatchedKpackImage(t, r, cl, nn, 2)
	markKpackImageBuilt(own)
	if err := cl.Update(ctx, own); err != nil {
		t.Fatal(err)
	}

	reconcileOnce(t, r, nn)
	if live := liveApp(t, cl, nn); live.Status.ArtifactImage == "" {
		t.Fatal("release 2's artifact was not stored")
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(earlier), earlier.DeepCopy()); !apierrors.IsNotFound(err) {
		t.Fatalf("release 1's kpack Image outlived release 2's stored build: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(own), own.DeepCopy()); err != nil {
		t.Fatalf("release 2's own kpack Image did not survive its stored build: %v", err)
	}
}
