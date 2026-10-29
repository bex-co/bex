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
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWebImageHTTPSelectionUsesReadyCandidateAndIsSticky(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	httpPort, _ := strconv.Atoi(portText)
	scanner := &cannedScanner{}
	r, app, dep := detectFixture(t, time.Minute, scanner, func(a *appv1alpha1.App, pod *corev1.Pod) {
		a.Spec.Type = appv1alpha1.TypeWebService
		a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image@sha256:1", Revision: "rev-1", PrimaryPort: 1, Ports: []int32{1, int32(httpPort)}}
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.ContainerStatuses[0].Ready = true
	})
	waiting, message := r.selectWebImageHTTP(context.Background(), app, dep)
	if !waiting || app.Status.ImageNetwork.PrimaryPort != int32(httpPort) || app.Status.ImageNetwork.HTTPSelection != "http" || !strings.Contains(message, strconv.Itoa(httpPort)) {
		t.Fatalf("waiting=%v network=%+v message=%q", waiting, app.Status.ImageNetwork, message)
	}
	server.Close()
	r.Client = nil
	if waiting, _ := r.selectWebImageHTTP(t.Context(), app, dep); waiting {
		t.Fatal("resolved digest was scanned again after listener stopped")
	}
}

func TestWebImageWithoutEXPOSEUsesRuntimePortAndHonestStall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	httpPort, _ := strconv.Atoi(portText)
	scanner := &httpDiscoveryScanner{open: []int{httpPort}}
	r, app, dep := detectFixture(t, time.Minute, scanner, func(a *appv1alpha1.App, pod *corev1.Pod) {
		a.Spec.Type = appv1alpha1.TypeWebService
		a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image", PrimaryPort: 1, Ports: []int32{1}, NoDeclaredPorts: true}
		pod.Status.PodIP = "127.0.0.1"
	})
	if hint := imagePortStallHint(app, 1); !strings.Contains(hint, "image declares no TCP ports") || strings.Contains(hint, "declared TCP ports (") {
		t.Fatalf("dishonest fallback hint: %s", hint)
	}
	r.selectWebImageHTTP(t.Context(), app, dep)
	if scanner.fullCalls != 1 || scanner.boundedCalls != 0 {
		t.Fatalf("HTTP discovery reused TCP-only early stop: full=%d bounded=%d", scanner.fullCalls, scanner.boundedCalls)
	}
	if ports := app.Status.ImageNetwork.Ports; len(ports) != 1 || ports[0] != int32(httpPort) {
		t.Fatalf("runtime detection kept invented port: %v", ports)
	}
}

func TestWebHTTPFallbackWaitsForStartupAndDoesNotChangePrivateOrExplicit(t *testing.T) {
	for _, age := range []time.Duration{time.Minute, 2 * time.Minute} {
		r, app, dep := detectFixture(t, age, &cannedScanner{}, func(a *appv1alpha1.App, pod *corev1.Pod) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
			a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image", PrimaryPort: 1, Ports: []int32{1}}
			pod.Status.PodIP = "127.0.0.1"
		})
		r.selectWebImageHTTP(t.Context(), app, dep)
		if got := app.Status.ImageNetwork.HTTPSelection; (age < webHTTPDiscoveryWindow && got != "") || (age > webHTTPDiscoveryWindow && got != "fallback") {
			t.Fatalf("age=%s selection=%q", age, got)
		}
	}
	for _, spec := range []appv1alpha1.AppSpec{
		{Type: appv1alpha1.TypePrivateService, PortMode: appv1alpha1.PortModeImageV1},
		{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageConfiguredV1},
	} {
		app := &appv1alpha1.App{Spec: spec, Status: appv1alpha1.AppStatus{ImageNetwork: &appv1alpha1.ImageNetworkStatus{}}}
		if webImageHTTPSelectionPending(app) {
			t.Fatalf("explicit/private network requires discovery: %+v", spec)
		}
	}
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1}, Status: appv1alpha1.AppStatus{EnvPort: 10000, ImageNetwork: &appv1alpha1.ImageNetworkStatus{}}}
	if webImageHTTPSelectionPending(app) {
		t.Fatal("PORT=10000 must override HTTP discovery")
	}
}

func TestWebHTTPRetainsServingDigestButNotNewImageOrRemovedPORT(t *testing.T) {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1}, Status: appv1alpha1.AppStatus{ServingNetwork: &appv1alpha1.ImageNetworkStatus{Image: "image-a", ConfiguredPort: new(int32(0)), PrimaryPort: 15672, Ports: []int32{4369, 15672}}}}
	candidate := &appv1alpha1.ImageNetworkStatus{Image: "image-a", PrimaryPort: 4369, Ports: []int32{4369, 15672}}
	retainWebHTTPSelection(app, candidate, false)
	if candidate.PrimaryPort != 15672 || candidate.HTTPSelection != "retained" {
		t.Fatalf("known-unconfigured serving choice changed: %+v", candidate)
	}
	candidate = &appv1alpha1.ImageNetworkStatus{Image: "image-b", PrimaryPort: 4369}
	retainWebHTTPSelection(app, candidate, false)
	if candidate.HTTPSelection != "" {
		t.Fatal("new digest inherited old decision")
	}
	app.Status.ServingNetwork.HTTPSelection = "configured"
	candidate = &appv1alpha1.ImageNetworkStatus{Image: "image-a", PrimaryPort: 4369}
	retainWebHTTPSelection(app, candidate, false)
	if candidate.HTTPSelection != "" {
		t.Fatal("removed PORT retained configured choice")
	}
}

type httpDiscoveryScanner struct {
	open                    []int
	fullCalls, boundedCalls int
}

func (s *httpDiscoveryScanner) Scan(context.Context, string) ([]int, error) {
	s.fullCalls++
	return s.open, nil
}
func (s *httpDiscoveryScanner) ScanPorts(context.Context, string, []int) ([]int, error) {
	s.boundedCalls++
	return []int{1}, nil
}

func TestWebHTTPDecisionSurvivesSameDigestMetadataRefresh(t *testing.T) {
	previous := &appv1alpha1.ImageNetworkStatus{Image: "image-a", ConfiguredPort: new(int32(0)), HTTPSelection: "http", PrimaryPort: 15672, Ports: []int32{4369, 15672}}
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1}, Status: appv1alpha1.AppStatus{ImageNetwork: previous}}
	candidate := &appv1alpha1.ImageNetworkStatus{Image: "image-a", PrimaryPort: 4369, Ports: []int32{4369, 15672}, Revision: "next"}
	retainWebHTTPSelection(app, candidate, false)
	if candidate.PrimaryPort != 15672 || candidate.HTTPSelection != "http" {
		t.Fatalf("same digest reselected its listener: %+v", candidate)
	}
	candidate.Ports[0] = 1
	if previous.Ports[0] != 4369 {
		t.Fatal("reused port set aliases previous release")
	}
}

func TestWebHTTPDiscoveryKeepsPriorServingEvidenceWithoutPromotingReadyCandidate(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 2}, Status: appv1alpha1.AppStatus{ActiveRevision: "rev-old"}}
	dep := progressDeadlineDep("web")
	dep.Status.Conditions = nil
	readyPod := func(name, revision string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: revision}}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	}
	old, candidate := readyPod("old", "rev-old"), readyPod("candidate", "rev-1")
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app, old, candidate).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: cl}
	if _, err := r.reportWebHTTPSelection(t.Context(), app, dep, "candidate", "Detecting HTTP", 4369); err != nil {
		t.Fatal(err)
	}
	if app.Status.ActiveRevision != "rev-old" || app.Status.Phase != appv1alpha1.PhaseDeploying || !meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionServing) || meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionReady) {
		t.Fatalf("pending selection promoted or lost serving release: %+v", app.Status)
	}
}

// pairedHTTPListeners starts two loopback listeners and answers GET / with
// lowerAnswer on the lower port and a fast 404 on the higher one.
func pairedHTTPListeners(t *testing.T, lowerAnswer func(net.Conn)) (lower, higher int32) {
	t.Helper()
	listeners := make([]net.Listener, 2)
	for i := range listeners {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners[i] = l
	}
	if listeners[0].Addr().(*net.TCPAddr).Port > listeners[1].Addr().(*net.TCPAddr).Port {
		listeners[0], listeners[1] = listeners[1], listeners[0]
	}
	var mu sync.Mutex
	active := make(map[net.Conn]struct{})
	var handlers, accepting sync.WaitGroup
	for i, l := range listeners {
		accepting.Go(func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				mu.Lock()
				active[conn] = struct{}{}
				mu.Unlock()
				handlers.Go(func() {
					defer func() { _ = conn.Close(); mu.Lock(); delete(active, conn); mu.Unlock() }()
					_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
					if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
						return
					}
					if i == 0 {
						lowerAnswer(conn)
						return
					}
					_, _ = io.WriteString(conn, "HTTP/1.1 404 Not Found\r\n")
				})
			}
		})
	}
	t.Cleanup(func() {
		for _, l := range listeners {
			_ = l.Close()
		}
		accepting.Wait()
		mu.Lock()
		for conn := range active {
			_ = conn.Close()
		}
		mu.Unlock()
		handlers.Wait()
	})
	return int32(listeners[0].Addr().(*net.TCPAddr).Port), int32(listeners[1].Addr().(*net.TCPAddr).Port)
}

// w8/m141 t007: RabbitMQ without PORT chose Prometheus 15692 when the
// management listener's cold first GET / on 15672 outlasted the per-port
// budget while 15692 answered a fast 404. Within the discovery window the
// slow lower port is retried rather than passed over; after the window a
// still-silent lower port no longer blocks the lowest answering one.
func TestWebHTTPWaitsForASlowLowerListenerBeforeCommittingAHigherOne(t *testing.T) {
	var cold atomic.Bool
	cold.Store(true)
	lower, higher := pairedHTTPListeners(t, func(conn net.Conn) {
		if cold.Swap(false) {
			time.Sleep(4 * time.Second) // beyond the per-port budget
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\n")
	})
	web := func(image string, ports ...int32) func(*appv1alpha1.App, *corev1.Pod) {
		return func(a *appv1alpha1.App, pod *corev1.Pod) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
			a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: image, PrimaryPort: ports[0], Ports: ports}
			pod.Status.PodIP = "127.0.0.1"
		}
	}
	// Port 1 is refused, like RabbitMQ's unbound declared ports.
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{}, web("rabbitmq@sha256:1", 1, lower, higher))
	waiting, message := r.selectWebImageHTTP(t.Context(), app, dep)
	if !waiting || app.Status.ImageNetwork.HTTPSelection != "" || app.Status.ImageNetwork.PrimaryPort != 1 || !strings.Contains(message, strconv.Itoa(int(lower))) {
		t.Fatalf("cold scan committed past the slow lower port: network=%+v message=%q", app.Status.ImageNetwork, message)
	}
	r.portScanned.Delete("http/" + app.Namespace + "/" + app.Name) // the next scan interval
	waiting, message = r.selectWebImageHTTP(t.Context(), app, dep)
	if !waiting || app.Status.ImageNetwork.HTTPSelection != appv1alpha1.HTTPSelectionDetected || app.Status.ImageNetwork.PrimaryPort != lower || !strings.Contains(message, "Detected HTTP on port "+strconv.Itoa(int(lower))) {
		t.Fatalf("warm scan network=%+v message=%q, want lower port %d", app.Status.ImageNetwork, message, lower)
	}

	silentLower, answering := pairedHTTPListeners(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) })
	r, app, dep = detectFixture(t, 2*time.Minute, &cannedScanner{}, web("image@sha256:2", silentLower, answering))
	if _, message := r.selectWebImageHTTP(t.Context(), app, dep); app.Status.ImageNetwork.PrimaryPort != answering || app.Status.ImageNetwork.HTTPSelection != appv1alpha1.HTTPSelectionDetected {
		t.Fatalf("after the window a silent lower port blocked the HTTP choice: network=%+v message=%q", app.Status.ImageNetwork, message)
	}
}
