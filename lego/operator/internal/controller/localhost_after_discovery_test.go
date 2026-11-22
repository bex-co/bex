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
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fallbackImageFixture is a free web image service past HTTP discovery's
// window with no port found (HTTPSelectionFallback), its first instance
// running a minute and waiting on the any-open-port startup check: w8/394's
// busybox fixture, httpd bound to 127.0.0.1:$PORT.
func fallbackImageFixture(t *testing.T, reader *cannedListeners) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment) {
	t.Helper()
	probe := &corev1.Probe{ProbeHandler: healthProbeOpenPortHandler(), PeriodSeconds: 10, TimeoutSeconds: 5}
	r, app, dep := detectFixture(t, time.Minute, nil, func(a *appv1alpha1.App, p *corev1.Pod) {
		a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypeWebService, appv1alpha1.PortModeWebImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "docker.io/library/busybox:1.36", Revision: "rev-1",
			PrimaryPort: 10000, Ports: []int32{10000}, NoDeclaredPorts: true, HTTPSelection: appv1alpha1.HTTPSelectionFallback}
		p.Spec.Containers[0].StartupProbe, p.Spec.Containers[0].ReadinessProbe = probe, probe
	})
	r.Listeners = reader
	return r, app, dep
}

// stallHintCodes is the codes of the hints riding a probe stall's {hint}.
func stallHintCodes(t *testing.T, stall string) []string {
	t.Helper()
	_, params, ok := appv1alpha1.ParseDeployReason(stall)
	if !ok {
		t.Fatalf("the stall parses as no reason: %q", stall)
	}
	var codes []string
	for _, p := range params {
		if p.Name != "hint" {
			continue
		}
		parts, rest := appv1alpha1.ParseDeployReasonParts(p.Value)
		if rest != "" {
			t.Fatalf("hint %q leaves %q unparsed", p.Value, rest)
		}
		for _, part := range parts {
			codes = append(codes, part.Code)
		}
	}
	return codes
}

// w8/394: once HTTP discovery's window closed, a web image service listening
// only on localhost got a stall that named the closed default port alone, and
// so did its failure at the budget, although discovery had found the listener
// on 127.0.0.1. The stall keeps the finding past discovery, the failure
// reason carries it, and the control plane reads the scan as localhost-only.
func TestStallKeepsTheLocalhostFindingAfterHTTPDiscovery(t *testing.T) {
	reader := &cannedListeners{loopback: []int{10000}}
	r, app, dep := fallbackImageFixture(t, reader)
	r.portDetectEvery = time.Hour
	bare := appv1alpha1.OpenPortProbeStallReason("startup", 0)
	localhost := " No open port was detected on 0.0.0.0, but ports are open on localhost (10000): the server must bind host 0.0.0.0 on the port in PORT (10000)."
	want := bare + localhost + " No port answered HTTP during startup discovery."

	for pass := range 3 {
		reason, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000)
		if reason != reasonHealthCheckFailing || msg != want {
			t.Fatalf("pass %d: stall = %s %q,\n want %s %q", pass, reason, msg, reasonHealthCheckFailing, want)
		}
	}
	if calls := reader.calls.Load(); calls != 1 {
		t.Fatalf("reader reads = %d, want 1: the passes between replay its verdict", calls)
	}
	stall := want
	if scan := appv1alpha1.PortScanOf(stall); scan != appv1alpha1.PortScanLocalhostOnly {
		t.Fatalf("the stall reads as port scan %d, want localhost only", scan)
	}
	if codes := stallHintCodes(t, stall); !slices.Equal(codes, []string{appv1alpha1.DeployReasonHintNoOpenPortLocalhost, appv1alpha1.DeployReasonHintNoHTTPDiscovered}) {
		t.Fatalf("hint codes = %v", codes)
	}
	if strings.Contains(stall, "background worker") {
		t.Fatalf("a process that serves on localhost is sent to a background worker: %q", stall)
	}

	// The verdict at the budget scans afresh (settleFailedRollout) and names it too.
	r.portScanned.Delete(app.Namespace + "/" + app.Name)
	reason, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000)
	failure := terminalStallMessage(reason, msg)
	if !strings.Contains(failure, "never succeeded within the rollout window") || !strings.HasSuffix(failure, localhost+" No port answered HTTP during startup discovery.") {
		t.Fatalf("failure reason = %q", failure)
	}
}

// The finding is the reader's alone, and only past discovery: a reader that
// sees nothing on loopback, a node without one, or a discovery still running
// keeps the image's own port hint.
func TestStallWithoutALocalhostFindingKeepsTheImageHint(t *testing.T) {
	plain := func(app *appv1alpha1.App) string {
		return appv1alpha1.OpenPortProbeStallReason("startup", 0) + imageStallNote(app, 10000)
	}
	for name, tc := range map[string]struct {
		reader *cannedListeners
		mutate func(*appv1alpha1.App)
	}{
		"nothing listens":          {reader: &cannedListeners{}},
		"a node without a reader":  {reader: &cannedListeners{loopback: []int{10000}, err: errNoListenerReader}},
		"the reader fails":         {reader: &cannedListeners{loopback: []int{10000}, err: errListenerReaderNotReady}},
		"a reachable port is open": {reader: &cannedListeners{open: []int{10000}, loopback: []int{9000}}},
		"discovery found a port": {reader: &cannedListeners{loopback: []int{10000}}, mutate: func(a *appv1alpha1.App) {
			a.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
		}},
	} {
		r, app, dep := fallbackImageFixture(t, tc.reader)
		if tc.mutate != nil {
			tc.mutate(app)
		}
		if _, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000); msg != plain(app) {
			t.Errorf("%s: stall = %q,\n want %q", name, msg, plain(app))
		}
	}

	// A discovery still inside its window never reaches this path: its own
	// stall names the finding (http_port_localhost_only).
	r, app, dep := fallbackImageFixture(t, &cannedListeners{loopback: []int{10000}})
	app.Status.ImageNetwork.HTTPSelection = ""
	if got := r.detectLocalhostAfterDiscovery(t.Context(), app, dep, 10000); got != "" {
		t.Fatalf("a running discovery got %q", got)
	}
}
