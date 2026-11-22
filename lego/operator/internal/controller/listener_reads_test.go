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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/listeners"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cannedListeners is a node reader's canned answer.
type cannedListeners struct {
	open     []int
	loopback []int
	err      error
	calls    atomic.Int32
	pods     []types.UID
}

func (c *cannedListeners) PodListeners(_ context.Context, pod *corev1.Pod) (listeners.Answer, error) {
	c.calls.Add(1)
	c.pods = append(c.pods, pod.UID)
	return listeners.Answer{Ports: c.open, Loopback: c.loopback}, c.err
}

// A node with a Ready reader is read, never swept: the record is the reader's
// whole answer, and the scanner is not called.
func TestPrivatePortsComeFromTheNodesReader(t *testing.T) {
	scanner := &cannedScanner{open: []int{1}}
	r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
	reader := &cannedListeners{open: []int{5432, 50051}}
	r.Listeners = reader
	got := sweptPrivatePorts(r, app, dep)
	if got == nil || !slices.Equal(got.Ports, []int32{5432, 50051}) || got.Revision != "rev-2" {
		t.Fatalf("record = %+v, want the reader's [5432 50051] for rev-2", got)
	}
	if scanner.calls != 0 || reader.calls.Load() != 1 {
		t.Fatalf("scanner called %d times and reader %d, want 0 and 1", scanner.calls, reader.calls.Load())
	}
}

// A reader's answer is complete: a listener that went away leaves the record
// at once, which a partial sweep could never prove.
func TestAReadersEmptyAnswerClearsTheRecord(t *testing.T) {
	r, app, dep := privateScanFixture(&cannedScanner{open: []int{1}}, true, time.Nanosecond)
	r.Listeners = &cannedListeners{open: nil}
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}}
	got := sweptPrivatePorts(r, app, dep)
	if got == nil || got.Revision != "rev-2" || len(got.Ports) != 0 {
		t.Fatalf("record = %+v, want rev-2 with no port", got)
	}
}

// A node whose reader fails, refuses the pod or is not Ready is never swept:
// the last record stands and detection retries at its next interval. This is
// the guard against the sweeps coming back through a failure path.
func TestAFailedReadNeverSweeps(t *testing.T) {
	for name, err := range map[string]error{
		"request failed":   errors.New("dial tcp 10.244.3.4:9097: connect: connection refused"),
		"5xx":              errors.New("reader 10.244.3.4:9097 answered 500 Internal Server Error"),
		"pod not on node":  fmt.Errorf("reader: %w", listeners.ErrPodNotFound),
		"address mismatch": fmt.Errorf("reader: %w", listeners.ErrAddressMismatch),
		"reader not Ready": errListenerReaderNotReady,
	} {
		t.Run(name, func(t *testing.T) {
			scanner := &cannedScanner{open: []int{5432, 6000}}
			r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
			r.Listeners = &cannedListeners{err: err}
			before := testutil.ToFloat64(listenerReads.WithLabelValues("reader", readerResult(err)))
			if got := sweptPrivatePorts(r, app, dep); got != nil {
				t.Fatalf("record = %+v after a failed read, want none (the last record stands)", got)
			}
			if scanner.calls != 0 {
				t.Fatalf("a failed read swept the pod %d times", scanner.calls)
			}
			if after := testutil.ToFloat64(listenerReads.WithLabelValues("reader", readerResult(err))); after != before+1 {
				t.Fatalf("reader reads counted %v -> %v, want one more", before, after)
			}

			detector := &cannedScanner{open: []int{8000}}
			dr, dapp, ddep := detectFixture(t, time.Minute, detector, nil)
			dr.Listeners = &cannedListeners{err: err}
			if hint := dr.detectBoundPort(t.Context(), dapp, ddep, appv1alpha1.DefaultPort); hint != noOpenPortMessage(dapp, appv1alpha1.DefaultPort) {
				t.Fatalf("detection hint = %q, want the no-open-port text", hint)
			}
			if detector.calls != 0 || dapp.Status.DetectedPort != 0 {
				t.Fatalf("a failed read swept %d times, detected %d", detector.calls, dapp.Status.DetectedPort)
			}
		})
	}
}

// A node with no reader pod at all is swept as before, and that sweep is
// counted.
func TestANodeWithoutAReaderIsSweptAndCounted(t *testing.T) {
	scanner := &cannedScanner{open: []int{5432, 6000}}
	r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
	r.Listeners = &cannedListeners{err: errNoListenerReader}
	before := testutil.ToFloat64(listenerReads.WithLabelValues("sweep", "ok"))
	got := sweptPrivatePorts(r, app, dep)
	if got == nil || !slices.Equal(got.Ports, []int32{5432, 6000}) {
		t.Fatalf("record = %+v, want the sweep's [5432 6000]", got)
	}
	if scanner.calls != 1 {
		t.Fatalf("scanner called %d times, want 1", scanner.calls)
	}
	if after := testutil.ToFloat64(listenerReads.WithLabelValues("sweep", "ok")); after != before+1 {
		t.Fatalf("sweeps counted %v -> %v, want one more", before, after)
	}
}

// Detection decides on a reader's answer as it did on the sweep's two phases:
// the common HTTP ports (and the probed port) when any of them listens, else
// every listener. So a web service binding 8000 and a metrics port is still
// detected on 8000, not left to choose between them.
func TestBoundPortDetectionNarrowsAReadersAnswerAsTheSweepDid(t *testing.T) {
	for _, tc := range []struct {
		open []int
		also int
		want []int
	}{
		{[]int{4369, 8000, 9100, 50051}, 10000, []int{8000}},
		{[]int{3000, 8080, 9100}, 10000, []int{3000, 8080}},
		{[]int{4369, 7001, 50051}, 7001, []int{7001}},
		{[]int{4369, 50051}, 10000, []int{4369, 50051}},
		{nil, 10000, nil},
	} {
		scanner := &cannedScanner{open: []int{1}}
		r, _, _ := detectFixture(t, time.Minute, scanner, nil)
		r.Listeners = &cannedListeners{open: tc.open}
		got, err := r.boundPortListeners(t.Context(), &corev1.Pod{}, tc.also)
		if err != nil || !slices.Equal(got.Ports, tc.want) || scanner.calls != 0 {
			t.Fatalf("boundPortListeners(%v, also %d) = %v, %v (%d sweeps); want %v and no sweep", tc.open, tc.also, got, err, scanner.calls, tc.want)
		}
	}
}

// A web service bound to 8000 with PORT unset is detected through the reader.
func TestBoundPortDetectedThroughTheReader(t *testing.T) {
	port := httpListenerPort(t)
	scanner := &cannedScanner{open: []int{1}}
	r, app, dep := detectFixture(t, time.Minute, scanner, onLoopback)
	r.Listeners = &cannedListeners{open: []int{port, freePort(t)}}
	for range 2 { // a candidate is adopted when two reads agree
		r.detectBoundPort(t.Context(), app, dep, appv1alpha1.DefaultPort)
	}
	if app.Status.DetectedPort != int32(port) || scanner.calls != 0 {
		t.Fatalf("detected %d after %d sweeps, want %d and none", app.Status.DetectedPort, scanner.calls, port)
	}
}

// A web image's HTTP discovery reads every listener from the reader when no
// declared port answers HTTP.
func TestWebImageHTTPDiscoveryReadsTheReader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	httpPort, _ := strconv.Atoi(portText)
	scanner := &cannedScanner{open: []int{1}}
	r, app, dep := detectFixture(t, time.Minute, scanner, func(a *appv1alpha1.App, pod *corev1.Pod) {
		a.Spec.Type = appv1alpha1.TypeWebService
		a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image", PrimaryPort: 1, Ports: []int32{1}}
		pod.Status.PodIP = "127.0.0.1"
	})
	r.Listeners = &cannedListeners{open: []int{httpPort}}
	r.selectWebImageHTTP(t.Context(), app, dep)
	if network := app.Status.ImageNetwork; network.PrimaryPort != int32(httpPort) || network.HTTPSelection != appv1alpha1.HTTPSelectionDetected || scanner.calls != 0 {
		t.Fatalf("network = %+v after %d sweeps, want %d detected and no sweep", network, scanner.calls, httpPort)
	}
}

// w8/388: when nothing answers HTTP yet, discovery names what the reader found,
// as Render's scan does: a listener only on localhost ("No open ports detected
// on 0.0.0.0"), or open ports that answer no HTTP ("No open HTTP ports
// detected on 0.0.0.0"). With neither, or without a reader to tell, it stays
// the plain wait.
func TestWebImageHTTPDiscoveryNamesWhatItFound(t *testing.T) {
	epmd, silent := epmdListenerPort(t), distributionListenerPort(t)
	// A silent listener below one that answers HTTP: discovery waits on it
	// within its window, and the scan is over, since an HTTP port answers.
	web := httpListenerPort(t)
	for range 64 {
		if web > silent {
			break
		}
		web = httpListenerPort(t)
	}
	if web < silent {
		t.Fatalf("no HTTP listener above the silent port %d in 64 tries", silent)
	}
	for _, tc := range []struct {
		name     string
		reader   *cannedListeners
		want     string
		wantScan appv1alpha1.PortScan
	}{
		{"only localhost listens", &cannedListeners{loopback: []int{10000}},
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortLocalhostOnly, appv1alpha1.DeployReasonParam{Name: "ports", Value: "10000"}),
			appv1alpha1.PortScanLocalhostOnly},
		{"an open port answers no HTTP", &cannedListeners{open: []int{epmd}, loopback: []int{10000}},
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneHTTP, appv1alpha1.DeployReasonParam{Name: "ports", Value: strconv.Itoa(epmd)}),
			appv1alpha1.PortScanNoHTTPPort},
		{"an open port accepts and stays silent", &cannedListeners{open: []int{silent}},
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneHTTP, appv1alpha1.DeployReasonParam{Name: "ports", Value: strconv.Itoa(silent)}),
			appv1alpha1.PortScanNoHTTPPort},
		{"a silent port below one that answers HTTP", &cannedListeners{open: []int{silent, web}},
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNotAnswering, portParam(int32(silent))),
			appv1alpha1.PortScanOver},
		{"nothing listens", &cannedListeners{}, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet), appv1alpha1.PortScanNoOpenPort},
		{"a reader read that failed", &cannedListeners{loopback: []int{10000}, err: listeners.ErrPodNotFound},
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet), appv1alpha1.PortScanNoOpenPort},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, app, dep := detectFixture(t, time.Minute, &cannedScanner{}, func(a *appv1alpha1.App, pod *corev1.Pod) {
				a.Spec.Type = appv1alpha1.TypeWebService
				a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
				a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image", PrimaryPort: 1, Ports: []int32{1}}
				pod.Status.PodIP = "127.0.0.1"
			})
			r.Listeners = tc.reader
			waiting, got := r.selectWebImageHTTP(t.Context(), app, dep)
			if !waiting || got != tc.want {
				t.Fatalf("selectWebImageHTTP = (%v, %q), want (true, %q)", waiting, got, tc.want)
			}
			if scan := appv1alpha1.PortScanOf(got); scan != tc.wantScan {
				t.Fatalf("PortScanOf(%q) = %d, want %d", got, scan, tc.wantScan)
			}
		})
	}
}

// readerPod is a listener-reader pod on node.
func readerPod(name, node, ip string, ready bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "bex-system", Labels: map[string]string{appv1alpha1.LabelComponent: listenerReaderComponent}},
		Spec:       corev1.PodSpec{NodeName: node},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: ip,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}},
	}
}

// NodeListenerReader asks the Ready reader on the pod's own node, with the
// pod's UID and address; a node without one is errNoListenerReader, and one
// whose reader is not Ready errListenerReaderNotReady, which is not.
func TestNodeListenerReaderAsksThePodsNode(t *testing.T) {
	var asked atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Store(r.URL.Query().Get("pod") + "@" + r.URL.Query().Get("ip"))
		_, _ = w.Write([]byte(`{"ports":[5432,50051],"loopback":[9187]}`))
	}))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-1", Namespace: "tea-a", UID: "3f6c1d2e-8a4b-4c5d-9e0f-1a2b3c4d5e6f"},
		Spec: corev1.PodSpec{NodeName: "tenant-a"}, Status: corev1.PodStatus{PodIP: "10.244.1.5"}}
	reader := func(objs ...*corev1.Pod) *NodeListenerReader {
		b := fake.NewClientBuilder()
		for _, o := range objs {
			b = b.WithObjects(o)
		}
		return &NodeListenerReader{Pods: b.Build(), Namespace: "bex-system", HTTP: listeners.NewClient(), Port: uint16(port)}
	}

	got, err := reader(readerPod("r-b", "tenant-b", "10.0.0.1", true), readerPod("r-a", "tenant-a", "127.0.0.1", true)).PodListeners(t.Context(), pod)
	if err != nil || !slices.Equal(got.Ports, []int{5432, 50051}) || !slices.Equal(got.Loopback, []int{9187}) {
		t.Fatalf("PodListeners = %+v, %v; want [5432 50051] and loopback [9187]", got, err)
	}
	if want := string(pod.UID) + "@10.244.1.5"; asked.Load() != want {
		t.Fatalf("reader asked for %v, want %s", asked.Load(), want)
	}
	if _, err := reader(readerPod("r-b", "tenant-b", "127.0.0.1", true)).PodListeners(t.Context(), pod); !errors.Is(err, errNoListenerReader) {
		t.Fatalf("a node whose only reader is elsewhere: %v, want errNoListenerReader", err)
	}
	if _, err := reader(readerPod("r-a", "tenant-a", "127.0.0.1", false)).PodListeners(t.Context(), pod); !errors.Is(err, errListenerReaderNotReady) {
		t.Fatalf("a node whose reader is not Ready: %v, want errListenerReaderNotReady", err)
	}
	// A roll's surge: the old reader terminating, the new one not Ready yet.
	terminating := readerPod("r-old", "tenant-a", "127.0.0.1", true)
	terminating.DeletionTimestamp, terminating.Finalizers = &metav1.Time{Time: time.Now()}, []string{"test"}
	if got, err := reader(terminating, readerPod("r-new", "tenant-a", "", false)).PodListeners(t.Context(), pod); err != nil || len(got.Ports) != 2 {
		t.Fatalf("a node mid-roll = %v, %v; want the Ready reader's answer", got, err)
	}
}
