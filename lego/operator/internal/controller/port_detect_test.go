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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type cannedScanner struct {
	open  []int
	err   error
	calls int
}

func (c *cannedScanner) Scan(context.Context, string) ([]int, error) {
	c.calls++
	return c.open, c.err
}

func detectFixture(t *testing.T, started time.Duration, scanner portscan.Scanner, mutate func(*appv1alpha1.App, *corev1.Pod)) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:1", Port: appv1alpha1.DefaultPort},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: "web", labelRevision: "rev-1"}}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "tea-a", Labels: map[string]string{labelApp: "web", labelRevision: "rev-1"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning, PodIP: "10.0.0.7",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-started))}},
			}},
		},
	}
	if mutate != nil {
		mutate(app, pod)
	}
	r := &AppReconciler{Client: fake.NewClientBuilder().WithObjects(pod).Build(), PortScanner: scanner, portDetectEvery: time.Nanosecond}
	return r, app, dep
}

// onLoopback points the detect fixture's pod at the test's own listeners.
func onLoopback(_ *appv1alpha1.App, pod *corev1.Pod) { pod.Status.PodIP = "127.0.0.1" }

// httpListenerPort is a loopback HTTP server's port, standing in for a
// process's HTTP listener on the pod.
func httpListenerPort(t *testing.T) int {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	t.Cleanup(server.Close)
	return server.Listener.Addr().(*net.TCPAddr).Port
}

// rawListenerPort is a loopback listener that hands each connection to handle
// and then closes it: a listener that speaks some other protocol than HTTP.
func rawListenerPort(t *testing.T, handle func(net.Conn)) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	active := map[net.Conn]struct{}{}
	var tasks sync.WaitGroup
	tasks.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			active[conn] = struct{}{}
			mu.Unlock()
			tasks.Go(func() {
				defer func() { _ = conn.Close(); mu.Lock(); delete(active, conn); mu.Unlock() }()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				handle(conn)
			})
		}
	})
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		for conn := range active {
			_ = conn.Close()
		}
		mu.Unlock()
		tasks.Wait()
	})
	return listener.Addr().(*net.TCPAddr).Port
}

// epmdListenerPort behaves as Erlang's epmd does on a request it cannot read:
// the first two bytes are a request length ("GE" is 18245, beyond its buffer),
// so it closes the connection without a word.
func epmdListenerPort(t *testing.T) int {
	return rawListenerPort(t, func(conn net.Conn) { _, _ = io.ReadFull(conn, make([]byte, 2)) })
}

// distributionListenerPort behaves as an Erlang distribution listener does: it
// waits for the handshake's 18245 bytes and answers nothing.
func distributionListenerPort(t *testing.T) int {
	return rawListenerPort(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) })
}

// w8/177: a web service whose check keeps failing on the default port and whose
// process serves HTTP on exactly one other port moves to that port once two
// scans agree; the serving port then follows it. A listener beside it that
// does not answer HTTP is not counted (w32/037).
func TestDetectBoundPortAdoptsALoneListener(t *testing.T) {
	web := httpListenerPort(t)
	scanner := &cannedScanner{open: []int{epmdListenerPort(t), web}}
	r, app, dep := detectFixture(t, time.Minute, scanner, onLoopback)

	if hint := r.detectBoundPort(context.Background(), app, dep, 10000); app.Status.DetectedPort != 0 || hint != " The process is listening on port "+strconv.Itoa(web)+"." {
		t.Fatalf("first scan: DetectedPort=%d hint=%q, want only the HTTP candidate %d", app.Status.DetectedPort, hint, web)
	}
	time.Sleep(time.Millisecond)
	hint := r.detectBoundPort(context.Background(), app, dep, 10000)
	if app.Status.DetectedPort != int32(web) || !strings.Contains(hint, "now uses") {
		t.Fatalf("second scan: DetectedPort=%d hint=%q, want %d adopted", app.Status.DetectedPort, hint, web)
	}
	if got := detectedServingPort(app, 10000); got != web {
		t.Fatalf("serving port = %d, want the detected %d", got, web)
	}
	if got := app.ServingPort(); got != int32(web) {
		t.Fatalf("App.ServingPort = %d, want %d", got, web)
	}
}

// w32/037: Render's phoenix_hello release starts Erlang distribution seconds
// after its container does (epmd on 4369 and a distribution listener on a
// random port), and Phoenix binds PORT about a minute later on a free
// instance. Port detection adopted epmd, the TCP check passed on it, the
// deploy went live and every request answered 502. Render routes a web
// service to an HTTP port only and keeps scanning ("No open HTTP ports
// detected on 0.0.0.0, continuing to scan"), and so does bex now; once
// Phoenix binds PORT the scan stands down.
func TestDetectBoundPortNeverAdoptsAListenerThatDoesNotAnswerHTTP(t *testing.T) {
	t.Parallel()
	epmd, distribution := epmdListenerPort(t), distributionListenerPort(t)
	for name, open := range map[string][]int{
		"epmd alone, as the scans that adopted it found": {epmd},
		"epmd and the distribution listener":             {epmd, distribution},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scanner := &cannedScanner{open: open}
			r, app, dep := detectFixture(t, time.Minute, scanner, onLoopback)
			for scan := range 2 {
				hint := r.detectBoundPort(t.Context(), app, dep, 10000)
				if app.Status.DetectedPort != 0 {
					t.Fatalf("scan %d adopted port %d, which does not answer HTTP", scan+1, app.Status.DetectedPort)
				}
				if want := noOpenPortMessage(app, 10000); hint != want {
					t.Fatalf("scan %d: hint = %q, want %q", scan+1, hint, want)
				}
				time.Sleep(time.Millisecond)
			}
			if got := app.ServingPort(); got != appv1alpha1.DefaultPort {
				t.Fatalf("App.ServingPort = %d, want PORT's %d", got, appv1alpha1.DefaultPort)
			}

			scanner.open = slices.Concat(open, []int{10000})
			if hint := r.detectBoundPort(t.Context(), app, dep, 10000); hint != "" || app.Status.DetectedPort != 0 {
				t.Fatalf("with PORT bound: hint=%q DetectedPort=%d, want nothing to add", hint, app.Status.DetectedPort)
			}
		})
	}
}

func TestDetectBoundPortLeavesAmbiguousOrConfiguredServicesAlone(t *testing.T) {
	web, metrics := httpListenerPort(t), httpListenerPort(t)
	for name, tc := range map[string]struct {
		open    []int
		mutate  func(*appv1alpha1.App, *corev1.Pod)
		started time.Duration
	}{
		"two HTTP listeners are named, not chosen":     {open: []int{web, metrics}, started: time.Minute, mutate: onLoopback},
		"the configured port answers (a path problem)": {open: []int{10000, 8000}, started: time.Minute},
		"nothing answers":              {open: nil, started: time.Minute},
		"an explicit port":             {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Port = 3000 }},
		"an explicit non-default port": {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Port = 8080 }},
		"a default PORT variable":      {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Status.EnvPort = appv1alpha1.DefaultPort }},
		"a PORT variable":              {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Status.EnvPort = 3000 }},
		"a worker":                     {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Type = appv1alpha1.TypeBackgroundWorker }},
		"a booting pod":                {open: []int{8000}, started: time.Second},
		"a cached service": {open: []int{8000}, started: time.Minute, mutate: func(_ *appv1alpha1.App, p *corev1.Pod) {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: edgeCacheContainerName})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			scanner := &cannedScanner{open: tc.open}
			r, app, dep := detectFixture(t, tc.started, scanner, tc.mutate)
			for range 3 {
				r.detectBoundPort(context.Background(), app, dep, 10000)
				time.Sleep(time.Millisecond)
			}
			if app.Status.DetectedPort != 0 {
				t.Fatalf("DetectedPort = %d, want none", app.Status.DetectedPort)
			}
		})
	}
}

// An explicit port or PORT variable wins over, and clears, a stale detection.
func TestExplicitPortClearsADetectedOne(t *testing.T) {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}, Status: appv1alpha1.AppStatus{DetectedPort: 8000}}
	if got := detectedServingPort(app, 3000); got != 3000 || app.Status.DetectedPort != 0 {
		t.Fatalf("serving port %d, DetectedPort %d; want 3000 and cleared", got, app.Status.DetectedPort)
	}
}

// w8/259: an instance with nothing listening where the platform can reach it
// (no server, or one bound to 127.0.0.1) is told so in Render's terms, whether
// the scan came back empty or unreachable. On a tenant pod a closed port drops
// the connect, so both are the same case; the stall text used to blame port
// detection for the second.
func TestDetectBoundPortNamesAnInstanceWithNoOpenPort(t *testing.T) {
	for name, scanner := range map[string]*cannedScanner{
		"the scan found nothing":   {},
		"the scan was unreachable": {err: portscan.ErrUnreachable},
	} {
		r, app, dep := detectFixture(t, time.Minute, scanner, nil)
		got := r.detectBoundPort(t.Context(), app, dep, 10000)
		for _, want := range []string{"No open port was detected", "0.0.0.0", "PORT (10000)", "background worker"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: stall text = %q, want %q in it", name, got, want)
			}
		}
		if strings.Contains(got, "could not reach the instance") {
			t.Errorf("%s: stall text still blames port detection: %q", name, got)
		}
		if app.Status.DetectedPort != 0 {
			t.Errorf("%s: DetectedPort = %d, want none", name, app.Status.DetectedPort)
		}
	}
}

func TestDetectBoundPortWithoutAPersistedDefault(t *testing.T) {
	web := httpListenerPort(t)
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{web}}, func(a *appv1alpha1.App, pod *corev1.Pod) {
		a.Spec.Port = 0
		onLoopback(a, pod)
	})
	r.detectBoundPort(t.Context(), app, dep, 10000)
	r.detectBoundPort(t.Context(), app, dep, 10000)
	if app.Status.DetectedPort != int32(web) || app.ServingPort() != int32(web) {
		t.Fatalf("detected=%d serving=%d, want %d", app.Status.DetectedPort, app.ServingPort(), web)
	}
}

// w8/267: a detected port decides where the platform routes, never what PORT
// the process is given. Render leaves PORT at its default; bex handed the
// detected port back as PORT, so the instance was restarted with a different
// environment than the one it was found in, and an app with a public listener
// on 8080 and an internal one on $PORT then bound 8080 twice and crash-looped.
func TestADetectedPortRoutesButIsNotGivenBackAsPort(t *testing.T) {
	portOf := func(app *appv1alpha1.App, serving int) string {
		env := appEnv(app, serving)
		last := env[len(env)-1]
		if last.Name != portEnvName {
			t.Fatalf("PORT is not last: %+v", env)
		}
		return last.Value
	}
	detected := &appv1alpha1.App{Status: appv1alpha1.AppStatus{DetectedPort: 8080}}
	if got := portOf(detected, 8080); got != "10000" {
		t.Errorf("PORT for a detected port = %s, want the default 10000", got)
	}
	if hint := crashHintPort(detected, 8080); hint != 0 {
		t.Errorf("crash hint port = %d, want none: the process is not expected on $PORT", hint)
	}
	if msg := crashLoopMessage(&corev1.Pod{}, corev1.ContainerStatus{}, crashHintPort(detected, 8080)); strings.Contains(msg, "$PORT") {
		t.Errorf("the crash message still says the process must listen on $PORT: %q", msg)
	}

	// A port the service names is both routed and handed to the process.
	for name, tc := range map[string]struct {
		app     *appv1alpha1.App
		serving int
	}{
		"an explicit port":          {&appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}}, 3000},
		"the service's own PORT":    {&appv1alpha1.App{Status: appv1alpha1.AppStatus{EnvPort: 4000}}, 4000},
		"the default, undetected":   {&appv1alpha1.App{}, 10000},
		"a stale detection, unused": {&appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}, Status: appv1alpha1.AppStatus{DetectedPort: 8080}}, 3000},
	} {
		if got := portOf(tc.app, tc.serving); got != strconv.Itoa(tc.serving) {
			t.Errorf("%s: PORT = %s, want %d", name, got, tc.serving)
		}
		if hint := crashHintPort(tc.app, tc.serving); hint != tc.serving {
			t.Errorf("%s: crash hint port = %d, want %d", name, hint, tc.serving)
		}
	}
}

// w4/615: a web image service whose image chooses its own HTTP port gets
// Render's PORT=10000 whatever port its image declares or HTTP discovery
// selects; only the routing and probes follow that port, and a crash hint
// never names it as $PORT. A configured port, the service's own PORT, a
// private image service and a web network resolved before w4/615 keep PORT on
// the primary.
func TestAWebImagePortIsRoutedButPORTStaysTheDefault(t *testing.T) {
	webImage := func(mode string, network appv1alpha1.ImageNetworkStatus, envPort int32) *appv1alpha1.App {
		return &appv1alpha1.App{
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: mode},
			Status: appv1alpha1.AppStatus{EnvPort: envPort, ImageNetwork: &network},
		}
	}
	scanned := func(primary int32, selection string, ports ...int32) appv1alpha1.ImageNetworkStatus {
		return appv1alpha1.ImageNetworkStatus{PrimaryPort: primary, Ports: ports, HTTPSelection: selection, ProcessPort: appv1alpha1.DefaultPort}
	}
	for name, tc := range map[string]struct {
		app             *appv1alpha1.App
		serving         int
		wantPORT, wantH int
	}{
		"an HTTP port discovery selected": {webImage(appv1alpha1.PortModeWebImageV1, scanned(8080, appv1alpha1.HTTPSelectionDetected, 8080), 0), 8080, 10000, 0},
		"an EXPOSE-declared primary":      {webImage(appv1alpha1.PortModeWebImageV1, scanned(80, "", 80), 0), 80, 10000, 0},
		"no EXPOSE, still the default":    {webImage(appv1alpha1.PortModeWebImageV1, scanned(10000, "", 10000), 0), 10000, 10000, 10000},
		"a configured port":               {webImage(appv1alpha1.PortModeWebImageConfiguredV1, appv1alpha1.ImageNetworkStatus{PrimaryPort: 3000, Ports: []int32{3000}}, 0), 3000, 3000, 3000},
		"the service's own PORT":          {webImage(appv1alpha1.PortModeWebImageV1, scanned(4000, appv1alpha1.HTTPSelectionConfigured, 4000), 4000), 4000, 4000, 4000},
		"a private image-v1 service": {&appv1alpha1.App{
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, PortMode: appv1alpha1.PortModeImageV1},
			Status: appv1alpha1.AppStatus{ImageNetwork: &appv1alpha1.ImageNetworkStatus{PrimaryPort: 3000, Ports: []int32{3000}}},
		}, 3000, 3000, 3000},
		"a web network from before w4/615": {webImage(appv1alpha1.PortModeWebImageV1, appv1alpha1.ImageNetworkStatus{PrimaryPort: 80, Ports: []int32{80}, HTTPSelection: appv1alpha1.HTTPSelectionDetected}, 0), 80, 80, 80},
	} {
		env := appEnv(tc.app, tc.serving)
		if last := env[len(env)-1]; last.Name != portEnvName || last.Value != strconv.Itoa(tc.wantPORT) {
			t.Errorf("%s: PORT = %+v, want %d", name, last, tc.wantPORT)
		}
		if hint := crashHintPort(tc.app, tc.serving); hint != tc.wantH {
			t.Errorf("%s: crash hint port = %d, want %d", name, hint, tc.wantH)
		}
	}
}

// On a service whose serving port was detected (3000), PORT stays 10000
// (w8/267), so a later deploy that binds nothing reachable is told to bind
// PORT (10000), and the detected port is named as where it listened before.
// It used to say "the port in PORT (3000)" (w8/281).
func TestNoOpenPortTextNamesThePortTheProcessReceives(t *testing.T) {
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{}, nil)
	app.Status.DetectedPort = 3000
	got := r.detectBoundPort(t.Context(), app, dep, 3000)
	for _, want := range []string{"the port in PORT (10000)", "it listened on port 3000 before", "background worker"} {
		if !strings.Contains(got, want) {
			t.Errorf("stall text = %q, want %q in it", got, want)
		}
	}
	if strings.Contains(got, "PORT (3000)") {
		t.Errorf("stall text names the detected port as PORT: %q", got)
	}

	// A service whose port is its own PORT keeps that number, and nothing else.
	app.Status.DetectedPort = 0
	if got := noOpenPortMessage(app, 10000); !strings.Contains(got, "the port in PORT (10000).") || strings.Contains(got, "before") {
		t.Errorf("stall text for an undetected port = %q", got)
	}
}

// privateService makes the detect fixture's App a private service that names
// no port: no PORT, no image port mode, the stored default 10000.
func privateService(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Type = appv1alpha1.TypePrivateService }

// w8/m504: a private service that names no port is probed on 10000 only, so a
// process bound elsewhere never went Ready and failed at the 15-minute start
// limit: a service switched from a native build to redis:7-alpine, or a native
// one that binds 6379. Render serves a private service on any port it binds,
// so its listener is adopted as a web service's is, once two scans agree, and
// without asking it to answer HTTP. PORT stays the default (w8/267).
func TestDetectBoundPortAdoptsAPrivateServicesListener(t *testing.T) {
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{6379}}, privateService)

	if hint := r.detectBoundPort(t.Context(), app, dep, 10000); app.Status.DetectedPort != 0 || hint != " The process is listening on port 6379." {
		t.Fatalf("first scan: DetectedPort=%d hint=%q, want only the candidate 6379", app.Status.DetectedPort, hint)
	}
	time.Sleep(time.Millisecond)
	hint := r.detectBoundPort(t.Context(), app, dep, 10000)
	if app.Status.DetectedPort != 6379 || !strings.Contains(hint, "now uses") {
		t.Fatalf("second scan: DetectedPort=%d hint=%q, want 6379 adopted", app.Status.DetectedPort, hint)
	}
	if got := detectedServingPort(app, 10000); got != 6379 {
		t.Fatalf("serving port = %d, want the detected 6379", got)
	}
	if got := app.ServingPort(); got != 6379 {
		t.Fatalf("App.ServingPort = %d, want 6379", got)
	}
	if got := app.OpenPorts(); !slices.Equal(got, []int32{6379}) {
		t.Fatalf("App.OpenPorts = %v, want [6379]", got)
	}
	if got := processPort(app, 6379); got != appv1alpha1.DefaultPort {
		t.Fatalf("PORT = %d, want the default %d", got, appv1alpha1.DefaultPort)
	}
}

// A private service that binds several ports is served on the one an
// image-port private service's primary would be: 3000 when open, else the
// lowest, never a reserved port (preferredPort, w3/m273 t010).
func TestDetectBoundPortChoosesAPrivateServicesListener(t *testing.T) {
	for name, tc := range map[string]struct {
		open []int
		want int32
	}{
		"3000 when it is open":  {open: []int{2000, 3000, 9090}, want: 3000},
		"else the lowest":       {open: []int{9187, 6379}, want: 6379},
		"never a reserved port": {open: []int{18012, 6379}, want: 6379},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: tc.open}, privateService)
			for range 2 {
				r.detectBoundPort(t.Context(), app, dep, 10000)
				time.Sleep(time.Millisecond)
			}
			if app.Status.DetectedPort != tc.want {
				t.Fatalf("DetectedPort = %d, want %d", app.Status.DetectedPort, tc.want)
			}
		})
	}
}

// A private service whose own configuration names its port keeps waiting on
// it, one whose port answers has nothing to detect, and an image-port private
// service is left to detectPrivatePrimary.
func TestDetectBoundPortLeavesConfiguredPrivateServicesAlone(t *testing.T) {
	for name, tc := range map[string]struct {
		open   []int
		mutate func(*appv1alpha1.App)
	}{
		"an explicit port":            {[]int{6379}, func(a *appv1alpha1.App) { a.Spec.Port = 6380 }},
		"a PORT variable":             {[]int{6379}, func(a *appv1alpha1.App) { a.Status.EnvPort = 7000 }},
		"a default PORT variable":     {[]int{6379}, func(a *appv1alpha1.App) { a.Status.EnvPort = appv1alpha1.DefaultPort }},
		"an image port mode":          {[]int{6379}, func(a *appv1alpha1.App) { a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeImageV1, 0 }},
		"a configured image port":     {[]int{6379}, func(a *appv1alpha1.App) { a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeImageConfiguredV1, 6380 }},
		"the configured port answers": {[]int{6379, 10000}, func(*appv1alpha1.App) {}},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: tc.open}, func(a *appv1alpha1.App, p *corev1.Pod) {
				privateService(a, p)
				tc.mutate(a)
			})
			for range 3 {
				if hint := r.detectBoundPort(t.Context(), app, dep, 10000); hint != "" {
					t.Fatalf("hint = %q, want nothing to add", hint)
				}
				time.Sleep(time.Millisecond)
			}
			if app.Status.DetectedPort != 0 {
				t.Fatalf("DetectedPort = %d, want none", app.Status.DetectedPort)
			}
		})
	}
}

// A private service whose server binds only 127.0.0.1 never answers on the
// pod address, so the scan finds nothing, or nothing at all answers: no port
// is adopted, and the stall says what to bind, in Render's terms.
func TestDetectBoundPortNamesAPrivateServiceWithNoOpenPort(t *testing.T) {
	for name, scanner := range map[string]*cannedScanner{
		"a loopback-only listener": {},
		"an unreachable instance":  {err: portscan.ErrUnreachable},
		"only a reserved port":     {open: []int{18012}},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := detectFixture(t, time.Minute, scanner, privateService)
			for range 2 {
				hint := r.detectBoundPort(t.Context(), app, dep, 10000)
				for _, want := range []string{"No open port was detected", "0.0.0.0", "PORT (10000)", "background worker"} {
					if !strings.Contains(hint, want) {
						t.Fatalf("stall text = %q, want %q in it", hint, want)
					}
				}
				time.Sleep(time.Millisecond)
			}
			if app.Status.DetectedPort != 0 {
				t.Fatalf("DetectedPort = %d, want none", app.Status.DetectedPort)
			}
		})
	}
}

// A metrics port a private service opens before its server is never adopted
// from one scan: the choice moves to the server's port once it binds, and that
// port is adopted only when the next scan agrees.
func TestDetectBoundPortWaitsOutAPrivateServicesEarlyMetricsPort(t *testing.T) {
	scanner := &cannedScanner{open: []int{9090}}
	r, app, dep := detectFixture(t, time.Minute, scanner, privateService)
	for scan, open := range [][]int{{9090}, {6379, 9090}} {
		scanner.open = open
		if hint := r.detectBoundPort(t.Context(), app, dep, 10000); app.Status.DetectedPort != 0 || !strings.Contains(hint, "is listening on port") {
			t.Fatalf("scan %d of %v: DetectedPort=%d hint=%q, want a candidate only", scan+1, open, app.Status.DetectedPort, hint)
		}
		time.Sleep(time.Millisecond)
	}
	r.detectBoundPort(t.Context(), app, dep, 10000)
	if app.Status.DetectedPort != 6379 {
		t.Fatalf("DetectedPort = %d, want the server's 6379 once two scans agree", app.Status.DetectedPort)
	}
}

// A private service switched back from an image to a native build that names
// its port, with spec.port or PORT, serves that port, and the detection left
// from the image is cleared, as on a web service.
func TestAPrivateServicesExplicitPortClearsADetectedOne(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*appv1alpha1.App)
		want   int
	}{
		"an explicit port": {func(a *appv1alpha1.App) { a.Spec.Port = 8080 }, 8080},
		"a PORT variable":  {func(a *appv1alpha1.App) { a.Status.EnvPort = 7000 }, 7000},
	} {
		app := &appv1alpha1.App{
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, Port: appv1alpha1.DefaultPort},
			Status: appv1alpha1.AppStatus{DetectedPort: 6379},
		}
		tc.mutate(app)
		if got := detectedServingPort(app, tc.want); got != tc.want || app.Status.DetectedPort != 0 {
			t.Errorf("%s: serving port %d, DetectedPort %d; want %d and cleared", name, got, app.Status.DetectedPort, tc.want)
		}
		if got := app.ServingPort(); got != int32(tc.want) {
			t.Errorf("%s: App.ServingPort = %d, want %d", name, got, tc.want)
		}
	}
}

// A private service switched to redis, which serves 6379, and rolled back to
// its native release, which binds PORT 10000, is probed on the detected 6379
// at first; the scan of its unready instance finds 10000 and the service serves
// it again.
func TestARolledBackPrivateServiceServesItsNativePortAgain(t *testing.T) {
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{10000}}, func(a *appv1alpha1.App, p *corev1.Pod) {
		privateService(a, p)
		a.Status.DetectedPort = 6379
	})
	port := detectedServingPort(app, appv1alpha1.DefaultPort)
	if port != 6379 {
		t.Fatalf("serving port before the scan = %d, want the detected 6379", port)
	}
	for range 2 {
		r.detectBoundPort(t.Context(), app, dep, port)
		time.Sleep(time.Millisecond)
	}
	if app.ServingPort() != appv1alpha1.DefaultPort || !slices.Equal(app.OpenPorts(), []int32{appv1alpha1.DefaultPort}) {
		t.Fatalf("serving %d, openPorts %v; want the native release's 10000", app.ServingPort(), app.OpenPorts())
	}
	if got := processPort(app, int(app.ServingPort())); got != appv1alpha1.DefaultPort {
		t.Fatalf("PORT = %d, want 10000", got)
	}
}
