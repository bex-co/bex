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

package v1alpha1

import (
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenPorts, ServicePort and InternalAddress are one derivation read three
// ways: the published set, its primary, and the `<slug>:<primary>` address.
// They follow the serving release during a replacement and never invent a
// port while discovery is pending (w6/m148).
func TestOpenPortsFollowTheServingNetwork(t *testing.T) {
	candidate := &ImageNetworkStatus{Image: "new", Revision: "r2", PrimaryPort: 9000, Ports: []int32{9000, 9100}}
	serving := &ImageNetworkStatus{Image: "old", Revision: "r1", PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	for _, tc := range []struct {
		name    string
		app     App
		ports   []int32
		primary int32
		address string
	}{
		{"configured web", App{ObjectMeta: metav1.ObjectMeta{Name: "web"}, Spec: AppSpec{Type: TypeWebService, Port: 8080}}, []int32{8080}, 8080, "web:8080"},
		{"configured default", App{ObjectMeta: metav1.ObjectMeta{Name: "pserv"}, Spec: AppSpec{Type: TypePrivateService}}, []int32{DefaultPort}, DefaultPort, "pserv:10000"},
		{"image pending", App{ObjectMeta: metav1.ObjectMeta{Name: "ch"}, Spec: AppSpec{Type: TypePrivateService, PortMode: PortModeImageV1}}, []int32{}, 0, ""},
		{"image first candidate", App{ObjectMeta: metav1.ObjectMeta{Name: "ch"}, Spec: AppSpec{Type: TypePrivateService, PortMode: PortModeImageV1},
			Status: AppStatus{ImageNetwork: candidate}}, []int32{9000, 9100}, 9000, "ch:9000"},
		{"image replacement keeps serving", App{ObjectMeta: metav1.ObjectMeta{Name: "ch"}, Spec: AppSpec{Type: TypePrivateService, PortMode: PortModeImageV1},
			Status: AppStatus{ImageNetwork: candidate, ServingNetwork: serving}}, []int32{8123, 9000, 9009}, 8123, "ch:8123"},
		{"worker has no network", App{ObjectMeta: metav1.ObjectMeta{Name: "w"}, Spec: AppSpec{Type: TypeBackgroundWorker}}, nil, 3000, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.app.OpenPorts(); !reflect.DeepEqual(got, tc.ports) {
				t.Fatalf("OpenPorts = %#v, want %#v", got, tc.ports)
			}
			if tc.app.Spec.InternallyAddressable() && tc.app.ServicePort() != tc.primary {
				t.Fatalf("ServicePort = %d, want %d", tc.app.ServicePort(), tc.primary)
			}
			if got := tc.app.InternalAddress(); got != tc.address {
				t.Fatalf("InternalAddress = %q, want %q", got, tc.address)
			}
		})
	}
	// The returned slice is a copy: a caller cannot mutate the status through it.
	app := App{Spec: AppSpec{Type: TypePrivateService, PortMode: PortModeImageV1}, Status: AppStatus{ServingNetwork: serving}}
	app.OpenPorts()[0] = 1
	if serving.Ports[0] != 8123 {
		t.Fatal("OpenPorts aliased the serving network")
	}
}

func TestServingPortPrefersTheServicesOwnPORT(t *testing.T) {
	app := &App{Spec: AppSpec{Port: 3000}}
	if got := app.ServingPort(); got != 3000 {
		t.Fatalf("ServingPort = %d, want spec.port 3000", got)
	}
	app.Status.EnvPort = 8080
	if app.ServingPort() != 8080 || app.ServicePort() != 8080 || app.OpenPorts()[0] != 8080 {
		t.Fatalf("a recorded PORT must drive every reader: serving=%d service=%d open=%v",
			app.ServingPort(), app.ServicePort(), app.OpenPorts())
	}
	if !strings.HasSuffix(app.InternalAddress(), ":8080") {
		t.Fatalf("InternalAddress = %q, want the PORT", app.InternalAddress())
	}
}

// ReceivesPrivateTraffic is false for every free-plan App (an empty tier is
// free) and for types with no internal address (w8/m130).
func TestReceivesPrivateTraffic(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec AppSpec
		want bool
	}{
		{"free web", AppSpec{Type: TypeWebService, Tier: "free"}, false},
		{"empty tier is free", AppSpec{Type: TypeWebService}, false},
		{"paid web", AppSpec{Type: TypeWebService, Tier: "starter"}, true},
		{"paid private", AppSpec{Type: TypePrivateService, Tier: "standard"}, true},
		{"paid worker", AppSpec{Type: TypeBackgroundWorker, Tier: "starter"}, false},
	} {
		if got := tc.spec.ReceivesPrivateTraffic(); got != tc.want {
			t.Errorf("%s: ReceivesPrivateTraffic = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A web image-port service publishes its serving network's primary while a
// candidate's PORT rolls out: the candidate is not serving yet (w4/m243 t007).
func TestWebImagePortsPublishTheServingNetwork(t *testing.T) {
	app := &App{Spec: AppSpec{Type: TypeWebService, PortMode: PortModeWebImageConfiguredV1}}
	app.Status.EnvPort = 9999
	if app.ServicePort() != 9999 || app.OpenPorts()[0] != 9999 {
		t.Fatalf("before discovery the PORT is the only answer: service=%d open=%v", app.ServicePort(), app.OpenPorts())
	}
	app.Status.ImageNetwork = &ImageNetworkStatus{Image: "new", Revision: "rev-2", PrimaryPort: 9999, Ports: []int32{80, 9999}}
	app.Status.ServingNetwork = &ImageNetworkStatus{Image: "old", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80, 443}}
	if app.ServicePort() != 80 || len(app.OpenPorts()) != 1 || app.OpenPorts()[0] != 80 {
		t.Fatalf("service=%d open=%v, want the serving 80", app.ServicePort(), app.OpenPorts())
	}
}
