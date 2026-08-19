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
	"context"
	"fmt"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Retry classification labels (docs/ADR060 D5). A CLOSED set: the reason is a
// Prometheus label, so an open-ended string would let one pathological build
// mint unbounded series. Every spent attempt maps to exactly one of these.
const (
	// RetryDisruption: the attempt was evicted/preempted/drained. buildPodFailurePolicy
	// Ignores DisruptionTarget, so this retry cost the tenant nothing — it is the
	// mechanism working, and separating it is what keeps it out of the infra SLO.
	RetryDisruption = "disruption"
	// RetryOOM: a phase was OOM-killed. Deliberately not absorbed as a disruption
	// (buildPodFailurePolicy's comment) — it is capped like any unclassified failure.
	RetryOOM = "oom"
	// RetryTransient: a phase exited ExitTransient — classifyPrelude matched
	// transientLogPattern, so a retry could plausibly change the outcome.
	RetryTransient = "transient"
	// RetryTenant: a phase exited ExitTenantError. podFailurePolicy fails the Job
	// outright on this code, so a retry attributed here means the policy did not
	// apply — the tripwire for a build phase that stopped classifying.
	RetryTenant = "tenant"
	// RetryUnclassified: anything else (signal exits, an image that never started).
	RetryUnclassified = "unclassified"
)

// Signals are the per-attempt facts the build plane can only read from a Job's
// pods: how long the build waited for capacity, how long its push phase took,
// and why each spent attempt was retried (docs/ADR060 D5).
//
// Read once per build at its terminal observation rather than every reconcile.
// The build-plane client is UNCACHED (cmd/manager wires BuildClient to the
// uncached reader, because the build namespace is outside the manager's Secret
// cache), so a per-reconcile read here would put a Job Get + Pod List on the
// hot path of every repo-backed App for the finished Job's whole hour-long TTL.
// Everything below is derivable after the fact — the pods outlive the Job by
// construction — so paying for it once is not a loss of fidelity.
type Signals struct {
	// QueueSeconds is admission → first placement: the Job's own creation
	// timestamp to the moment the scheduler first bound one of its pods to a
	// node. THE platform capacity SLI (queue time is ours; run time is mostly the
	// tenant's — conflating them is what misattributed the 2026-08-11 incident).
	// QueueMeasured distinguishes "waited zero seconds" from "never observed".
	// metav1.Time serializes at SECOND granularity, so a build placed on a warm
	// node reads as exactly 0 — and that is the regime the low buckets exist to
	// resolve. Dropping it as unmeasured would populate the histogram only with
	// the slow cases and inflate the p95 the capacity alert pages on.
	QueueSeconds  float64
	QueueMeasured bool
	// PushSeconds is the skopeo push phase's own terminated duration on the final
	// attempt, and PushFailed whether it exited non-zero. The push phase is the
	// one part of a build the platform owns end to end (docs/ADR060 D4), so its
	// duration and error rate are platform signals even when the build failed.
	PushSeconds float64
	PushFailed  bool
	// Retries holds one classification label per SPENT attempt — that is, per pod
	// that was superseded by a later one. len(Retries) is the number of retries,
	// never the number of attempts.
	Retries []string
}

// ReadSignals derives a finished build's queue/push/retry facts from its Job and
// pods. Errors are returned rather than swallowed so the caller can log them;
// a build with no pods (kpack, or a Job reaped before observation) yields a
// zero Signals and no error, which the metric recorders skip.
func ReadSignals(ctx context.Context, cl client.Client, namespace, jobName string) (Signals, error) {
	var job batchv1.Job
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: jobName}, &job); err != nil {
		return Signals{}, fmt.Errorf("build signals: get job %s: %w", jobName, err)
	}
	pods, err := listJobPods(ctx, cl, namespace, jobName)
	if err != nil {
		return Signals{}, fmt.Errorf("build signals: %w", err)
	}
	return signalsFrom(&job, pods), nil
}

// signalsFrom is the pure half of ReadSignals, so the derivation is testable
// without a cluster.
func signalsFrom(job *batchv1.Job, pods []corev1.Pod) Signals {
	if len(pods) == 0 {
		return Signals{}
	}
	// Attempt order is the Job's retry order. Sort by creation, tie-broken by
	// name: two pods created inside the same second must still order stably, or
	// "the last attempt" (whose push phase is metered) would flip between reads.
	attempts := make([]*corev1.Pod, 0, len(pods))
	for i := range pods {
		attempts = append(attempts, &pods[i])
	}
	sort.Slice(attempts, func(a, b int) bool {
		return newerFirst(attempts[b].CreationTimestamp, attempts[b].Name,
			attempts[a].CreationTimestamp, attempts[a].Name)
	})

	sig := Signals{}
	// Queue time is measured to the FIRST placement, not the last: a retry's own
	// wait is a consequence of the retry, and charging it here would double-count
	// the same capacity shortage the first attempt already recorded.
	for _, pod := range attempts {
		if at, ok := scheduledAt(pod); ok {
			sig.QueueSeconds = max(at.Sub(job.CreationTimestamp.Time).Seconds(), 0)
			sig.QueueMeasured = true
			break
		}
	}
	// Every attempt but the last was superseded by a retry; classify why.
	for _, pod := range attempts[:len(attempts)-1] {
		sig.Retries = append(sig.Retries, retryReason(pod))
	}
	last := attempts[len(attempts)-1]
	if secs, failed, ok := phaseDuration(last, pushContainer); ok {
		sig.PushSeconds, sig.PushFailed = secs, failed
	}
	return sig
}

// scheduledAt is when the scheduler bound pod to a node. The PodScheduled
// condition's transition — not the pod's start time — is the exact end of the
// capacity wait: the build's own work begins only once a kubelet owns the pod.
func scheduledAt(pod *corev1.Pod) (metav1.Time, bool) {
	for i := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[i]
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionTrue {
			return cond.LastTransitionTime, true
		}
	}
	return metav1.Time{}, false
}

// retryReason classifies one spent attempt into the closed label set above.
// Order matters: DisruptionTarget is checked first because an evicted pod's
// containers also report an exit code, and the disruption is the cause.
func retryReason(pod *corev1.Pod) string {
	for i := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[i]
		if cond.Type == corev1.DisruptionTarget && cond.Status == corev1.ConditionTrue {
			return RetryDisruption
		}
	}
	// First failing phase wins: the build's phases run strictly serially, so the
	// earliest non-zero exit is the one that ended the attempt — anything after it
	// is a consequence.
	for _, cs := range containerStatuses(pod) {
		term := cs.State.Terminated
		if term == nil || term.ExitCode == 0 {
			continue
		}
		switch {
		case term.Reason == "OOMKilled":
			return RetryOOM
		case term.ExitCode == ExitTransient:
			return RetryTransient
		case term.ExitCode == ExitTenantError:
			return RetryTenant
		}
		return RetryUnclassified
	}
	return RetryUnclassified
}

// phaseDuration is how long the named build phase ran on this attempt, whether
// it failed, and whether it ran at all.
func phaseDuration(pod *corev1.Pod, name string) (seconds float64, failed, ok bool) {
	for _, cs := range containerStatuses(pod) {
		if cs.Name != name || cs.State.Terminated == nil {
			continue
		}
		term := cs.State.Terminated
		seconds = max(term.FinishedAt.Sub(term.StartedAt.Time).Seconds(), 0)
		return seconds, term.ExitCode != 0, true
	}
	return 0, false, false
}

// containerStatuses is a pod's phase statuses in execution order: initContainers
// (clone, native prepare, BuildKit, and push when signing is enabled) then main
// containers. Both slices always, because tenant-image signing MOVES the push
// phase between them — a slice-specific lookup would silently stop metering the
// push the moment signing is turned on.
func containerStatuses(pod *corev1.Pod) []corev1.ContainerStatus {
	all := make([]corev1.ContainerStatus, 0, len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	all = append(all, pod.Status.InitContainerStatuses...)
	return append(all, pod.Status.ContainerStatuses...)
}
