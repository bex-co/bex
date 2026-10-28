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
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1}, Status: appv1alpha1.AppStatus{ServingNetwork: &appv1alpha1.ImageNetworkStatus{Image: "image-a", PrimaryPort: 15672, Ports: []int32{4369, 15672}}}}
	candidate := &appv1alpha1.ImageNetworkStatus{Image: "image-a", PrimaryPort: 4369, Ports: []int32{4369, 15672}}
	retainWebHTTPSelection(app, candidate, false)
	if candidate.PrimaryPort != 15672 || candidate.HTTPSelection != "retained" {
		t.Fatalf("legacy serving choice changed: %+v", candidate)
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
	previous := &appv1alpha1.ImageNetworkStatus{Image: "image-a", HTTPSelection: "http", PrimaryPort: 15672, Ports: []int32{4369, 15672}}
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
