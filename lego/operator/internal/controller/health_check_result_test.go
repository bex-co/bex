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
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m634: an Unhealthy event's message, whichever check wrote it, reads as
// what the check got back in Render's words. A probe that errored, or a line
// that says nothing about the check's answer, reads as no result: the surfaces
// then keep their generic sentence rather than invent a cause.
func TestHealthCheckResultText(t *testing.T) {
	probe := func(h corev1.ProbeHandler) *corev1.Probe { return &corev1.Probe{ProbeHandler: h, TimeoutSeconds: 5} }
	helperHTTP := probe(healthProbeExecHandler("/healthz", 8080, ""))
	helperOpenPort := probe(healthProbeOpenPortHandler())
	helperTCP := probe(corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{healthProbeBinary, "tcp", "--port", "8080", "--timeout", "5s"}}})
	kubeletHTTP := probe(corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(8080)}})
	kubeletTCP := probe(corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(8080)}})
	const refused = "dial tcp 10.0.0.7:8080: connect: connection refused"
	for _, tc := range []struct {
		name, message string
		probe         *corev1.Probe
		want          string
	}{
		// The helper since w6/m634 speaks Render's words already.
		{"helper 503", "Readiness probe failed: HTTP health check failed with status code 503\n", helperHTTP,
			"HTTP health check failed with status code 503"},
		{"helper timeout", "Startup probe failed: HTTP health check failed (timed out after 5 seconds)", helperHTTP,
			"HTTP health check failed (timed out after 5 seconds)"},
		{"helper refused", "Liveness probe failed: HTTP health check failed: " + refused, helperHTTP,
			"HTTP health check failed: " + refused},
		{"helper TCP", "Readiness probe failed: TCP health check failed: dial tcp 10.0.0.7:8080: i/o timeout", helperTCP,
			"TCP health check failed: dial tcp 10.0.0.7:8080: i/o timeout"},
		{"repeats combined by the kubelet", "(combined from similar events): Readiness probe failed: HTTP health check failed with status code 500", helperHTTP,
			"HTTP health check failed with status code 500"},
		// The kubelet killed the command at its 5 s and kept nothing it printed.
		{"command timeout of an HTTP check", `Readiness probe failed: command "/bex-health/health-probe check --port 8080 --path /healthz --timeout 5s" timed out`, helperHTTP,
			"HTTP health check failed (timed out after 5 seconds)"},
		{"command timeout of the open-port check", `Startup probe failed: command "/bex-health/health-probe open-port --skip 18000 --timeout 5s" timed out`, helperOpenPort,
			"TCP health check failed (timed out after 5 seconds)"},
		// The helper before w6/m634, still installed in an instance whose pod
		// template has not changed since.
		{"earlier helper status", "Readiness probe failed: health-probe: health check answered 503\n", helperHTTP,
			"HTTP health check failed with status code 503"},
		{"earlier helper refused", `Readiness probe failed: health-probe: Get "http://10.0.0.7:8080/healthz": ` + refused, helperHTTP,
			"HTTP health check failed: " + refused},
		{"earlier helper timeout", `Readiness probe failed: health-probe: Get "http://10.0.0.7:8080/healthz": context deadline exceeded`, helperHTTP,
			"HTTP health check failed (timed out after 5 seconds)"},
		{"earlier helper, nothing listening", "Startup probe failed: health-probe: no open port: nothing listens", helperOpenPort,
			"TCP health check failed: no open ports detected"},
		{"earlier helper, nothing accepted", "Startup probe failed: health-probe: no open port: none of the 2 listening ports accepted a connection on 10.0.0.7", helperOpenPort,
			""},
		// The kubelet's own probes.
		{"kubelet HTTP status", "Readiness probe failed: HTTP probe failed with statuscode: 502", kubeletHTTP,
			"HTTP health check failed with status code 502"},
		{"kubelet HTTP timeout", `Readiness probe failed: Get "http://10.0.0.7:8080/healthz": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`, kubeletHTTP,
			"HTTP health check failed (timed out after 5 seconds)"},
		{"kubelet TCP refused", "Readiness probe failed: " + refused, kubeletTCP,
			"TCP health check failed: " + refused},
		{"kubelet TCP timeout", "Liveness probe failed: dial tcp 10.0.0.7:8080: i/o timeout", kubeletTCP,
			"TCP health check failed: dial tcp 10.0.0.7:8080: i/o timeout"},
		// No result.
		{"errored probe", "Readiness probe errored and resulted in unknown state: rpc error: code = NotFound", helperHTTP, ""},
		{"exec that could not start", "Readiness probe failed: OCI runtime exec failed: exec failed: unable to start container process", helperHTTP, ""},
		{"not a probe event", "Back-off restarting failed container app", helperHTTP, ""},
		{"empty output", "Readiness probe failed: ", helperHTTP, ""},
	} {
		if got := healthCheckResultText(tc.message, tc.probe); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	// A result is bounded to what the status field holds, on a rune boundary.
	long := healthCheckResultText("Readiness probe failed: HTTP health check failed: "+strings.Repeat("é", 400), helperHTTP)
	if len(long) > appv1alpha1.HealthCheckFailureMaxLength || !strings.HasPrefix(long, "HTTP health check failed: é") || strings.ContainsRune(long, '�') {
		t.Fatalf("a long result reads %d bytes: %q", len(long), long)
	}
}

// The newest event of the stalled container that carries a result wins:
// another pod's or another container's never does, and a newer event with no
// result (an errored probe) gives way to the older failure.
func TestNewestHealthCheckResult(t *testing.T) {
	now := time.Now()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1", UID: "uid-1"}}
	stall := &probeStall{pod: pod, status: corev1.ContainerStatus{Name: appContainerName},
		probe: &corev1.Probe{ProbeHandler: healthProbeExecHandler("/healthz", 8080, ""), TimeoutSeconds: 5}}
	event := func(uid types.UID, fieldPath, message string, last time.Time) corev1.Event {
		return corev1.Event{
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "web-1", UID: uid, FieldPath: fieldPath},
			Reason:         unhealthyEventReason, Message: message, LastTimestamp: metav1.NewTime(last),
		}
	}
	app := "spec.containers{" + appContainerName + "}"
	events := []corev1.Event{
		event("uid-1", app, "Readiness probe failed: HTTP health check failed with status code 500", now.Add(-time.Minute)),
		event("uid-1", app, "Readiness probe failed: HTTP health check failed with status code 503", now.Add(-10*time.Second)),
		event("uid-1", app, "Readiness probe errored and resulted in unknown state: rpc error", now.Add(-5*time.Second)),
		event("uid-0", app, "Readiness probe failed: HTTP health check failed with status code 404", now),
		event("uid-1", "spec.containers{sidecar}", "Readiness probe failed: HTTP health check failed with status code 418", now),
	}
	if got := newestHealthCheckResult(events, stall); got != "HTTP health check failed with status code 503" {
		t.Fatalf("newest = %q", got)
	}
	// events.k8s.io records repeats on its series.
	series := event("uid-1", app, "Readiness probe failed: HTTP health check failed (timed out after 5 seconds)", now.Add(-time.Hour))
	series.Series = &corev1.EventSeries{Count: 4, LastObservedTime: metav1.NewMicroTime(now.Add(-time.Second))}
	if got := newestHealthCheckResult(slices.Concat(events, []corev1.Event{series}), stall); got != "HTTP health check failed (timed out after 5 seconds)" {
		t.Fatalf("newest with a series = %q", got)
	}
	if got := newestHealthCheckResult(events[2:], stall); got != "" {
		t.Fatalf("no result of its own = %q", got)
	}
}

// healthCheckFixture is a live release whose one instance web-1 fails its
// readiness check, with the events the kubelet recorded for it, and an API
// reader that counts its event lists.
func healthCheckFixture(t *testing.T, events ...corev1.Event) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment, *int) {
	t.Helper()
	r, app, dep := detectFixture(t, 5*time.Minute, nil, func(_ *appv1alpha1.App, p *corev1.Pod) {
		probe := &corev1.Probe{ProbeHandler: healthProbeExecHandler("/healthz", 8080, ""), PeriodSeconds: 5, TimeoutSeconds: 5}
		p.UID = "uid-1"
		p.Spec.Containers[0].ReadinessProbe = probe
		started := true
		p.Status.ContainerStatuses[0].Started = &started
	})
	pods := &corev1.PodList{}
	if err := r.List(t.Context(), pods); err != nil {
		t.Fatal(err)
	}
	objects := make([]client.Object, 0, len(events)+len(pods.Items))
	for i := range pods.Items {
		objects = append(objects, &pods.Items[i])
	}
	for i := range events {
		objects = append(objects, &events[i])
	}
	app.Generation, app.Status.ReleaseGeneration = 1, 1
	objects = append(objects, app)
	lists := 0
	r.Client = fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(objects...).
		WithStatusSubresource(&appv1alpha1.App{}).
		WithIndex(&corev1.Event{}, "involvedObject.name", func(o client.Object) []string { return []string{o.(*corev1.Event).InvolvedObject.Name} }).
		WithIndex(&corev1.Event{}, "reason", func(o client.Object) []string { return []string{o.(*corev1.Event).Reason} }).
		WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*corev1.EventList); ok {
				lists++
			}
			return c.List(ctx, list, opts...)
		}}).Build()
	r.APIReader = r.Client
	return r, app, dep, &lists
}

func unhealthyEvent(name, pod, message string) corev1.Event {
	return corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: "tea-a"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "tea-a", Name: pod, UID: "uid-1", FieldPath: "spec.containers{" + appContainerName + "}"},
		Reason:         unhealthyEventReason, Message: message, Type: corev1.EventTypeWarning, LastTimestamp: metav1.Now(),
	}
}

// w6/m634 t003: a probe stall records the instance's last result once, ends
// its reason with it, and lists events again only for a refresh or another
// instance. Without a result the reason is what it was, and nothing is
// recorded.
func TestHealthCheckResultIsReadOnceAndRecorded(t *testing.T) {
	r, app, dep, lists := healthCheckFixture(t, unhealthyEvent("web-1.a", "web-1", "Readiness probe failed: HTTP health check failed with status code 503\n"))
	bare := appv1alpha1.ProbeStallReason("readiness", false, "/healthz", "8080", 0)
	if reason, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 8080); reason != reasonHealthCheckFailing || msg != bare {
		t.Fatalf("diagnosis = %s %q", reason, msg)
	}
	hint := r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, false)
	if hint != " Last result: HTTP health check failed with status code 503." {
		t.Fatalf("hint = %q", hint)
	}
	if code, params, ok := appv1alpha1.ParseDeployReason(bare + hint); !ok || code != appv1alpha1.DeployReasonProbeHTTPWaiting {
		t.Fatalf("the reason with its result parses as %s %v %v", code, params, ok)
	}
	if parts, rest := appv1alpha1.ParseDeployReasonParts(hint); len(parts) != 1 || parts[0].Code != appv1alpha1.DeployReasonHintLastResult ||
		parts[0].Params[0].Value != "HTTP health check failed with status code 503" || rest != "" {
		t.Fatalf("the result reads as parts %+v + %q", parts, rest)
	}
	want := &appv1alpha1.HealthCheckFailureStatus{Instance: "web-1", Message: "HTTP health check failed with status code 503"}
	if got := app.Status.HealthCheckFailure; got == nil || *got != *want {
		t.Fatalf("recorded %+v, want %+v", got, want)
	}
	for range 3 {
		r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, false)
	}
	if *lists != 1 {
		t.Fatalf("events listed %d times for one instance, want once", *lists)
	}
	if r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, true) != hint || *lists != 2 {
		t.Fatalf("a refresh listed events %d times in all, want 2", *lists)
	}
	if r.healthCheckResultHint(t.Context(), app, dep, crashLoopBackOff, false) != "" || *lists != 2 {
		t.Fatal("another reason read a result")
	}

	// An instance with no event yet: no result, nothing recorded, and its
	// events are not listed again on every pass.
	r, app, dep, lists = healthCheckFixture(t)
	if hint := r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, false); hint != "" || app.Status.HealthCheckFailure != nil {
		t.Fatalf("no event: hint %q, recorded %+v", hint, app.Status.HealthCheckFailure)
	}
	r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, false)
	if *lists != 1 {
		t.Fatalf("events listed %d times within the retry, want once", *lists)
	}
	// A record of another instance never speaks for this one.
	app.Status.HealthCheckFailure = &appv1alpha1.HealthCheckFailureStatus{Instance: "web-0", Message: "HTTP health check failed with status code 500"}
	if hint := r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, true); hint != "" || app.Status.HealthCheckFailure != nil {
		t.Fatalf("another instance's record: hint %q, recorded %+v", hint, app.Status.HealthCheckFailure)
	}

	// With no API reader nothing is read: the cached client would start a
	// cluster-wide events informer.
	r, app, dep, lists = healthCheckFixture(t, unhealthyEvent("web-1.a", "web-1", "Readiness probe failed: HTTP health check failed with status code 503"))
	r.APIReader = nil
	if hint := r.healthCheckResultHint(t.Context(), app, dep, reasonHealthCheckFailing, false); hint != "" || *lists != 0 {
		t.Fatalf("no API reader: hint %q, %d lists", hint, *lists)
	}
}

// The record describes Ready only while Ready names a failing health check.
func TestHealthCheckFailureClearsWithAnyOtherReady(t *testing.T) {
	recorded := &appv1alpha1.HealthCheckFailureStatus{Instance: "web-1", Message: "HTTP health check failed with status code 503"}
	for _, tc := range []struct {
		name  string
		ready *metav1.Condition
		keep  bool
	}{
		{"health check failing", &metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonHealthCheckFailing}, true},
		{"crash loop", &metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: crashLoopBackOff}, false},
		{"rollout progressing", &metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonRolloutProgressing}, false},
		{"ready", &metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed"}, false},
		{"no Ready", nil, false},
	} {
		app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{HealthCheckFailure: recorded}}
		if tc.ready != nil {
			meta.SetStatusCondition(&app.Status.Conditions, *tc.ready)
		}
		keepHealthCheckFailureWithReady(app)
		if kept := app.Status.HealthCheckFailure != nil; kept != tc.keep {
			t.Errorf("%s: kept %v, want %v", tc.name, kept, tc.keep)
		}
	}
}

// w6/m634 t005: a deploy that never passed its health check ends its failure
// reason with what the check last got back, read afresh when it settles, and
// the reason keeps its code with the result as one more part. With no result
// the reason is the sentence it always was.
func TestFailedDeployReasonEndsWithTheLastResult(t *testing.T) {
	failed := func(r *AppReconciler, app *appv1alpha1.App, dep *appsv1.Deployment) string {
		t.Helper()
		if _, err := r.settleFailedRollout(t.Context(), app, dep, 8080); err != nil {
			t.Fatal(err)
		}
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		if ready == nil || ready.Reason != reasonHealthCheckFailing || app.Status.Phase != appv1alpha1.PhaseFailed {
			t.Fatalf("settled %s with Ready %+v", app.Status.Phase, ready)
		}
		return ready.Message
	}
	sentence := terminalStallMessage(reasonHealthCheckFailing, appv1alpha1.ProbeStallReason("readiness", false, "/healthz", "8080", 0))

	r, app, dep, lists := healthCheckFixture(t, unhealthyEvent("web-1.a", "web-1", "Readiness probe failed: HTTP health check failed with status code 503"))
	// An earlier read of the same instance is no reason to skip the final one.
	app.Status.HealthCheckFailure = &appv1alpha1.HealthCheckFailureStatus{Instance: "web-1", Message: "HTTP health check failed with status code 500"}
	msg := failed(r, app, dep)
	if want := sentence + " Last result: HTTP health check failed with status code 503."; msg != want || *lists != 1 {
		t.Fatalf("failure reason (%d lists)\n %q\nwant\n %q", *lists, msg, want)
	}
	code, params, ok := appv1alpha1.ParseDeployReason(msg)
	if !ok || code != appv1alpha1.DeployReasonProbeHTTPFailed {
		t.Fatalf("%q parses as %s %v", msg, code, ok)
	}
	hint := slices.IndexFunc(params, func(p appv1alpha1.DeployReasonParam) bool { return p.Name == "hint" })
	if hint < 0 {
		t.Fatalf("%q parses with no hint: %v", msg, params)
	}
	parts, rest := appv1alpha1.ParseDeployReasonParts(params[hint].Value)
	if len(parts) == 0 || parts[len(parts)-1].Code != appv1alpha1.DeployReasonHintLastResult || rest != "" {
		t.Fatalf("the hint %q reads as %+v + %q", params[hint].Value, parts, rest)
	}

	r, app, dep, _ = healthCheckFixture(t)
	if msg := failed(r, app, dep); msg != sentence {
		t.Fatalf("no result: %q, want the sentence alone %q", msg, sentence)
	}
}

// Of several stalled instances the stall names the first by pod name, so the
// cache's unordered list never makes it name, and read, another one each pass.
func TestProbeStallNamesTheSameInstanceEveryPass(t *testing.T) {
	probe := &corev1.Probe{ProbeHandler: healthProbeExecHandler("/healthz", 8080, ""), PeriodSeconds: 5, TimeoutSeconds: 5}
	stalled := func(name string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, ReadinessProbe: probe}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-time.Minute))}},
			}}},
		}
	}
	for _, order := range [][]string{{"web-c", "web-a", "web-b"}, {"web-b", "web-c", "web-a"}} {
		pods := make([]corev1.Pod, len(order))
		for i, name := range order {
			pods[i] = stalled(name)
		}
		if stall := probeStallOf(pods); stall == nil || stall.pod.Name != "web-a" {
			t.Fatalf("listed as %v, the stall names %+v, want web-a", order, stall)
		}
		if pods[0].Name != order[0] {
			t.Fatalf("the pick reordered the caller's pods: %v", pods)
		}
	}
}
