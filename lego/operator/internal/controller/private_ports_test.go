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
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func privateScanFixture(scanner *cannedScanner, ready bool, every time.Duration) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, Image: "ghcr.io/example/db:1", Port: 5432},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
	labels := map[string]string{labelApp: "db", labelRevision: "rev-2"}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: "db"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-1", Namespace: "tea-a", Labels: labels},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.9",
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}},
	}
	scheme := wakeScheme()
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(), Scheme: scheme, PortScanner: scanner, portDetectEvery: every}
	return r, app, dep
}

func TestScanPrivatePorts(t *testing.T) {
	prev := &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}}
	for name, tc := range map[string]struct {
		scanner *cannedScanner
		ready   bool
		prev    *appv1alpha1.RuntimePortsStatus
		network *appv1alpha1.ImageNetworkStatus // the release's image network
		want    *appv1alpha1.RuntimePortsStatus // nil: no new record, the last one stands
	}{
		"a Ready instance's listeners, reserved ports and duplicates dropped": {
			scanner: &cannedScanner{open: []int{9187, 5432, 19099, 18013, 9187}}, ready: true,
			want: &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}},
		},
		"an unready instance is not scanned": {
			scanner: &cannedScanner{open: []int{9187}}, ready: false,
		},
		"an unreachable instance keeps the last set": {
			scanner: &cannedScanner{err: portscan.ErrUnreachable}, ready: true, prev: prev,
		},
		"a scan that ran out of time adds, never removes": {
			scanner: &cannedScanner{open: []int{5432, 7000}, err: fmt.Errorf("%w: %w", portscan.ErrPartial, context.DeadlineExceeded)}, ready: true, prev: prev,
			want: &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 7000, 9187}},
		},
		"a full scan of the same revision drops a closed listener": {
			scanner: &cannedScanner{open: []int{5432}}, ready: true, prev: prev,
			want: &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432}},
		},
		"a new release's unfinished first sweep keeps its declared ports": {
			scanner: &cannedScanner{open: []int{8123}, err: fmt.Errorf("%w: %w", portscan.ErrPartial, context.DeadlineExceeded)}, ready: true,
			network: &appv1alpha1.ImageNetworkStatus{Revision: "rev-2", PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}},
			prev:    &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{8123, 9009, 9363}},
			want:    &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{8123, 9000, 9009}, Retained: []int32{9363}},
		},
		"another revision's network adds nothing to an unfinished sweep": {
			scanner: &cannedScanner{open: []int{8123}, err: fmt.Errorf("%w: %w", portscan.ErrPartial, context.DeadlineExceeded)}, ready: true,
			network: &appv1alpha1.ImageNetworkStatus{Revision: "rev-1", PrimaryPort: 8123, Ports: []int32{8123, 9000}},
			want:    &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{8123}},
		},
		"a new revision retains what only the old one serves": {
			scanner: &cannedScanner{open: []int{5432, 9200}}, ready: true,
			prev: &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{5432, 9187}, Retained: []int32{7000}},
			want: &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9200}, Retained: []int32{7000, 9187}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := privateScanFixture(tc.scanner, tc.ready, time.Nanosecond)
			app.Status.RuntimePorts, app.Status.ImageNetwork = tc.prev, tc.network
			got := r.scanPrivatePorts(context.Background(), app, dep)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("scan = %+v, want no new record", got)
				}
				return
			}
			if got == nil || got.Revision != tc.want.Revision || !slices.Equal(got.Ports, tc.want.Ports) || !slices.Equal(got.Retained, tc.want.Retained) {
				t.Fatalf("scan = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// A scanned revision is rescanned only after the interval; a new revision is
// scanned at once.
func TestScanPrivatePortsSpacing(t *testing.T) {
	scanner := &cannedScanner{open: []int{5432}}
	r, app, dep := privateScanFixture(scanner, true, time.Hour)
	app.Status.RuntimePorts = r.scanPrivatePorts(context.Background(), app, dep)
	if app.Status.RuntimePorts == nil || r.scanPrivatePorts(context.Background(), app, dep) != nil || scanner.calls != 1 {
		t.Fatalf("calls = %d, want one scan inside the interval", scanner.calls)
	}
	dep.Spec.Template.Labels[labelRevision] = "rev-3"
	r.Client = fake.NewClientBuilder().WithObjects(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-3", Namespace: "tea-a", Labels: dep.Spec.Template.Labels},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.10",
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}).Build()
	if got := r.scanPrivatePorts(context.Background(), app, dep); got == nil || got.Revision != "rev-3" || scanner.calls != 2 {
		t.Fatalf("new revision: %+v after %d calls, want an immediate scan", got, scanner.calls)
	}
}

// w4/517: a due rescan of a revision already scanned waits while the App is not
// Running — a wake keeps its revision, and the sweep held Running back while the
// woken pod served — and runs once it is. A new revision is still scanned at once.
func TestScanPrivatePortsWaitsForRunningOnAKnownRevision(t *testing.T) {
	scanner := &cannedScanner{open: []int{5432}}
	r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432}}
	r.privatePortScanned.Store(app.Namespace+"/"+app.Name, time.Now().Add(-time.Hour))
	for _, phase := range []appv1alpha1.AppPhase{appv1alpha1.PhaseDeploying, appv1alpha1.PhaseHibernated} {
		app.Status.Phase = phase
		if got := r.scanPrivatePorts(context.Background(), app, dep); got != nil || scanner.calls != 0 {
			t.Fatalf("%s: scan = %+v after %d calls, want none", phase, got, scanner.calls)
		}
	}
	app.Status.Phase = appv1alpha1.PhaseRunning
	if got := r.scanPrivatePorts(context.Background(), app, dep); got == nil || scanner.calls != 1 {
		t.Fatalf("Running: scan = %+v after %d calls, want one", got, scanner.calls)
	}
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.RuntimePorts.Revision = "rev-1"
	if got := r.scanPrivatePorts(context.Background(), app, dep); got == nil || got.Revision != "rev-2" || scanner.calls != 2 {
		t.Fatalf("new revision while Deploying: scan = %+v after %d calls, want one", got, scanner.calls)
	}
}

// A failed scan or a type without a network address publishes nothing new, and a
// complete rollout drops the retained set without a scan.
func TestDiscoverPrivatePortsSettles(t *testing.T) {
	r, app, dep := privateScanFixture(&cannedScanner{err: errors.New("boom")}, true, time.Nanosecond)
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432}}
	if err := r.discoverPrivatePorts(context.Background(), app, dep, 5432, false); err != nil || len(app.Status.RuntimePorts.Ports) != 1 {
		t.Fatalf("err=%v status=%+v, want the last set kept", err, app.Status.RuntimePorts)
	}
	app.Status.RuntimePorts.Retained = []int32{9187}
	// The rollout completed: the retained 9187 leaves the Service and status.
	if err := r.discoverPrivatePorts(context.Background(), app, dep, 5432, true); err != nil || len(app.Status.RuntimePorts.Retained) != 0 {
		t.Fatalf("err=%v status=%+v, want retained cleared", err, app.Status.RuntimePorts)
	}
	svc := &corev1.Service{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: "db"}, svc); err != nil || len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 5432 {
		t.Fatalf("Service = %+v (%v), want only 5432", svc.Spec.Ports, err)
	}
	worker := app.DeepCopy()
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	if err := r.discoverPrivatePorts(context.Background(), worker, dep, 5432, true); err != nil || worker.Status.RuntimePorts != nil {
		t.Fatalf("a background worker keeps no runtime ports: %+v", worker.Status.RuntimePorts)
	}
}

// A web service's listeners beyond its primary HTTP port are published on its
// Service and in openPorts, as a private service's are (w3/m276); the app's
// own 10000 and the edge cache's reserved 18014 are not, since 10000 stays
// the route to the primary HTTP server.
func TestDiscoverWebServiceListeners(t *testing.T) {
	r, app, dep := privateScanFixture(&cannedScanner{open: []int{8080, 9000, 10000, 18014}}, true, time.Nanosecond)
	app.Spec.Type, app.Spec.Port = appv1alpha1.TypeWebService, 8080
	if err := r.discoverPrivatePorts(context.Background(), app, dep, 8080, true); err != nil {
		t.Fatal(err)
	}
	if got := app.Status.RuntimePorts; got == nil || !slices.Equal(got.Ports, []int32{8080, 9000, 10000}) {
		t.Fatalf("runtime ports = %+v, want the scanned 8080 9000 10000", got)
	}
	if got := app.OpenPorts(); !slices.Equal(got, []int32{8080, 9000}) {
		t.Fatalf("openPorts = %v, want the primary and the extra 9000", got)
	}
	svc := &corev1.Service{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: "db"}, svc); err != nil {
		t.Fatal(err)
	}
	targets := map[int32]intstr.IntOrString{}
	for _, p := range svc.Spec.Ports {
		targets[p.Port] = p.TargetPort
	}
	if len(targets) != 3 || targets[9000] != intstr.FromInt32(9000) || targets[10000] != targets[8080] {
		t.Fatalf("Service ports = %v, want 8080, 10000 on the primary's target, and 9000", targets)
	}
}

func TestWithPrivateNetworkPorts(t *testing.T) {
	named := intstr.FromString(servingPortName)
	for name, tc := range map[string]struct {
		app   appv1alpha1.App
		ports []corev1.ServicePort
		want  map[int32]intstr.IntOrString
	}{
		"a configured web service adds 10000 on its serving listener": {
			app:   appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Port: 8080}},
			ports: []corev1.ServicePort{{Name: servingPortName, Port: 8080, TargetPort: named}},
			want:  map[int32]intstr.IntOrString{8080: named, 10000: named},
		},
		"an edge-cached web service's 10000 goes through the cache too": {
			app:   appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}},
			ports: []corev1.ServicePort{{Name: servingPortName, Port: 3000, TargetPort: intstr.FromString(edgeCachePortName)}},
			want:  map[int32]intstr.IntOrString{3000: intstr.FromString(edgeCachePortName), 10000: intstr.FromString(edgeCachePortName)},
		},
		"a web service already on 10000 is unchanged": {
			app:   appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService}},
			ports: []corev1.ServicePort{{Name: servingPortName, Port: 10000, TargetPort: named}},
			want:  map[int32]intstr.IntOrString{10000: named},
		},
		"an image's declared 10000 is retargeted to its primary": {
			app: appv1alpha1.App{
				Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1},
				Status: appv1alpha1.AppStatus{ServingNetwork: &appv1alpha1.ImageNetworkStatus{Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80, 10000}}},
			},
			ports: []corev1.ServicePort{{Name: "tcp-80", Port: 80, TargetPort: intstr.FromInt32(80)}, {Name: "tcp-10000", Port: 10000, TargetPort: intstr.FromInt32(10000)}},
			want:  map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 10000: intstr.FromInt32(80)},
		},
		"a web service adds its undeclared listeners, its own 10000 still the alias": {
			app: appv1alpha1.App{
				Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Port: 8080},
				Status: appv1alpha1.AppStatus{RuntimePorts: &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{8080, 9000, 10000}}},
			},
			ports: []corev1.ServicePort{{Name: servingPortName, Port: 8080, TargetPort: named}},
			want:  map[int32]intstr.IntOrString{8080: named, 9000: intstr.FromInt32(9000), 10000: named},
		},
		"a private service adds its undeclared listeners and no 10000": {
			app: appv1alpha1.App{
				Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, Port: 5432},
				Status: appv1alpha1.AppStatus{RuntimePorts: &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{5432, 9187}}},
			},
			ports: []corev1.ServicePort{{Name: servingPortName, Port: 5432, TargetPort: named}},
			want:  map[int32]intstr.IntOrString{5432: named, 9187: intstr.FromInt32(9187)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := withPrivateNetworkPorts(&tc.app, tc.ports)
			if got[0].Port != tc.ports[0].Port {
				t.Fatalf("first port = %d, want the primary kept first", got[0].Port)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ports = %+v, want %v", got, tc.want)
			}
			for _, p := range got {
				if want, ok := tc.want[p.Port]; !ok || p.TargetPort != want || p.Protocol != corev1.ProtocolTCP {
					t.Fatalf("port %d -> %v (%s), want %v", p.Port, p.TargetPort, p.Protocol, tc.want)
				}
			}
		})
	}
}

// w4/517: the Ready=True that ends a park is stamped when the woken pod became
// Ready, not when the pass that noticed it wrote status; any other Ready flip,
// and a pod with no usable time, keeps the write's own clock.
func TestWakeReadyIsStampedFromThePod(t *testing.T) {
	parkedAt := metav1.NewTime(time.Now().Add(-10 * time.Minute).Truncate(time.Second))
	podReadyAt := metav1.NewTime(time.Now().Add(-30 * time.Second).Truncate(time.Second))
	for name, tc := range map[string]struct {
		reason  string
		status  metav1.ConditionStatus
		podTime metav1.Time
		want    metav1.Time
	}{
		"auto-hibernated":         {reasonAutoHibernated, metav1.ConditionFalse, podReadyAt, podReadyAt},
		"suspended":               {reasonSuspended, metav1.ConditionFalse, podReadyAt, podReadyAt},
		"a rollout, not a park":   {reasonRolloutProgressing, metav1.ConditionFalse, podReadyAt, metav1.Time{}},
		"already ready":           {"Deployed", metav1.ConditionTrue, podReadyAt, metav1.Time{}},
		"a pod ready before park": {reasonAutoHibernated, metav1.ConditionFalse, metav1.NewTime(parkedAt.Add(-time.Minute)), metav1.Time{}},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := privateScanFixture(&cannedScanner{}, true, time.Hour)
			var pods corev1.PodList
			if err := r.List(context.Background(), &pods); err != nil || len(pods.Items) != 1 {
				t.Fatalf("pods = %v (%v)", pods.Items, err)
			}
			pod := &pods.Items[0]
			pod.Status.Conditions[0].LastTransitionTime = tc.podTime
			if err := r.Status().Update(context.Background(), pod); err != nil {
				t.Fatal(err)
			}
			app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: tc.status, Reason: tc.reason, LastTransitionTime: parkedAt}}
			if got := r.wakeReadySince(context.Background(), app, dep); !got.Equal(&tc.want) {
				t.Fatalf("wakeReadySince = %v, want %v", got, tc.want)
			}
		})
	}
}

// w3/m273 t010: a private image-port service whose process binds only a port
// its image does not declare has that listener chosen as its primary once two
// scans of the unready candidate agree. PORT, a configured port, an HTTP
// check, a primary that answers (ClickHouse's 8123 beside 9000) and a web
// service are left alone.
func TestDetectPrivatePrimary(t *testing.T) {
	private := func(primary int32, ports ...int32) func(*appv1alpha1.App, *corev1.Pod) {
		return func(a *appv1alpha1.App, _ *corev1.Pod) {
			a.Spec.Type, a.Spec.PortMode, a.Spec.Port = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1, 0
			a.Status.ReleaseGeneration = 1
			a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{
				Image: "i@sha256:x", Revision: "rev-1", PrimaryPort: primary, Ports: ports, ConfiguredPort: new(int32(0)),
			}
		}
	}
	t.Run("an undeclared listener becomes the primary", func(t *testing.T) {
		r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{8080}}, private(80, 80))
		if hint := r.detectPrivatePrimary(t.Context(), app, dep, 80); app.Status.ImageNetwork.PrimaryPort != 80 || !strings.Contains(hint, "port 8080") {
			t.Fatalf("first scan: primary=%d hint=%q, want only a candidate", app.Status.ImageNetwork.PrimaryPort, hint)
		}
		time.Sleep(time.Millisecond)
		hint := r.detectPrivatePrimary(t.Context(), app, dep, 80)
		network := app.Status.ImageNetwork
		if network.PrimaryPort != 8080 || !slices.Equal(network.Ports, []int32{80, 8080}) || !strings.Contains(hint, "now uses") {
			t.Fatalf("second scan: network=%+v hint=%q, want primary 8080 over [80 8080]", network, hint)
		}
	})
	for name, tc := range map[string]struct {
		open   []int
		mutate func(*appv1alpha1.App, *corev1.Pod)
	}{
		"ClickHouse's declared primary answers": {open: []int{8123, 9000}, mutate: private(8123, 8123, 9000, 9009)},
		"nothing answers":                       {open: nil, mutate: private(80, 80)},
		"a PORT variable": {open: []int{9090}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			private(8080, 80, 8080)(a, p)
			a.Status.EnvPort = 8080
		}},
		"a configured port": {open: []int{9090}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			private(80, 80)(a, p)
			a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeImageConfiguredV1, 80
		}},
		"an HTTP health check": {open: []int{8080}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			private(80, 80)(a, p)
			a.Spec.HealthCheckPath = "/healthz"
		}},
		"a web image service": {open: []int{8080}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			private(80, 80)(a, p)
			a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypeWebService, appv1alpha1.PortModeWebImageV1
		}},
		"a ready instance": {open: []int{8080}, mutate: func(a *appv1alpha1.App, p *corev1.Pod) {
			private(80, 80)(a, p)
			p.Status.ContainerStatuses[0].Ready = true
		}},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: tc.open}, tc.mutate)
			want := app.Status.ImageNetwork.DeepCopy()
			for range 3 {
				r.detectPrivatePrimary(t.Context(), app, dep, int(want.PrimaryPort))
				time.Sleep(time.Millisecond)
			}
			if got := app.Status.ImageNetwork; got.PrimaryPort != want.PrimaryPort || !slices.Equal(got.Ports, want.Ports) {
				t.Fatalf("network = %+v, want unchanged %+v", got, want)
			}
		})
	}
}
