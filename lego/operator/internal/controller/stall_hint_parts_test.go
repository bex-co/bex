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
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const workerHint = " If the service does not need to receive traffic on any port, create a background worker instead."

// privateImageService makes the detect fixture a private image-port service
// whose image declares primary among ports.
func privateImageService(primary int32, ports ...int32) func(*appv1alpha1.App, *corev1.Pod) {
	return func(a *appv1alpha1.App, _ *corev1.Pod) {
		a.Spec.Type, a.Spec.PortMode, a.Spec.Port = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1, 0
		a.Status.ReleaseGeneration = 1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{
			Image: "i@sha256:x", Revision: "rev-1", PrimaryPort: primary, Ports: ports, ConfiguredPort: new(int32(0)),
		}
	}
}

// w5/m356: every port hint and queued-build cause the operator writes is a
// part template's sentence, byte for byte the English REST and MCP have
// always returned, so the dashboard can word each one.
func TestStallHintsAndQueuedCausesAreTemplated(t *testing.T) {
	web := func(mode string, envPort int32, noDeclared bool, ports ...int32) *appv1alpha1.App {
		return projectionApp(func(a *appv1alpha1.App) {
			a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypeWebService, mode
			a.Status.EnvPort = envPort
			a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "i", Revision: "r", PrimaryPort: 10000, Ports: ports, NoDeclaredPorts: noDeclared}
		})
	}
	private := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "i", Revision: "r", PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	})
	fallback := web(appv1alpha1.PortModeWebImageV1, 0, true, 10000)
	fallback.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionFallback
	detected := projectionApp(func(a *appv1alpha1.App) { a.Status.DetectedPort = 3000 })

	scan := func(scanner *cannedScanner, mutate func(*appv1alpha1.App, *corev1.Pod), port int,
		detect func(*AppReconciler, context.Context, *appv1alpha1.App, *appsv1.Deployment, int) string) []string {
		r, app, dep := detectFixture(t, time.Minute, scanner, mutate)
		out := make([]string, 0, 2)
		for range 2 {
			out = append(out, detect(r, t.Context(), app, dep, port))
			time.Sleep(time.Millisecond)
		}
		return out
	}
	lone := scan(&cannedScanner{open: []int{8000}}, nil, 10000, (*AppReconciler).detectBoundPort)
	several := scan(&cannedScanner{open: []int{8000, 9090}}, nil, 10000, (*AppReconciler).detectBoundPort)
	privateLone := scan(&cannedScanner{open: []int{8080}}, privateImageService(80, 80), 80, (*AppReconciler).detectPrivatePrimary)
	privateErr := scan(&cannedScanner{err: portscan.ErrUnreachable}, privateImageService(80, 80), 80, (*AppReconciler).detectPrivatePrimary)

	for name, tc := range map[string]struct {
		got, want string
		codes     []string
	}{
		"image declares no ports": {
			got: imagePortStallHint(web(appv1alpha1.PortModeWebImageV1, 0, true, 10000), 10000),
			want: " Nothing accepted a TCP connection on port 10000, the default because the image declares no TCP ports." +
				" The server must bind host 0.0.0.0 on the port in PORT (10000)." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintImageDefaultPort, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"image declares ports": {
			got: imagePortStallHint(private, 8123),
			want: " Nothing accepted a TCP connection on port 8123, the primary port selected from the image's declared TCP ports (8123, 9000, 9009)." +
				" Bind the server to 0.0.0.0 on one of those ports, or set the service's port to the one it listens on." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintImageDeclaredPorts, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"PORT and no other declared port": {
			got: imagePortStallHint(web(appv1alpha1.PortModeWebImageV1, 9999, false, 9999), 9999),
			want: " Nothing accepted a TCP connection on port 9999, set by the service's PORT environment variable; the image declares no other TCP port." +
				" Bind the server to 0.0.0.0 on port 9999, or change the service's port to the one it listens on." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintEnvPortOnly, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"configured port and no other declared port": {
			got: imagePortStallHint(web(appv1alpha1.PortModeWebImageConfiguredV1, 0, false, 10000), 10000),
			want: " Nothing accepted a TCP connection on port 10000, set by the service's configured port; the image declares no other TCP port." +
				" Bind the server to 0.0.0.0 on port 10000, or change the service's port to the one it listens on." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintConfiguredPortOnly, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"PORT beside declared ports": {
			got: imagePortStallHint(web(appv1alpha1.PortModeWebImageV1, 9999, false, 80, 9999), 9999),
			want: " Nothing accepted a TCP connection on port 9999, set by the service's PORT environment variable; the image declares TCP ports (80)." +
				" Change the PORT environment variable to the port it listens on, or remove it to use the image's port." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintEnvPortDeclared, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"configured port beside declared ports": {
			got: imagePortStallHint(web(appv1alpha1.PortModeWebImageConfiguredV1, 0, false, 80, 10000), 10000),
			want: " Nothing accepted a TCP connection on port 10000, set by the service's configured port; the image declares TCP ports (80)." +
				" Change the service's port in Settings to the one it listens on, or remove it to use the image's port." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintConfiguredPortDeclared, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"discovery found no HTTP": {
			got: imageStallNote(fallback, 10000),
			want: " Nothing accepted a TCP connection on port 10000, the default because the image declares no TCP ports." +
				" The server must bind host 0.0.0.0 on the port in PORT (10000)." + workerHint + " No port answered HTTP during startup discovery.",
			codes: []string{appv1alpha1.DeployReasonHintImageDefaultPort, appv1alpha1.DeployReasonHintBackgroundWorker, appv1alpha1.DeployReasonHintNoHTTPDiscovered},
		},
		"no open port": {
			got:   noOpenPortMessage(projectionApp(), 10000),
			want:  " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT (10000)." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintNoOpenPort, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"no open port, a detected one before": {
			got:   noOpenPortMessage(detected, 3000),
			want:  " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT (10000); it listened on port 3000 before." + workerHint,
			codes: []string{appv1alpha1.DeployReasonHintNoOpenPortBefore, appv1alpha1.DeployReasonHintBackgroundWorker},
		},
		"a lone listener":         {got: lone[0], want: " The process is listening on port 8000.", codes: []string{appv1alpha1.DeployReasonHintListening}},
		"a lone listener adopted": {got: lone[1], want: " The process is listening on port 8000, which the service now uses.", codes: []string{appv1alpha1.DeployReasonHintListeningAdopted}},
		"several listeners": {
			got:   several[0],
			want:  " The process is listening on ports 8000, 9090, not on port 10000; set the service's port (or its PORT variable) to the one that serves HTTP.",
			codes: []string{appv1alpha1.DeployReasonHintListeningPorts},
		},
		"a private listener": {got: privateLone[0], want: " The process is listening on port 8080.", codes: []string{appv1alpha1.DeployReasonHintListening}},
		"a private listener adopted": {
			got:   privateLone[1],
			want:  " The process is listening on port 8080, which the service now uses as its primary port.",
			codes: []string{appv1alpha1.DeployReasonHintListeningPrimary},
		},
		"a private instance with no open port": {
			got:   privateErr[0],
			want:  " No open port was detected on the instance: the server must bind host 0.0.0.0 on the service's port (80).",
			codes: []string{appv1alpha1.DeployReasonHintNoOpenServicePort},
		},

		"a full workspace": {
			got:   buildCap{workspace: "tea-a", limit: 2, noun: "workspace"}.fullMessage(2),
			want:  "workspace has 2/2 concurrent builds active; waiting for a slot",
			codes: []string{appv1alpha1.DeployReasonQueuedBuildLimit},
		},
		"a full cluster": {
			got: clusterFullMessage, want: "the cluster's build capacity is full; waiting for a slot",
			codes: []string{appv1alpha1.DeployReasonQueuedClusterFull},
		},
		"a lost slot": {
			got:   appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonQueuedSlotTaken),
			want:  "a concurrent dispatch took the last build slot; waiting for a slot",
			codes: []string{appv1alpha1.DeployReasonQueuedSlotTaken},
		},
		"a linked database": {
			got:   appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonQueuedDatabase, appv1alpha1.DeployReasonParam{Name: "database", Value: "dpg-abc"}),
			want:  "Waiting for linked database dpg-abc to become available before building",
			codes: []string{appv1alpha1.DeployReasonQueuedDatabase},
		},
		"a linked datastore": {
			got:   appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonQueuedDatastore),
			want:  "Waiting for a linked datastore's connection details before building",
			codes: []string{appv1alpha1.DeployReasonQueuedDatastore},
		},
		"the build credential": {
			got:   appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonQueuedBuildCred),
			want:  "Waiting for the registry to accept this app's build credential",
			codes: []string{appv1alpha1.DeployReasonQueuedBuildCred},
		},
		"the pull credential": {
			got:   appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonQueuedPullCred),
			want:  "Waiting for the registry to accept this app's pull credential",
			codes: []string{appv1alpha1.DeployReasonQueuedPullCred},
		},
	} {
		if tc.got != tc.want {
			t.Errorf("%s:\n got  %q\n want %q", name, tc.got, tc.want)
		}
		parts, rest := appv1alpha1.ParseDeployReasonParts(tc.got)
		codes := make([]string, len(parts))
		for i, p := range parts {
			codes[i] = p.Code
		}
		if !slices.Equal(codes, tc.codes) || rest != "" {
			t.Errorf("%s: %q reads as %v + %q, want %v", name, tc.got, codes, rest, tc.codes)
		}
	}
}

// w5/m356: the port scan runs once per portDetectInterval while an in-flight
// rollout requeues every 5 s. The stall replays the last scan's verdict on the
// passes between, so its text holds still until a scan says otherwise or the
// rollout's pod changes.
func TestRolloutStallHoldsItsHintBetweenScans(t *testing.T) {
	probe := &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(10000)}},
		PeriodSeconds: 10, TimeoutSeconds: 5}
	scanner := &cannedScanner{}
	r, app, dep := detectFixture(t, time.Minute, scanner, func(_ *appv1alpha1.App, p *corev1.Pod) {
		p.Spec.Containers[0].StartupProbe, p.Spec.Containers[0].ReadinessProbe = probe, probe
	})
	r.portDetectEvery = time.Hour
	key := app.Namespace + "/" + app.Name
	stall := func() string {
		_, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, 10000)
		return msg
	}
	bare := appv1alpha1.ProbeStallReason("startup", true, "", "10000", 0)
	hinted := bare + noOpenPortMessage(app, 10000)

	for pass := range 4 {
		if got := stall(); got != hinted {
			t.Fatalf("pass %d: stall = %q, want %q", pass, got, hinted)
		}
	}
	if scanner.calls != 1 {
		t.Fatalf("scans = %d, want 1: the passes between replay its verdict", scanner.calls)
	}

	// A hint is about the port it checked: a stall about another port does
	// not replay it, and the next scan of that port decides afresh.
	if hint, _ := r.portHint.Load(key); hint == nil {
		t.Fatal("no hint kept after the scan")
	}
	if got := r.replayPortHint(t.Context(), key, dep, 8080); got != "" {
		t.Fatalf("a stall about port 8080 replayed %q", got)
	}
	if _, kept := r.portHint.Load(key); kept {
		t.Fatal("a hint about another port is kept")
	}
	r.portScanned.Delete(key)
	if got := stall(); got != hinted {
		t.Fatalf("rescanned after the port changed: stall = %q, want %q", got, hinted)
	}

	// A deleted App's scan state goes with it.
	r.forgetPortScans(key)
	for _, m := range []*sync.Map{&r.portScanned, &r.portCandidate, &r.portHint} {
		if _, kept := m.Load(key); kept {
			t.Fatal("a deleted App's scan state is kept")
		}
	}

	// A scan that finds the configured port answering clears the hint.
	r.portScanned.Delete(key)
	scanner.open = []int{10000}
	for pass := range 2 {
		if got := stall(); got != bare {
			t.Fatalf("after the port answered, pass %d: stall = %q, want %q", pass, got, bare)
		}
	}

	// A hint belongs to the pod it scanned: the next pod does not inherit it.
	r.portScanned.Delete(key)
	scanner.open = nil
	if got := stall(); got != hinted {
		t.Fatalf("rescanned stall = %q, want %q", got, hinted)
	}
	var pod corev1.Pod
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: dep.Namespace, Name: "web-1"}, &pod); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(t.Context(), &pod); err != nil {
		t.Fatal(err)
	}
	next := pod.DeepCopy()
	next.Name, next.UID, next.ResourceVersion = "web-2", "", ""
	if err := r.Create(t.Context(), next); err != nil {
		t.Fatal(err)
	}
	if got := stall(); got != bare {
		t.Fatalf("a new pod's stall = %q, want %q", got, bare)
	}
}

// A private service's candidate listener is replayed too, and a replay is no
// second scan: only two scans that agree move the primary.
func TestPrivatePrimaryHintHoldsBetweenScans(t *testing.T) {
	scanner := &cannedScanner{open: []int{8080}}
	r, app, dep := detectFixture(t, time.Minute, scanner, privateImageService(80, 80))
	r.portDetectEvery = time.Hour
	first := r.detectPrivatePrimary(t.Context(), app, dep, 80)
	for range 3 {
		if got := r.detectPrivatePrimary(t.Context(), app, dep, 80); got != first {
			t.Fatalf("replayed hint = %q, want %q", got, first)
		}
	}
	if scanner.calls != 1 || app.Status.ImageNetwork.PrimaryPort != 80 {
		t.Fatalf("scans = %d, primary = %d; want one scan and the primary unchanged", scanner.calls, app.Status.ImageNetwork.PrimaryPort)
	}
}
