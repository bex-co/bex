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
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var (
	digestA = "sha256:" + strings.Repeat("a", 64)
	digestB = "sha256:" + strings.Repeat("b", 64)
)

func TestImageDigestFromRef(t *testing.T) {
	for _, tc := range []struct{ ref, want string }{
		{"docker.io/ealen/echo-server@" + digestA, digestA},
		{"docker.io/ealen/echo-server:0.9.2@" + digestA, digestA},
		{"docker-pullable://docker.io/x/y@" + digestA, digestA},
		{"zot.bex-registry.svc:5000/tea-1/web:gen-3@" + digestB, digestB},
		// A tag pins nothing.
		{"docker.io/ealen/echo-server:0.9.2", ""},
		// A bare imageID is a local image's config id, not a manifest digest.
		{digestA, ""},
		{"docker.io/x/y@sha256:short", ""},
		{"docker.io/x/y@sha512:" + strings.Repeat("a", 128), ""},
		{"", ""},
	} {
		if got := imageDigestFromRef(tc.ref); got != tc.want {
			t.Errorf("imageDigestFromRef(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

// digestPod is a pod of revision rev whose app container the kubelet reports
// running imageID.
func digestPod(name, rev string, ready bool, created time.Time, imageID string) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "default", CreationTimestamp: metav1.NewTime(created),
			Labels: map[string]string{"app": "web", labelRevision: rev},
		},
		Status: corev1.PodStatus{
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: appContainerName, ImageID: imageID}},
		},
	}
}

// TestRecordImageDigest pins where status.imageDigest comes from and that it
// belongs to one release generation only.
func TestRecordImageDigest(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web", labelRevision: "rev-2"}}},
		},
	}
	now := time.Now()
	terminating := digestPod("web-gone", "rev-2", true, now.Add(-time.Hour), "docker.io/x/y@"+digestB)
	terminating.DeletionTimestamp = &metav1.Time{Time: now}
	terminating.Finalizers = []string{"test/hold"}
	sidecar := digestPod("web-sidecar", "rev-2", true, now.Add(-time.Hour), "docker.io/x/y@"+digestB)
	sidecar.Status.ContainerStatuses[0].Name = "sidecar"

	tests := []struct {
		name  string
		image string
		prior *appv1alpha1.ImageDigestStatus
		pods  []client.Object
		dep   *appsv1.Deployment
		want  *appv1alpha1.ImageDigestStatus
	}{{
		name:  "tag: the Ready current-revision pod's imageID",
		image: "docker.io/x/y:latest",
		pods:  []client.Object{digestPod("web-1", "rev-2", true, now, "docker.io/x/y@"+digestA)},
		dep:   dep,
		want:  &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestA},
	}, {
		name:  "tag: the earliest-created Ready pod wins when the tag moved mid-rollout",
		image: "docker.io/x/y:latest",
		pods: []client.Object{
			// Listed first by name, created last.
			digestPod("web-a", "rev-2", true, now, "docker.io/x/y@"+digestB),
			digestPod("web-z", "rev-2", true, now.Add(-time.Minute), "docker.io/x/y@"+digestA),
		},
		dep:  dep,
		want: &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestA},
	}, {
		name:  "tag: not-ready, terminating, older-revision, non-app and config-id pods are no evidence",
		image: "docker.io/x/y:latest",
		pods: []client.Object{
			digestPod("web-starting", "rev-2", false, now, "docker.io/x/y@"+digestB),
			terminating,
			digestPod("web-old", "rev-1", true, now, "docker.io/x/y@"+digestB),
			sidecar,
			digestPod("web-local", "rev-2", true, now, digestB),
		},
		dep: dep,
	}, {
		name:  "digest-pinned image is its own evidence, no pod read",
		image: "zot.bex-registry.svc:5000/tea-1/web:gen-2@" + digestB,
		dep:   nil,
		want:  &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestB},
	}, {
		name:  "a tag with no pod to read (cron) records nothing",
		image: "docker.io/x/y:latest",
		dep:   nil,
	}, {
		name:  "the generation's first digest stays when a pod later reports another",
		image: "docker.io/x/y:latest",
		prior: &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestA},
		pods:  []client.Object{digestPod("web-1", "rev-2", true, now, "docker.io/x/y@"+digestB)},
		dep:   dep,
		want:  &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestA},
	}, {
		name:  "an earlier release's digest is dropped, never carried forward",
		image: "docker.io/x/y:latest",
		prior: &appv1alpha1.ImageDigestStatus{Generation: 1, Digest: digestA},
		dep:   dep,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{Client: fake.NewClientBuilder().WithObjects(tc.pods...).Build()}
			app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{
				ReleaseGeneration: 2, Image: tc.image, ImageDigest: tc.prior,
			}}
			r.recordImageDigest(context.Background(), app, tc.dep)
			got := app.Status.ImageDigest
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("imageDigest = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// addReadyPod reports one Ready pod of the Deployment's current template whose
// app container runs imageID.
func addReadyPod(t *testing.T, cl client.Client, nn types.NamespacedName, name, imageID string) {
	t.Helper()
	tmpl := deploymentTemplate(t, cl, nn)
	// The fixture registers no Pod status subresource, so Create keeps status.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace, Labels: tmpl.Labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "unused"}}},
		Status: corev1.PodStatus{
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: appContainerName, ImageID: imageID}},
		},
	}
	if err := cl.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

// TestServingReleaseRecordsItsOwnImageDigest drives the web path: each release
// records the digest its own pods run once it serves, and a release that never
// serves leaves the previous release's digest attributed to that release.
func TestServingReleaseRecordsItsOwnImageDigest(t *testing.T) {
	app := activeApp(defaultAppsNamespace)
	app.Spec.Image = "docker.io/ealen/echo-server:latest"
	r, cl, nn := lifecycleFixture(t, app)

	reconcileTwice(t, r, nn)
	addReadyPod(t, cl, nn, "web-r1", "docker.io/ealen/echo-server@"+digestA)
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	if live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("setup: release 1 phase %q, want Running", live.Status.Phase)
	}
	want1 := appv1alpha1.ImageDigestStatus{Generation: 1, Digest: digestA}
	if got := live.Status.ImageDigest; got == nil || *got != want1 {
		t.Fatalf("release 1 imageDigest = %+v, want %+v", got, want1)
	}

	// Release 2 rolls but never serves: only release 1's pod is ready.
	deployImageAt(t, cl, nn, "docker.io/ealen/echo-server:0.9.2", 2)
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn)
	live = liveApp(t, cl, nn)
	if live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("setup: activeRevision %q, want rev-1 while release 2 rolls", live.Status.ActiveRevision)
	}
	if got := live.Status.ImageDigest; got == nil || *got != want1 {
		t.Fatalf("while release 2 rolls, imageDigest = %+v, want release 1's %+v", got, want1)
	}

	// Release 2 serves with its own bytes.
	addReadyPod(t, cl, nn, "web-r2", "docker.io/ealen/echo-server@"+digestB)
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live = liveApp(t, cl, nn)
	want2 := appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestB}
	if live.Status.ActiveRevision != "rev-2" {
		t.Fatalf("activeRevision %q, want rev-2", live.Status.ActiveRevision)
	}
	if got := live.Status.ImageDigest; got == nil || *got != want2 {
		t.Fatalf("release 2 imageDigest = %+v, want %+v", got, want2)
	}
}

// TestWorkerReleaseRecordsImageDigest: a background worker serves through the
// same markRunning, so it records its digest the same way.
func TestWorkerReleaseRecordsImageDigest(t *testing.T) {
	app := heldWorkerApp(defaultAppsNamespace)
	app.Spec.Image = "docker.io/library/busybox:latest"
	r, cl, nn := lifecycleFixture(t, app)

	reconcileTwice(t, r, nn)
	addReadyPod(t, cl, nn, "worker-r1", "docker.io/library/busybox@"+digestA)
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	want := appv1alpha1.ImageDigestStatus{Generation: 1, Digest: digestA}
	if got := live.Status.ImageDigest; live.Status.Phase != appv1alpha1.PhaseRunning || got == nil || *got != want {
		t.Fatalf("phase %q imageDigest %+v, want Running with %+v", live.Status.Phase, got, want)
	}
}
