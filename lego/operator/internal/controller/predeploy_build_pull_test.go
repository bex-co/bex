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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Git-built service's pre-deploy step and initial deploy hook run its build
// from the platform registry. A pull of that build that can never succeed ends
// the run with the rollout's product sentence, never the kubelet's line with
// the registry host, repository path and digest.

const buildPullRegistry = "zot.bex-registry.svc:5000"

var buildPullImage = buildPullRegistry + "/tea-ws/tea-ws-web@sha256:" + strings.Repeat("0123abcd", 8)

// buildGoneKubelet is the kubelet's answer for a build retention removed.
var buildGoneKubelet = `Failed to pull image "` + buildPullImage + `": rpc error: code = NotFound desc = failed to pull and unpack image "` +
	buildPullImage + `": failed to resolve reference "` + buildPullImage + `": ` + buildPullImage + `: not found`

const buildGoneMessage = "image pull is failing: the build this release runs on (tea-ws/tea-ws-web) is no longer stored," +
	" so its image cannot be pulled; the pre-deploy command never ran." +
	" Only a service's recent builds are kept. Deploy the commit again to build it."

// buildPullPod is jobUID's pod, past the pull grace window, waiting on the build.
func buildPullPod(job *batchv1.Job, jobUID types.UID) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.Name + "-abcd", Namespace: job.Namespace,
			Labels:            map[string]string{batchv1.ControllerUidLabel: string(jobUID)},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "predeploy", Image: buildPullImage,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull", Message: buildGoneKubelet}},
		}}},
	}
}

func TestPreDeployBuildPullFailureReadsInProductTerms(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-ws", Generation: 2, UID: "app-uid-1"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/acme/web", Port: 3000, PreDeployCommand: "./migrate"},
		Status: appv1alpha1.AppStatus{
			ReleaseGeneration: 2,
			// Read already: the step never reads the registry for its commit.
			ImageRevision: &appv1alpha1.ImageRevisionStatus{Image: buildPullImage},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: buildPullRegistry}

	if _, halt, err := r.reconcilePreDeploy(ctx, app, buildPullImage, 3000); err != nil || !halt {
		t.Fatalf("first pass: halt=%v err=%v", halt, err)
	}
	var job batchv1.Job
	jobKey := client.ObjectKey{Namespace: app.Namespace, Name: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(2))}
	if err := cl.Get(ctx, jobKey, &job); err != nil {
		t.Fatalf("pre-deploy Job: %v", err)
	}
	job.UID = "job-uid-2"
	if err := cl.Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	if err := cl.Create(ctx, buildPullPod(&job, job.UID)); err != nil {
		t.Fatal(err)
	}

	if _, halt, _ := r.reconcilePreDeploy(ctx, app, buildPullImage, 3000); !halt {
		t.Fatal("a step whose build cannot be pulled did not hold the rollout")
	}
	stored := storedApp(t, cl, app)
	if pd := stored.Status.PreDeploy; pd == nil || pd.Status != appv1alpha1.PreDeployFailed || pd.Message != buildGoneMessage {
		t.Fatalf("status.preDeploy = %+v, want Failed with %q", pd, buildGoneMessage)
	}
	if ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady); ready == nil || ready.Message != buildGoneMessage {
		t.Fatalf("Ready = %+v, want %q", ready, buildGoneMessage)
	}
	if err := cl.Get(ctx, jobKey, &job); !apierrors.IsNotFound(err) {
		t.Errorf("the stuck Job must be deleted; get err = %v", err)
	}
}

func TestInitialDeployHookBuildPullFailureReadsInProductTerms(t *testing.T) {
	ctx := context.Background()
	jobName := predeploy.HookJobName("web", appv1alpha1.BuildRevision(1))
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-ws", Generation: 1, UID: "app-uid-1"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/acme/web", Port: 3000, InitialDeployHook: "./seed"},
		Status: appv1alpha1.AppStatus{InitialDeployHook: &appv1alpha1.PreDeployStatus{
			Job: jobName, Generation: 1, Status: appv1alpha1.PreDeployRunning, StartedAt: time.Now().UTC().Format(time.RFC3339),
		}},
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: jobName, Namespace: app.Namespace, UID: "hook-job-uid",
		Labels: map[string]string{predeploy.LabelStep: string(predeploy.StepInitialDeployHook)},
	}}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app, job, buildPullPod(job, "hook-job-uid")).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: buildPullRegistry}

	if r.observeInitialDeployHook(ctx, app) {
		t.Fatal("a hook whose build cannot be pulled is still running")
	}
	want := strings.ReplaceAll(buildGoneMessage, "the pre-deploy command", "the initial deploy hook")
	hook := storedApp(t, cl, app).Status.InitialDeployHook
	if hook == nil || hook.Status != appv1alpha1.PreDeployFailed || hook.Message != want {
		t.Fatalf("status.initialDeployHook = %+v, want Failed with %q", hook, want)
	}
	if !strings.Contains(hook.Message, "the initial deploy hook never ran") {
		t.Fatalf("message %q does not name the initial deploy hook", hook.Message)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(job), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Errorf("the stuck hook Job must be deleted; get err = %v", err)
	}
}
