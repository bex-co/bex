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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPreDeployFailsFastOnAnUnpullableImage (w8/m44): live, a nonexistent tag
// held a pre-deploy Job for its whole 10-minute deadline and then closed the
// deploy as "the pre-deploy command did not finish … check the pre-deploy
// logs". Once the pod has waited on the pull past the grace window, the step
// fails with the kubelet's pull error and the Job is deleted, so a pull that
// succeeds later can never run the migration after the verdict.
func TestPreDeployFailsFastOnAnUnpullableImage(t *testing.T) {
	ctx := context.Background()
	const image = "docker.io/mendhak/http-https-echo:does-not-exist-999"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 2, UID: "app-uid-1"},
		Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000, PreDeployCommand: "migrate"},
		Status:     appv1alpha1.AppStatus{ReleaseGeneration: 2},
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme, Mode: ModeKubernetes}

	// First pass creates the Job; its pod is then stuck on the pull.
	if _, halt, err := r.reconcilePreDeploy(ctx, app, image, 3000); err != nil || !halt {
		t.Fatalf("first pass: halt=%v err=%v", halt, err)
	}
	var job batchv1.Job
	jobKey := client.ObjectKey{Namespace: "default", Name: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(2))}
	if err := cl.Get(ctx, jobKey, &job); err != nil {
		t.Fatalf("pre-deploy Job: %v", err)
	}
	job.UID = "job-uid-2"
	if err := cl.Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.Name + "-abcd", Namespace: "default",
			Labels:            map[string]string{batchv1.ControllerUidLabel: "job-uid-2"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Second)),
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "predeploy",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason:  "ImagePullBackOff",
				Message: `Back-off pulling image "` + image + `": not found`,
			}},
		}}},
	}
	if err := cl.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}

	// Inside the grace window the step keeps running: a first ErrImagePull can
	// be a registry blip.
	if _, halt, err := r.reconcilePreDeploy(ctx, app, image, 3000); err != nil || !halt ||
		app.Status.PreDeploy.Status != appv1alpha1.PreDeployRunning {
		t.Fatalf("inside grace: halt=%v err=%v preDeploy=%+v", halt, err, app.Status.PreDeploy)
	}

	pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Minute))
	if err := cl.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	_, halt, _ := r.reconcilePreDeploy(ctx, app, image, 3000)
	pd := app.Status.PreDeploy
	if !halt || pd == nil || pd.Status != appv1alpha1.PreDeployFailed {
		t.Fatalf("past grace: halt=%v preDeploy=%+v, want Failed", halt, pd)
	}
	if !strings.HasPrefix(pd.Message, "image pull is failing: Back-off pulling image") ||
		strings.Contains(pd.Message, "check the pre-deploy logs") || strings.Contains(pd.Message, "did not finish") {
		t.Errorf("message = %q", pd.Message)
	}
	if err := cl.Get(ctx, jobKey, &job); !apierrors.IsNotFound(err) {
		t.Errorf("the stuck Job must be deleted so a late pull cannot run the migration; get err = %v", err)
	}
}

func TestInvalidImageNamePreDeployFailsFirstRelease(t *testing.T) {
	app := preDeployApp("", "")
	_, stored := failedInvalidImagePreDeploy(t, app)
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("first release phase = %q", stored.Status.Phase)
	}
}

func TestInvalidImageNamePreDeployPreservesServingRelease(t *testing.T) {
	app := preDeployApp("docker.io/team/app:good", "rev-2")
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec: appsv1.DeploymentSpec{Replicas: new(int32(1)), Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: app.Status.Image}}},
		}},
	}
	cl, stored := failedInvalidImagePreDeploy(t, app, dep)
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if stored.Status.Phase != appv1alpha1.PhaseRunning || stored.Status.ActiveRevision != "rev-2" ||
		stored.Status.Image != "docker.io/team/app:good" || stored.Status.ObservedGeneration != 2 ||
		ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != appv1alpha1.ReasonPriorReleaseServing {
		t.Fatalf("serving release changed: %+v", stored.Status)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), dep); err != nil {
		t.Fatal(err)
	}
	if *dep.Spec.Replicas != 1 || dep.Spec.Template.Spec.Containers[0].Image != "docker.io/team/app:good" {
		t.Fatalf("serving Deployment changed: %+v", dep.Spec)
	}
}

func failedInvalidImagePreDeploy(t *testing.T, app *appv1alpha1.App, objects ...client.Object) (client.Client, appv1alpha1.App) {
	t.Helper()
	const image = "docker.io/team/APP:latest"
	const message = "invalid reference format: repository name must be lowercase"
	ctx := context.Background()
	app.UID = "app-uid-1"
	app.Spec.Image = image
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(append(objects, app)...).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
	if _, halt, err := r.reconcilePreDeploy(ctx, app, image, 3000); err != nil || !halt {
		t.Fatalf("create step: halt=%v err=%v", halt, err)
	}
	var job batchv1.Job
	jobKey := client.ObjectKey{Namespace: app.Namespace, Name: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(3))}
	if err := cl.Get(ctx, jobKey, &job); err != nil {
		t.Fatal(err)
	}
	job.UID = "job-uid-3"
	if err := cl.Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-old", Namespace: app.Namespace,
			Labels: map[string]string{batchv1.ControllerUidLabel: "earlier-job-uid"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "predeploy",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "InvalidImageName", Message: message}},
		}}},
	}
	if err := cl.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if _, halt, err := r.reconcilePreDeploy(ctx, app, image, 3000); err != nil || !halt || app.Status.PreDeploy.Status != appv1alpha1.PreDeployRunning {
		t.Fatalf("earlier Job affected the step: halt=%v err=%v status=%+v", halt, err, app.Status.PreDeploy)
	}
	pod.Labels[batchv1.ControllerUidLabel] = string(job.UID)
	if err := cl.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	_, halt, err := r.reconcilePreDeploy(ctx, app, image, 3000)
	want := "image reference is invalid: " + message + "; the pre-deploy command never ran"
	stored := storedApp(t, cl, app)
	pd := stored.Status.PreDeploy
	if !halt || err == nil || err.Error() != want || pd == nil || pd.Status != appv1alpha1.PreDeployFailed || pd.Message != want || pd.Generation != 3 {
		t.Fatalf("invalid image: halt=%v err=%v preDeploy=%+v", halt, err, pd)
	}
	if err := cl.Get(ctx, jobKey, &job); !apierrors.IsNotFound(err) {
		t.Fatalf("failed step's Job still exists: %v", err)
	}
	return cl, stored
}
