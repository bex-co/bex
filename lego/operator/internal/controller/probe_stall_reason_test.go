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
	"maps"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/591: a GET or TCP probe stall, in flight and restated once the deploy
// failed, parses back to its template and values, so the dashboard can word
// it in its own language. The English is what every API has always returned.
func TestProbeStallMessageParsesAsItsTemplate(t *testing.T) {
	const (
		head      = "the container is running but its "
		restarted = " The liveness check has already restarted the container 3 time(s), so the check is failing rather than merely slow."
		pathTail  = " Check that the path returns a 2xx or 3xx status on the service's own port."
		tcpTail   = " No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port."
	)
	probe := func(h corev1.ProbeHandler) *corev1.Probe {
		return &corev1.Probe{ProbeHandler: h, PeriodSeconds: 10, TimeoutSeconds: 5}
	}
	httpGet := probe(corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(3000)}})
	tcp := probe(corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(3000)}})
	exec := probe(healthProbeExecHandler("/cgi-bin/health", 8080, ""))
	started := true
	for name, tc := range map[string]struct {
		probe              *corev1.Probe
		started            *bool
		restarts           int32
		waiting, failed    string
		waitCode, failCode string
		params             map[string]string
	}{
		"GET at startup": {
			probe:    httpGet,
			waiting:  head + "startup health check has not succeeded, so the rollout is waiting: GET /healthz on port 3000." + pathTail,
			failed:   head + "startup health check never succeeded within the rollout window, so the deploy failed: GET /healthz on port 3000." + pathTail,
			waitCode: appv1alpha1.DeployReasonProbeHTTPWaiting, failCode: appv1alpha1.DeployReasonProbeHTTPFailed,
			params: map[string]string{"kind": "startup", "path": "/healthz", "port": "3000"},
		},
		"GET restarted by liveness": {
			probe: httpGet, started: &started, restarts: 3,
			waiting:  head + "readiness health check has not succeeded, so the rollout is waiting: GET /healthz on port 3000." + restarted + pathTail,
			failed:   head + "readiness health check never succeeded within the rollout window, so the deploy failed: GET /healthz on port 3000." + restarted + pathTail,
			waitCode: appv1alpha1.DeployReasonProbeHTTPWaitingRestarted, failCode: appv1alpha1.DeployReasonProbeHTTPFailedRestarted,
			params: map[string]string{"kind": "readiness", "path": "/healthz", "port": "3000", "restarts": "3"},
		},
		"helper GET": {
			probe:    exec,
			waiting:  head + "startup health check has not succeeded, so the rollout is waiting: GET /cgi-bin/health on port 8080." + pathTail,
			failed:   head + "startup health check never succeeded within the rollout window, so the deploy failed: GET /cgi-bin/health on port 8080." + pathTail,
			waitCode: appv1alpha1.DeployReasonProbeHTTPWaiting, failCode: appv1alpha1.DeployReasonProbeHTTPFailed,
			params: map[string]string{"kind": "startup", "path": "/cgi-bin/health", "port": "8080"},
		},
		"TCP at startup": {
			probe:    tcp,
			waiting:  head + "startup health check has not succeeded, so the rollout is waiting: a TCP connect to port 3000." + tcpTail,
			failed:   head + "startup health check never succeeded within the rollout window, so the deploy failed: a TCP connect to port 3000." + tcpTail,
			waitCode: appv1alpha1.DeployReasonProbeTCPWaiting, failCode: appv1alpha1.DeployReasonProbeTCPFailed,
			params: map[string]string{"kind": "startup", "port": "3000"},
		},
		"TCP restarted by liveness": {
			probe: tcp, started: &started, restarts: 3,
			waiting:  head + "readiness health check has not succeeded, so the rollout is waiting: a TCP connect to port 3000." + restarted + tcpTail,
			failed:   head + "readiness health check never succeeded within the rollout window, so the deploy failed: a TCP connect to port 3000." + restarted + tcpTail,
			waitCode: appv1alpha1.DeployReasonProbeTCPWaitingRestarted, failCode: appv1alpha1.DeployReasonProbeTCPFailedRestarted,
			params: map[string]string{"kind": "readiness", "port": "3000", "restarts": "3"},
		},
	} {
		pod := corev1.Pod{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", StartupProbe: tc.probe, ReadinessProbe: tc.probe}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Started: tc.started, RestartCount: tc.restarts,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-5 * time.Minute))}},
			}}},
		}
		_, waiting := probeStallMessage([]corev1.Pod{pod})
		failed := terminalStallMessage(reasonHealthCheckFailing, waiting)
		if waiting != tc.waiting || failed != tc.failed {
			t.Errorf("%s:\n waiting %q\n want    %q\n failed  %q\n want    %q", name, waiting, tc.waiting, failed, tc.failed)
		}
		if shared := appv1alpha1.TerminalStallText(waiting); shared != failed {
			t.Errorf("%s: bex-api restates it as %q, the operator as %q", name, shared, failed)
		}
		// A port hint rolloutStallDiagnosis appends on a scanning pass keeps
		// the sentence's code, so its language does not flip between passes.
		const hint = " No open port was detected on the instance."
		for _, tail := range []string{"", hint} {
			for text, want := range map[string]string{waiting + tail: tc.waitCode, failed + tail: tc.failCode} {
				code, params, ok := appv1alpha1.ParseDeployReason(text)
				got := map[string]string{}
				for _, p := range params {
					got[p.Name] = p.Value
				}
				wantParams := maps.Clone(tc.params)
				wantParams["hint"] = tail
				if !ok || code != want || !maps.Equal(got, wantParams) {
					t.Errorf("%s: %q parses as %s %v (ok=%v), want %s %v", name, text, code, got, ok, want, wantParams)
				}
			}
		}
	}
}

// w5/m356: a check that is neither a GET nor a TCP connect keeps its English
// sentence, which parses as its own template, in flight and once failed.
func TestProbeStallOfAnotherCheckParsesAsItsTemplate(t *testing.T) {
	grpc := &corev1.Probe{ProbeHandler: corev1.ProbeHandler{GRPC: &corev1.GRPCAction{Port: 9000}}}
	const (
		head      = "the container is running but its readiness health check "
		restarted = " The liveness check has already restarted the container 2 time(s), so the check is failing rather than merely slow."
		pathTail  = " Check that the path returns a 2xx or 3xx status on the service's own port."
	)
	for name, tc := range map[string]struct {
		restarts           int32
		waiting, failed    string
		waitCode, failCode string
	}{
		"restarted": {
			restarts: 2,
			waiting:  head + "has not succeeded, so the rollout is waiting: the configured health check." + restarted + pathTail,
			failed:   head + "never succeeded within the rollout window, so the deploy failed: the configured health check." + restarted + pathTail,
			waitCode: appv1alpha1.DeployReasonProbeOtherWaitingRestarted, failCode: appv1alpha1.DeployReasonProbeOtherFailedRestarted,
		},
		"not restarted": {
			waiting:  head + "has not succeeded, so the rollout is waiting: the configured health check." + pathTail,
			failed:   head + "never succeeded within the rollout window, so the deploy failed: the configured health check." + pathTail,
			waitCode: appv1alpha1.DeployReasonProbeOtherWaiting, failCode: appv1alpha1.DeployReasonProbeOtherFailed,
		},
	} {
		waiting := probeStallText("readiness", grpc, tc.restarts)
		failed := terminalStallMessage(reasonHealthCheckFailing, waiting)
		if waiting != tc.waiting || failed != tc.failed {
			t.Errorf("%s:\n waiting %q\n want    %q\n failed  %q\n want    %q", name, waiting, tc.waiting, failed, tc.failed)
		}
		for text, want := range map[string]string{waiting: tc.waitCode, failed: tc.failCode} {
			if code, _, ok := appv1alpha1.ParseDeployReason(text); !ok || code != want {
				t.Errorf("%s: %q parses as %s (ok=%v), want %s", name, text, code, ok, want)
			}
		}
	}
}
