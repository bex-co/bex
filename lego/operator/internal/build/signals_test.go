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

package build

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var signalsEpoch = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

func at(offset time.Duration) metav1.Time { return metav1.NewTime(signalsEpoch.Add(offset)) }

// signalsJob is the admitted build Job every case derives its queue time from.
func signalsJob() *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "bld", CreationTimestamp: at(0)}}
}

type podOpt func(*corev1.Pod)

func attempt(name string, created time.Duration, opts ...podOpt) corev1.Pod {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:              name,
		CreationTimestamp: at(created),
		Labels:            map[string]string{"job-name": "bld"},
	}}
	for _, o := range opts {
		o(&pod)
	}
	return pod
}

func scheduled(offset time.Duration) podOpt {
	return func(p *corev1.Pod) {
		p.Spec.NodeName = "node-a"
		p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
			Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: at(offset),
		})
	}
}

func disrupted() podOpt {
	return func(p *corev1.Pod) {
		p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
			Type: corev1.DisruptionTarget, Status: corev1.ConditionTrue,
		})
	}
}

// initTerminated adds a terminated INIT container (clone/buildkit, and push when
// signing is on); mainTerminated adds a terminated main container (push, or
// cosign when signing is on).
func initTerminated(name string, exit int32, reason string, start, end time.Duration) podOpt {
	return func(p *corev1.Pod) {
		p.Status.InitContainerStatuses = append(p.Status.InitContainerStatuses,
			terminatedStatus(name, exit, reason, start, end))
	}
}

func mainTerminated(name string, exit int32, reason string, start, end time.Duration) podOpt {
	return func(p *corev1.Pod) {
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses,
			terminatedStatus(name, exit, reason, start, end))
	}
}

func terminatedStatus(name string, exit int32, reason string, start, end time.Duration) corev1.ContainerStatus {
	return corev1.ContainerStatus{Name: name, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: exit, Reason: reason, StartedAt: at(start), FinishedAt: at(end),
	}}}
}

// TestSignalsQueueSecondsIsAdmissionToFirstPlacement pins the ONE definition
// that makes bex_build_queue_seconds the platform's SLI rather than a second
// copy of run time: admission (the Job's own creation) to the scheduler binding
// the pod, and to the FIRST placement even when the build was later retried.
func TestSignalsQueueSecondsIsAdmissionToFirstPlacement(t *testing.T) {
	job := signalsJob()
	// First attempt waits 4 minutes for a node, is disrupted, and the retry is
	// placed instantly. Charging the retry's own wait would double-count the same
	// capacity shortage; charging the retry's placement would report ~0s for a
	// build that in fact queued four minutes.
	pods := []corev1.Pod{
		attempt("bld-b", 5*time.Minute, scheduled(5*time.Minute+2*time.Second)),
		attempt("bld-a", 0, scheduled(4*time.Minute), disrupted()),
	}
	sig := signalsFrom(job, pods)
	if sig.QueueSeconds != 240 {
		t.Errorf("QueueSeconds = %v, want 240 (admission → first placement)", sig.QueueSeconds)
	}
	if len(sig.Retries) != 1 || sig.Retries[0] != RetryDisruption {
		t.Errorf("Retries = %v, want exactly [%s]", sig.Retries, RetryDisruption)
	}
}

// TestSignalsUnscheduledBuildHasNoQueueObservation: a build still waiting has no
// queue time yet. Recording 0 would report the pathological case as the healthy
// one — precisely inverting the series' meaning.
func TestSignalsUnscheduledBuildHasNoQueueObservation(t *testing.T) {
	sig := signalsFrom(signalsJob(), []corev1.Pod{attempt("bld-a", 0)})
	if sig.QueueSeconds != 0 {
		t.Errorf("QueueSeconds = %v, want 0 for a never-placed build", sig.QueueSeconds)
	}
	if len(sig.Retries) != 0 {
		t.Errorf("Retries = %v, want none: a sole attempt was never superseded", sig.Retries)
	}
}

// TestSignalsRetryReasons pins the closed classification set. The reason is a
// Prometheus label, and each value drives a different operational conclusion:
// disruption is the podFailurePolicy working, tenant should be impossible.
func TestSignalsRetryReasons(t *testing.T) {
	cases := []struct {
		name  string
		spent podOpt
		want  string
	}{
		{"eviction beats the exit code it also produces", disrupted(), RetryDisruption},
		{"oom is never absorbed as a disruption",
			initTerminated("buildkit", 137, "OOMKilled", 0, time.Minute), RetryOOM},
		{"a classified transient",
			initTerminated("clone", ExitTransient, "Error", 0, time.Minute), RetryTransient},
		{"a tenant exit that somehow retried",
			initTerminated("buildkit", ExitTenantError, "Error", 0, time.Minute), RetryTenant},
		{"an unrecognised signal exit",
			initTerminated("buildkit", 143, "Error", 0, time.Minute), RetryUnclassified},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := signalsFrom(signalsJob(), []corev1.Pod{
				attempt("bld-a", 0, c.spent),
				attempt("bld-b", time.Minute, scheduled(time.Minute)),
			})
			if len(sig.Retries) != 1 || sig.Retries[0] != c.want {
				t.Errorf("Retries = %v, want [%s]", sig.Retries, c.want)
			}
		})
	}
}

// TestSignalsOOMOnAnEvictedPodStaysDisruption: an evicted pod's containers also
// report a non-zero exit, so ordering is load-bearing — the disruption is the
// cause, and misfiling it would make node churn look like tenant OOM.
func TestSignalsOOMOnAnEvictedPodStaysDisruption(t *testing.T) {
	sig := signalsFrom(signalsJob(), []corev1.Pod{
		attempt("bld-a", 0, disrupted(), initTerminated("buildkit", 137, "OOMKilled", 0, time.Minute)),
		attempt("bld-b", time.Minute, scheduled(time.Minute)),
	})
	if len(sig.Retries) != 1 || sig.Retries[0] != RetryDisruption {
		t.Errorf("Retries = %v, want [%s]", sig.Retries, RetryDisruption)
	}
}

// TestSignalsPushPhaseFoundInEitherSlice is the regression the signing feature
// would otherwise cause silently: enabling BEX_TENANT_SIGNING_KEY_SECRET moves
// push from a main container to an initContainer, and a slice-specific lookup
// would stop metering the push without any error.
func TestSignalsPushPhaseFoundInEitherSlice(t *testing.T) {
	unsigned := signalsFrom(signalsJob(), []corev1.Pod{
		attempt("bld-a", 0, scheduled(0), mainTerminated("push", 0, "Completed", time.Minute, 3*time.Minute)),
	})
	if unsigned.PushSeconds != 120 || unsigned.PushFailed {
		t.Errorf("unsigned build: PushSeconds = %v failed = %v, want 120 / false", unsigned.PushSeconds, unsigned.PushFailed)
	}
	signed := signalsFrom(signalsJob(), []corev1.Pod{
		attempt("bld-a", 0, scheduled(0),
			initTerminated("push", 1, "Error", time.Minute, 2*time.Minute),
			mainTerminated("sign", 0, "Completed", 2*time.Minute, 3*time.Minute)),
	})
	if signed.PushSeconds != 60 || !signed.PushFailed {
		t.Errorf("signing build: PushSeconds = %v failed = %v, want 60 / true", signed.PushSeconds, signed.PushFailed)
	}
}

// TestSignalsNoPodsIsSilent: the kpack path dispatches an Image, not a Job, so
// it has no pods here. Everything must stay zero so the recorders skip it rather
// than dragging the platform's capacity percentile toward an unmeasured 0s.
func TestSignalsNoPodsIsSilent(t *testing.T) {
	sig := signalsFrom(signalsJob(), nil)
	if sig.QueueSeconds != 0 || sig.PushSeconds != 0 || sig.PushFailed || len(sig.Retries) != 0 {
		t.Errorf("Signals = %+v, want the zero value", sig)
	}
}
