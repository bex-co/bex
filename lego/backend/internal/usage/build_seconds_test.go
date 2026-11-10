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

package usage

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m307: build_seconds counts what Render counts as pipeline minutes — the
// build and the pre-deploy command, succeeded or failed, from when the pod
// started — and never the platform's own failures or node wait.

var meterWindow = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func meterTime(minutes int) *metav1.Time {
	t := metav1.NewTime(meterWindow.Add(time.Duration(minutes) * time.Minute))
	return &t
}

// meterJob is a Job that started at startMin and either completed or failed at
// endMin (minutes into the window).
func meterJob(name, namespace string, labels map[string]string, startMin, endMin int, failed bool) *batchv1.Job {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
	job.Status.StartTime = meterTime(startMin)
	if failed {
		job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: *meterTime(endMin)}}
	} else {
		job.Status.CompletionTime = meterTime(endMin)
		job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	}
	return job
}

func meterService(t *testing.T, objs ...client.Object) *Service {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, BuildNamespace: "bex-build"}
}

func buildLabels(revision string) map[string]string {
	return map[string]string{appv1alpha1.LabelBuild: "web", appv1alpha1.LabelBuildRevision: revision}
}

func TestBuildSecondsCountsFailedBuildsButNotThePlatformsFailures(t *testing.T) {
	tenantFailed := meterJob("bld-tenant", "bex-build", buildLabels("gen-1"), 0, 4, true)
	tenantFailed.Annotations = map[string]string{appv1alpha1.AnnotationBuildFault: appv1alpha1.BuildFaultTenant}
	infraFailed := meterJob("bld-infra", "bex-build", buildLabels("gen-2"), 10, 30, true)
	infraFailed.Annotations = map[string]string{appv1alpha1.AnnotationBuildFault: appv1alpha1.BuildFaultInfra}
	unstamped := meterJob("bld-old", "bex-build", buildLabels("gen-3"), 40, 42, true)
	unstamped.Status.Conditions[0].Reason = "PodFailurePolicy" // the tenant's exit, by the Job's own reason
	// No stamp, but the Job used up its retries: the platform's failure.
	retriesSpent := meterJob("bld-retries", "bex-build", buildLabels("gen-5"), 43, 48, true)
	retriesSpent.Status.Conditions[0].Reason = "BackoffLimitExceeded"
	running := meterJob("bld-running", "bex-build", buildLabels("gen-4"), 50, 0, false)
	running.Status.CompletionTime, running.Status.Conditions = nil, nil

	svc := meterService(t, tenantFailed, infraFailed, unstamped, retriesSpent, running)
	secs, ok := svc.queryBuildSeconds(context.Background(), "web", "tea-a", meterWindow, meterWindow.Add(time.Hour), nil)
	if !ok {
		t.Fatal("listing failed")
	}
	if want := int64((4 + 2) * 60); secs != want {
		t.Errorf("build_seconds = %d, want %d: tenant failures bill; a platform failure (stamped or by reason) and a running build do not", secs, want)
	}
}

func TestBuildSecondsStartsWhenTheBuildPodWasPlaced(t *testing.T) {
	job := meterJob("bld-web", "bex-build", buildLabels("gen-1"), 0, 20, false)
	// It waited 5 minutes for a build node: the platform's time.
	job.Annotations = map[string]string{appv1alpha1.AnnotationBuildPlacedAt: meterTime(5).UTC().Format(time.RFC3339)}
	bogus := meterJob("bld-bogus", "bex-build", buildLabels("gen-2"), 30, 40, false)
	bogus.Annotations = map[string]string{appv1alpha1.AnnotationBuildPlacedAt: meterTime(50).UTC().Format(time.RFC3339)}

	svc := meterService(t, job, bogus)
	secs, ok := svc.queryBuildSeconds(context.Background(), "web", "tea-a", meterWindow, meterWindow.Add(time.Hour), nil)
	if !ok {
		t.Fatal("listing failed")
	}
	if want := int64((15 + 10) * 60); secs != want {
		t.Errorf("build_seconds = %d, want %d: from placement, and a placement after the end falls back to startTime", secs, want)
	}
}

func TestBuildSecondsCountsPreDeployRunsInTheAppsNamespace(t *testing.T) {
	predeploy := func(name, revision string, startMin, endMin int, failed bool) *batchv1.Job {
		return meterJob(name, "tea-a", map[string]string{
			core.PodLabelPreDeploy: "web", appv1alpha1.LabelBuildRevision: revision,
		}, startMin, endMin, failed)
	}
	build := meterJob("bld-web", "bex-build", buildLabels("gen-1"), 0, 10, false)
	migrate := predeploy("predeploy-web-gen-1", "gen-1", 11, 13, false)
	failedMigrate := predeploy("predeploy-web-gen-2", "gen-2", 20, 21, true)
	// A pre-deploy Job has no pod failure policy: its BackoffLimitExceeded is
	// the tenant's failed migration, not the platform's.
	failedMigrate.Status.Conditions[0].Reason = "BackoffLimitExceeded"
	platform := predeploy("predeploy-web-gen-3", "gen-3", 30, 40, false)
	otherApp := meterJob("predeploy-api-gen-1", "tea-a", map[string]string{core.PodLabelPreDeploy: "api"}, 0, 50, false)

	svc := meterService(t, build, migrate, failedMigrate, platform, otherApp)
	secs, ok := svc.queryBuildSeconds(context.Background(), "web", "tea-a", meterWindow, meterWindow.Add(time.Hour), map[string]bool{"gen-3": true})
	if !ok {
		t.Fatal("listing failed")
	}
	if want := int64((10 + 2 + 1) * 60); secs != want {
		t.Errorf("build_seconds = %d, want %d: the build plus both pre-deploy runs, not the platform's or another App's", secs, want)
	}
}

func kpackBuild(name, revision string, created time.Time, status string, endedAt time.Time) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(appv1alpha1.KpackGroupVersion.WithKind("Build"))
	u.SetNamespace("bex-build")
	u.SetName(name)
	u.SetLabels(buildLabels(revision)) // kpack copies the Image's labels onto its Builds
	u.SetCreationTimestamp(metav1.NewTime(created))
	_ = unstructured.SetNestedSlice(u.Object, []any{map[string]any{
		"type": "Succeeded", "status": status, "lastTransitionTime": endedAt.UTC().Format(time.RFC3339),
	}}, "status", "conditions")
	return u
}

func TestBuildSecondsCountsBuildpackBuilds(t *testing.T) {
	ok1 := kpackBuild("bld-web-gen-1-abc-build-1", "gen-1", meterTime(0).Time, "True", meterTime(6).Time)
	_ = unstructured.SetNestedField(ok1.Object, "bld-web-gen-1-abc-build-1-pod", "status", "podName")
	failed := kpackBuild("bld-web-gen-1-abc-build-2", "gen-1", meterTime(10).Time, "False", meterTime(13).Time)
	running := kpackBuild("bld-web-gen-1-abc-build-3", "gen-1", meterTime(20).Time, "Unknown", meterTime(25).Time)
	exempt := kpackBuild("bld-web-gen-2-def-build-1", "gen-2", meterTime(30).Time, "True", meterTime(40).Time)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bld-web-gen-1-abc-build-1-pod", Namespace: "bex-build",
		Labels: map[string]string{appv1alpha1.LabelBuild: "web"}}}
	pod.Status.StartTime = meterTime(2) // two minutes for a node

	svc := meterService(t, ok1, failed, running, exempt, pod)
	secs, ok := svc.queryBuildSeconds(context.Background(), "web", "tea-a", meterWindow, meterWindow.Add(time.Hour), map[string]bool{"gen-2": true})
	if !ok {
		t.Fatal("listing failed")
	}
	if want := int64((4 + 3) * 60); secs != want {
		t.Errorf("build_seconds = %d, want %d: both terminal builds of the tenant's release, from pod start or creation", secs, want)
	}
}

func TestBuildSecondsWithoutKpackInstalled(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	job := meterJob("bld-web", "bex-build", buildLabels("gen-1"), 0, 3, false)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if u, ok := list.(*unstructured.UnstructuredList); ok {
				return &meta.NoKindMatchError{GroupKind: u.GroupVersionKind().GroupKind()}
			}
			return c.List(ctx, list, opts...)
		},
	}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, BuildNamespace: "bex-build"}
	secs, ok := svc.queryBuildSeconds(context.Background(), "web", "tea-a", meterWindow, meterWindow.Add(time.Hour), nil)
	if !ok || secs != 180 {
		t.Errorf("build_seconds = %d (ok=%v), want 180: a cluster without kpack meters its Jobs as before", secs, ok)
	}
}
