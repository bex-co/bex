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

package predeploy

import (
	"context"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// w1/m149: a failed pre-deploy command is explained by its exit code, OOM kill,
// or deadline. At filing time an immediate `exit 3` closed its deploy with
// "BackoffLimitExceeded: Job has reached the specified backoff limit".

const jobUID = types.UID("0b7d2c1e-job-gen-3")

func failedJob(reason string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "predeploy-web-gen-3", Namespace: "tea-ws", UID: jobUID},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
			Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: reason,
			Message: "Job has reached the specified backoff limit",
		}}},
	}
}

func jobPod(uid types.UID, namespace string, state corev1.ContainerState) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "predeploy-web-gen-3-" + string(uid)[:4], Namespace: namespace,
			Labels: map[string]string{batchv1.ControllerUidLabel: string(uid)},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: containerName, State: state}}},
	}
}

func terminated(code int32, reason string) corev1.ContainerState {
	return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: code, Reason: reason}}
}

func TestFailureMessageNamesTheExitCode(t *testing.T) {
	job := failedJob("BackoffLimitExceeded")
	cl := fakeClient(job, jobPod(jobUID, job.Namespace, terminated(3, "Error")))

	got := FailureMessage(context.Background(), cl, job)
	if want := "the pre-deploy command exited with code 3; check the pre-deploy logs"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if strings.Contains(got, "BackoffLimitExceeded") {
		t.Fatalf("message %q leaks the Job controller's condition", got)
	}
}

func TestFailureMessageOutOfMemory(t *testing.T) {
	job := failedJob("BackoffLimitExceeded")
	cl := fakeClient(job, jobPod(jobUID, job.Namespace, terminated(137, "OOMKilled")))

	if got, want := FailureMessage(context.Background(), cl, job),
		"the pre-deploy command ran out of memory (exit code 137); check the pre-deploy logs"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// A real timeout keeps the timeout wording, with its window named.
func TestFailureMessageDeadlineNamesTheWindow(t *testing.T) {
	job := failedJob(batchv1.JobReasonDeadlineExceeded)
	cl := fakeClient(job, jobPod(jobUID, job.Namespace, terminated(137, "Error")))

	if got, want := FailureMessage(context.Background(), cl, job),
		"the pre-deploy command did not finish within its 10-minute window; check the pre-deploy logs"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// With the pod gone (TTL-reaped), never started, or belonging to another Job —
// including an earlier Job of the same name — there is no exit code to report;
// the message still never falls back to Kubernetes text.
func TestFailureMessageWithoutATerminatedPod(t *testing.T) {
	job := failedJob("BackoffLimitExceeded")
	waiting := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}
	for name, cl := range map[string]client.Client{
		"no pod":                       fakeClient(job),
		"waiting":                      fakeClient(job, jobPod(jobUID, job.Namespace, waiting)),
		"other namespace":              fakeClient(job, jobPod(jobUID, "bex-build", terminated(3, "Error"))),
		"same-named Job's earlier pod": fakeClient(job, jobPod("9f1e-earlier-run", job.Namespace, terminated(3, "Error"))),
	} {
		if got, want := FailureMessage(context.Background(), cl, job), "the pre-deploy command failed; check the pre-deploy logs"; got != want {
			t.Errorf("%s: message = %q, want %q", name, got, want)
		}
	}
}

// pullWaiting is the kubelet's wait on an image that does not exist.
func pullWaiting(reason string) corev1.ContainerState {
	return corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason,
		Message: `rpc error: code = NotFound desc = failed to pull and unpack image "docker.io/x/y:nope": not found`}}
}

// w8/m44: a pre-deploy Job whose image cannot be pulled failed only when its
// 10-minute deadline ran out, as "did not finish … check the pre-deploy logs"
// for a command that never started. A failed Job whose pod is stuck on the
// pull names the pull instead, whatever the Job's own reason.
func TestFailureMessageNamesAFailingImagePull(t *testing.T) {
	for _, reason := range []string{"ImagePullBackOff", "ErrImagePull"} {
		job := failedJob(batchv1.JobReasonDeadlineExceeded)
		got := FailureMessage(context.Background(), fakeClient(job, jobPod(jobUID, job.Namespace, pullWaiting(reason))), job)
		if !strings.HasPrefix(got, "image pull is failing: rpc error") || !strings.Contains(got, "never ran") ||
			strings.Contains(got, "check the pre-deploy logs") || strings.Contains(got, "did not finish") {
			t.Errorf("%s: message = %q", reason, got)
		}
	}
}

// PullFailure fails a pending Job fast once its pod has waited on a failing
// pull past the grace window, and never a pod that is pulling slowly, one
// still inside the window, or another Job's pod.
func TestPullFailureAfterGrace(t *testing.T) {
	job := failedJob("")
	job.Status.Conditions = nil
	now := time.Date(2026, 9, 26, 7, 8, 0, 0, time.UTC)
	pod := func(state corev1.ContainerState, age time.Duration, uid types.UID) *corev1.Pod {
		p := jobPod(uid, job.Namespace, state)
		p.CreationTimestamp = metav1.NewTime(now.Add(-age))
		return p
	}
	for name, tc := range map[string]struct {
		pod  *corev1.Pod
		fail bool
	}{
		"backoff past grace":        {pod(pullWaiting("ImagePullBackOff"), 2*time.Minute, jobUID), true},
		"err pull past grace":       {pod(pullWaiting("ErrImagePull"), 91*time.Second, jobUID), true},
		"backoff inside grace":      {pod(pullWaiting("ImagePullBackOff"), 30*time.Second, jobUID), false},
		"slow pull, still creating": {pod(corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}, 5*time.Minute, jobUID), false},
		"running":                   {pod(corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, 5*time.Minute, jobUID), false},
		"another Job's pod":         {pod(pullWaiting("ImagePullBackOff"), 5*time.Minute, "9f1e-earlier-run"), false},
	} {
		got := PullFailure(context.Background(), fakeClient(job, tc.pod), job, now)
		if (got != "") != tc.fail {
			t.Errorf("%s: PullFailure = %q, want fail=%v", name, got, tc.fail)
		}
		if tc.fail && !strings.HasPrefix(got, "image pull is failing: ") {
			t.Errorf("%s: message = %q", name, got)
		}
	}
}
