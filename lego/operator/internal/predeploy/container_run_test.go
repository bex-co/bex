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
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// w32/m572: the deploy log's `==> Running '<command>'` comes at the step
// container's start and `==> Exited with status N` names its exit status, so
// the operator reads both off the Job's pod. A command that never ran has
// neither, and one the Job's time limit stopped did not exit on its own.
func TestContainerRunReadsTheCommandsStartAndExit(t *testing.T) {
	started := metav1.NewTime(time.Date(2026, 11, 22, 11, 29, 27, 0, time.UTC))
	ended := func(code int32, reason string) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			ExitCode: code, Reason: reason, StartedAt: started, FinishedAt: metav1.NewTime(started.Add(4 * time.Second)),
		}}
	}
	running := &batchv1.Job{ObjectMeta: failedJob("").ObjectMeta}
	code := func(c int32) *int32 { return &c }
	for _, tc := range []struct {
		name  string
		job   *batchv1.Job
		state *corev1.ContainerState
		want  Run
	}{
		{name: "no pod yet", job: running},
		{name: "waiting on its image", job: running, state: &corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
		{name: "running", job: running, state: &corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: started}}, want: Run{StartedAt: started}},
		{name: "exited 1", job: failedJob("BackoffLimitExceeded"), state: new(ended(1, "Error")), want: Run{StartedAt: started, ExitCode: code(1)}},
		{name: "exited 0", job: running, state: new(ended(0, "Completed")), want: Run{StartedAt: started, ExitCode: code(0)}},
		{name: "killed for memory", job: failedJob("BackoffLimitExceeded"), state: new(ended(137, "OOMKilled")),
			want: Run{StartedAt: started, ExitCode: code(137), OOMKilled: true}},
		{name: "stopped at its time limit", job: failedJob(batchv1.JobReasonDeadlineExceeded), state: new(ended(137, "Error")), want: Run{StartedAt: started}},
		{name: "the runtime could not launch its program", job: failedJob("BackoffLimitExceeded"),
			state: &corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 128, Reason: "StartError"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := fakeClient(tc.job)
			if tc.state != nil {
				cl = fakeClient(tc.job, jobPod(jobUID, tc.job.Namespace, *tc.state))
			}
			got := ContainerRun(context.Background(), cl, tc.job)
			if !got.StartedAt.Equal(&tc.want.StartedAt) || got.OOMKilled != tc.want.OOMKilled ||
				(got.ExitCode == nil) != (tc.want.ExitCode == nil) || (got.ExitCode != nil && *got.ExitCode != *tc.want.ExitCode) {
				t.Fatalf("ContainerRun = %+v (exit %v), want %+v (exit %v)", got, deref(got.ExitCode), tc.want, deref(tc.want.ExitCode))
			}
		})
	}
}

// A same-named Job's earlier pod is not this run's.
func TestContainerRunIgnoresAnotherJobsPod(t *testing.T) {
	job := failedJob("BackoffLimitExceeded")
	other := jobPod("9f1e-earlier-run", job.Namespace, terminated(3, "Error"))
	other.Status.ContainerStatuses[0].State.Terminated.StartedAt = metav1.Now()
	if got := ContainerRun(context.Background(), fakeClient(job, other), job); !got.StartedAt.IsZero() || got.ExitCode != nil {
		t.Fatalf("ContainerRun read another Job's pod: %+v", got)
	}
}

func deref(p *int32) any {
	if p == nil {
		return nil
	}
	return *p
}
