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
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// reservedHTTPListenerPort serves HTTP on loopback on one of the ports bex
// reserves, standing in for a process that binds one. A host that already
// uses both skips the test.
func reservedHTTPListenerPort(t *testing.T) int {
	t.Helper()
	for _, port := range []int{18012, 18013} {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		server := &httptest.Server{
			Listener: listener,
			Config:   &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })},
		}
		server.Start()
		t.Cleanup(server.Close)
		return port
	}
	t.Skip("ports 18012 and 18013 are both in use on this host")
	return 0
}

// reservedHint is the stall hint naming reserved as the one HTTP listener.
func reservedHint(reserved int) string {
	return " The process serves HTTP on port " + strconv.Itoa(reserved) +
		", which is reserved: bind the server to 0.0.0.0 on the port in PORT (10000) instead. The reserved ports are 18012, 18013 and 19099."
}

// w1/m659: a web service whose only HTTP listener is on a port bex reserves
// never goes live, since nothing routes there, and its stall said no port was
// open at all. Render's deploy page warns about such a listener on a free
// plan. The scan now names it in the stall's hint, which the failure reason
// keeps and the control plane reads the port from, and the port stays out of
// the routable set: the scan still reads as no open port, and nothing is
// adopted.
func TestDetectBoundPortNamesAReservedHTTPListener(t *testing.T) {
	reserved := reservedHTTPListenerPort(t)
	probe := &corev1.Probe{ProbeHandler: healthProbeOpenPortHandler(), PeriodSeconds: 10, TimeoutSeconds: 5}
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{reserved}}, func(a *appv1alpha1.App, p *corev1.Pod) {
		onLoopback(a, p)
		a.Spec.Type = appv1alpha1.TypeWebService
		p.Spec.Containers[0].StartupProbe, p.Spec.Containers[0].ReadinessProbe = probe, probe
	})

	for scan := range 2 {
		hint := r.detectBoundPort(t.Context(), app, dep, 10000)
		if hint != reservedHint(reserved) {
			t.Fatalf("scan %d: hint = %q,\n want %q", scan+1, hint, reservedHint(reserved))
		}
		time.Sleep(time.Millisecond)
	}
	if app.Status.DetectedPort != 0 || app.ServingPort() != appv1alpha1.DefaultPort {
		t.Fatalf("DetectedPort=%d ServingPort=%d: a reserved port was adopted", app.Status.DetectedPort, app.ServingPort())
	}
	if found := listenerSet([]int{reserved}); len(found) != 0 {
		t.Fatalf("the routable set holds %v", found)
	}

	stall := appv1alpha1.OpenPortProbeStallReason("startup", 0) + reservedHint(reserved)
	if scan := appv1alpha1.PortScanOf(stall); scan != appv1alpha1.PortScanNoOpenPort {
		t.Fatalf("the stall reads as port scan %d, want no open port", scan)
	}
	if got := appv1alpha1.ReservedPortListener(stall); got != int32(reserved) {
		t.Fatalf("ReservedPortListener(stall) = %d, want %d", got, reserved)
	}
	r.portScanned.Delete(app.Namespace + "/" + app.Name)
	reason, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000)
	if failure := terminalStallMessage(reason, msg); appv1alpha1.ReservedPortListener(failure) != int32(reserved) {
		t.Fatalf("the failure reason %q names no reserved listener", failure)
	}
}

// A scan that finds no reserved listener, or only one a platform container
// declares (the edge-cache sidecar of a pod template from before w3/m489),
// says what it said before.
func TestDetectBoundPortNamesNoReservedListenerWhenNoneIsTheProcesss(t *testing.T) {
	reserved := reservedHTTPListenerPort(t)
	for name, tc := range map[string]struct {
		open   []int
		mutate func(*appv1alpha1.App, *corev1.Pod)
	}{
		"nothing listens": {mutate: onLoopback},
		"a platform container declares it": {open: []int{reserved}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			onLoopback(a, p)
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "sidecar", Ports: []corev1.ContainerPort{{ContainerPort: int32(reserved)}}})
		}},
	} {
		r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: tc.open}, tc.mutate)
		hint := r.detectBoundPort(t.Context(), app, dep, 10000)
		if hint != noOpenPortMessage(app, 10000) {
			t.Errorf("%s: hint = %q, want %q", name, hint, noOpenPortMessage(app, 10000))
		}
		if got := appv1alpha1.ReservedPortListener(appv1alpha1.OpenPortProbeStallReason("startup", 0) + hint); got != 0 {
			t.Errorf("%s: ReservedPortListener = %d, want none", name, got)
		}
	}
	edge := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName}, {Name: edgeCacheContainerName}}}}
	if !platformPort(edge, edgeCachePort) || platformPort(edge, 18012) {
		t.Fatal("the edge-cache sidecar's port is not told apart from the process's")
	}
}

// A web image service past HTTP discovery reads its listeners from the node's
// reader, which sees a reserved one too: the stall names it, as it names a
// listener bound only to localhost (w8/394).
func TestStallNamesAReservedListenerAfterHTTPDiscovery(t *testing.T) {
	reserved := reservedHTTPListenerPort(t)
	probe := &corev1.Probe{ProbeHandler: healthProbeOpenPortHandler(), PeriodSeconds: 10, TimeoutSeconds: 5}
	r, app, dep := detectFixture(t, time.Minute, nil, func(a *appv1alpha1.App, p *corev1.Pod) {
		onLoopback(a, p)
		a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypeWebService, appv1alpha1.PortModeWebImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "docker.io/library/busybox:1.36", Revision: "rev-1",
			PrimaryPort: 10000, Ports: []int32{10000}, NoDeclaredPorts: true, HTTPSelection: appv1alpha1.HTTPSelectionFallback}
		p.Spec.Containers[0].StartupProbe, p.Spec.Containers[0].ReadinessProbe = probe, probe
	})
	r.Listeners = &cannedListeners{open: []int{reserved}}
	_, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000)
	if codes := stallHintCodes(t, msg); !slices.Contains(codes, appv1alpha1.DeployReasonHintReservedPort) {
		t.Fatalf("stall %q carries hints %v, want %s", msg, codes, appv1alpha1.DeployReasonHintReservedPort)
	}
	if got := appv1alpha1.ReservedPortListener(msg); got != int32(reserved) {
		t.Fatalf("ReservedPortListener = %d, want %d", got, reserved)
	}
}
