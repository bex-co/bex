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
		"a new revision retains what only the old one serves": {
			scanner: &cannedScanner{open: []int{5432, 9200}}, ready: true,
			prev: &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{5432, 9187}, Retained: []int32{7000}},
			want: &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9200}, Retained: []int32{7000, 9187}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, app, dep := privateScanFixture(tc.scanner, tc.ready, time.Nanosecond)
			app.Status.RuntimePorts = tc.prev
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

// A failed scan or a non-private service publishes nothing new, and a
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
	web := app.DeepCopy()
	web.Spec.Type = appv1alpha1.TypeWebService
	if err := r.discoverPrivatePorts(context.Background(), web, dep, 5432, true); err != nil || web.Status.RuntimePorts != nil {
		t.Fatalf("a web service keeps no runtime ports: %+v", web.Status.RuntimePorts)
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
