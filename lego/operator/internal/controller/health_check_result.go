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
	"cmp"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/healthprobe"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// What a failing health check got back (w6/m634, ADR004 § A failed health
// check says what it got back). The kubelet records a failed probe as an
// Unhealthy event on the pod, with the check's output: the health-probe
// helper's one line in Render's words, or the kubelet's own TCP or HTTP probe
// error. Nothing else carries it: pod status records only that the container
// is not ready. When Ready names a probe stall, the operator lists that one
// pod's Unhealthy events, keeps the newest result in Render's words, and
// records it as status.healthCheckFailure. bex-api reports it as
// server_failed's failureReason.unhealthy, and a deploy's probe-stall reason
// ends with it.
//
// The list goes through the API reader, never the cached client: a cached
// List of events would start a cluster-wide events informer, a watch of every
// event in the cluster. The operator may list events and not watch them.
//
// +kubebuilder:rbac:groups="",resources=events,verbs=list

// unhealthyEventReason is the kubelet's event reason for a failed probe, and
// eventReasonField the event field selector that matches it.
const (
	unhealthyEventReason = "Unhealthy"
	eventReasonField     = "reason"
)

// healthCheckReadRetry is how long an instance whose Unhealthy events gave no
// result waits before they are listed again. Its first read comes once a
// probe has had the time to fail (probeHadTimeToFail), so the kubelet has
// almost always recorded one by then. A retry still lands before bex-api
// confirms the outage 10 s after it first sees it, so its server_failed
// carries a result found late; a probe that only errors costs a list per
// retry while it stalls.
const healthCheckReadRetry = 10 * time.Second

// healthCheckReadAt is the instance an App's events were last listed for, and
// when.
type healthCheckReadAt struct {
	instance string
	at       time.Time
}

// healthCheckResultHint is the part a probe stall's message ends with: what
// the failing check last got back, which lastHealthCheckResult also records on
// app's status. Empty for any other reason, and when no result was read: a
// reason then reads as it did before. refresh reads the events again even for
// the instance already recorded, for the verdict a failed deploy keeps.
func (r *AppReconciler) healthCheckResultHint(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, reason string, refresh bool) string {
	if reason != reasonHealthCheckFailing {
		return ""
	}
	result := r.lastHealthCheckResult(ctx, app, dep, refresh)
	if result == "" {
		return ""
	}
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintLastResult,
		appv1alpha1.DeployReasonParam{Name: "result", Value: result})
}

// keepHealthCheckFailureWithReady clears the recorded result unless Ready
// still names a failing health check, which is the only Ready it describes.
func keepHealthCheckFailureWithReady(app *appv1alpha1.App) {
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != reasonHealthCheckFailing {
		app.Status.HealthCheckFailure = nil
	}
}

// lastHealthCheckResult is what the failing health check of the instance a
// probe stall names last got back, in Render's words, and records it as
// app.Status.HealthCheckFailure. The instance already recorded is not read
// again unless refresh is set; one whose events give no result keeps the
// result it had. A stall on another instance with no result yet clears the
// record, so it never speaks for the wrong instance.
func (r *AppReconciler) lastHealthCheckResult(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, refresh bool) string {
	stall := r.probeStallInstance(ctx, dep)
	if stall == nil {
		return ""
	}
	recorded := app.Status.HealthCheckFailure
	same := recorded != nil && recorded.Instance == stall.pod.Name
	if same && !refresh {
		return recorded.Message
	}
	if result := r.readHealthCheckResult(ctx, app, stall, refresh); result != "" {
		app.Status.HealthCheckFailure = &appv1alpha1.HealthCheckFailureStatus{Instance: stall.pod.Name, Message: result}
		return result
	}
	if same {
		return recorded.Message
	}
	app.Status.HealthCheckFailure = nil
	return ""
}

// probeStallInstance is the instance a probe stall of dep's pods names, read
// as stuckPodMessage reads them, from the cache.
func (r *AppReconciler) probeStallInstance(ctx context.Context, dep *appsv1.Deployment) *probeStall {
	if dep == nil || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return nil
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace),
		client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return nil
	}
	return probeStallOf(instancePods(pods.Items))
}

// readHealthCheckResult lists the stalled instance's Unhealthy events, one
// namespaced list filtered to that pod, and words the newest that carries a
// check's result. With no API reader it reads nothing. An instance whose last
// read found nothing is read again only after healthCheckReadRetry, unless
// force is set.
func (r *AppReconciler) readHealthCheckResult(ctx context.Context, app *appv1alpha1.App, stall *probeStall, force bool) string {
	if r.APIReader == nil {
		return ""
	}
	key := app.Namespace + "/" + app.Name
	now := time.Now()
	if last, ok := r.healthCheckRead.Load(key); ok && !force {
		if read := last.(healthCheckReadAt); read.instance == stall.pod.Name && now.Sub(read.at) < healthCheckReadRetry {
			return ""
		}
	}
	r.healthCheckRead.Store(key, healthCheckReadAt{instance: stall.pod.Name, at: now})
	var events corev1.EventList
	if err := r.APIReader.List(ctx, &events, client.InNamespace(stall.pod.Namespace),
		client.MatchingFields{"involvedObject.name": stall.pod.Name, eventReasonField: unhealthyEventReason}); err != nil {
		logf.FromContext(ctx).V(1).Info("listing an instance's health-check events failed",
			"app", app.Name, "pod", stall.pod.Name, "error", err.Error())
		return ""
	}
	return newestHealthCheckResult(events.Items, stall)
}

// newestHealthCheckResult is the newest of the stalled container's Unhealthy
// events that carries a check's result, in Render's words; "" when none does.
func newestHealthCheckResult(events []corev1.Event, stall *probeStall) string {
	container := "spec.containers{" + stall.status.Name + "}"
	events = slices.DeleteFunc(slices.Clone(events), func(e corev1.Event) bool {
		return e.Reason != unhealthyEventReason ||
			e.InvolvedObject.UID != "" && e.InvolvedObject.UID != stall.pod.UID ||
			e.InvolvedObject.FieldPath != "" && e.InvolvedObject.FieldPath != container
	})
	slices.SortStableFunc(events, func(a, b corev1.Event) int { return eventLastSeen(b).Compare(eventLastSeen(a)) })
	for _, e := range events {
		if result := healthCheckResultText(e.Message, stall.probe); result != "" {
			return result
		}
	}
	return ""
}

// eventLastSeen is when an event last happened, whichever API wrote it: the
// kubelet's recorder counts repeats on lastTimestamp, events.k8s.io on its
// series.
func eventLastSeen(e corev1.Event) time.Time {
	seen := e.CreationTimestamp.Time
	for _, t := range []time.Time{e.FirstTimestamp.Time, e.LastTimestamp.Time, e.EventTime.Time} {
		if t.After(seen) {
			seen = t
		}
	}
	if e.Series != nil && e.Series.LastObservedTime.After(seen) {
		seen = e.Series.LastObservedTime.Time
	}
	return seen
}

var (
	// unhealthyProbeFailed is the kubelet's message for a probe that ran and
	// failed. A probe that errored (it could not run at all) is the
	// platform's failure, not what the check got back, and is left out.
	unhealthyProbeFailed = regexp.MustCompile(`^(?:\(combined from similar events\): )?(?:Startup|Readiness|Liveness) probe failed: `)
	// execTimedOut is the kubelet's own output for an exec probe it killed at
	// its timeout, which keeps nothing the command printed.
	execTimedOut = regexp.MustCompile(`^command ".*" timed out$`)
	// probeStatusCode is an answer outside 2xx-3xx as the kubelet's HTTP probe
	// and the helper before w6/m634 worded it.
	probeStatusCode = regexp.MustCompile(`^(?:HTTP probe failed with statuscode: |health check answered )(\d{3})$`)
	// requestURL is the request a Go HTTP client error starts with.
	requestURL = regexp.MustCompile(`^Get "[^"]*": `)
)

// healthCheckResultText words one Unhealthy event's message as what probe
// got back, in Render's words (healthprobe's), bounded to
// HealthCheckFailureMaxLength; "" when it carries no check's result. It reads
// the helper's line, the kubelet's own TCP and HTTP probe errors, and the
// helper's output before it spoke Render's words, which an instance keeps
// until its pod template next changes (the helper is adopted lazily).
func healthCheckResultText(message string, probe *corev1.Probe) string {
	prefix := unhealthyProbeFailed.FindString(message)
	if prefix == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(message[len(prefix):]), "\n")
	out := strings.TrimSpace(lines[len(lines)-1])
	out = strings.TrimPrefix(out, "health-probe: ")
	kind := healthCheckKind(probe)
	timeout := time.Duration(cmp.Or(probe.TimeoutSeconds, healthCheckTimeoutSeconds)) * time.Second
	var result string
	switch m := probeStatusCode.FindStringSubmatch(out); {
	case strings.HasPrefix(out, healthprobe.HTTPFailed), strings.HasPrefix(out, healthprobe.TCPFailed):
		result = out
	case m != nil:
		code, _ := strconv.Atoi(m[1])
		result = healthprobe.StatusFailure(code)
	case execTimedOut.MatchString(out):
		result = healthprobe.TimeoutFailure(kind, timeout)
	case requestURL.MatchString(out):
		// An HTTP check that got no answer.
		cause := requestURL.ReplaceAllString(out, "")
		if strings.Contains(cause, "context deadline exceeded") || strings.Contains(cause, "Client.Timeout exceeded") ||
			strings.HasSuffix(cause, "i/o timeout") {
			result = healthprobe.TimeoutFailure(healthprobe.HTTPFailed, timeout)
		} else {
			result = healthprobe.HTTPFailed + ": " + cause
		}
	case strings.HasPrefix(out, "dial tcp "):
		result = kind + ": " + out
	case out == "no open port: nothing listens":
		result = healthprobe.NoOpenPorts
	default:
		return ""
	}
	return appv1alpha1.BoundHealthCheckFailure(result)
}

// healthCheckKind is the words probe's failure starts with: HTTP for a Health
// Check Path, checked by the helper or by the kubelet's HTTP probe, and TCP
// for every connect check.
func healthCheckKind(probe *corev1.Probe) string {
	if _, _, ok := probeGET(probe); ok {
		return healthprobe.HTTPFailed
	}
	return healthprobe.TCPFailed
}
