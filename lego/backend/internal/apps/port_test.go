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

package apps

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func portApp(name, serviceType string, port int32) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type = serviceType
	a.Spec.Port = port
	return a
}

// TestPortIsEditableAfterCreate is w4/m121's core regression. The port was
// create-only on every surface: 23 Set… mutators and no SetPort, no `port` on
// the PATCH body (`unknown field "port"`), and no way to read it back
// (`Cannot query field "port" on type "Service"`). So an off-the-shelf image
// binding anything but 3000 was hostable through the create API and nowhere
// else — and bex's own reserved-PORT refusal told every caller to "change the
// service port instead", naming a setting that did not exist.
func TestPortIsEditableAfterCreate(t *testing.T) {
	svc, cl := newService(nil, portApp("web", appv1alpha1.TypeWebService, 3000))
	ctx := context.Background()

	view, err := svc.SetPort(ctx, "web", 8080)
	if err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if view.Port != 8080 {
		t.Errorf("returned view port = %d, want 8080", view.Port)
	}

	var a appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.Spec.Port != 8080 {
		t.Errorf("spec.port = %d, want 8080", a.Spec.Port)
	}
	// The port is release identity: the container binds PORT at startup, so a
	// change that did not roll the pods would leave the process on the old port
	// while the Service, Ingress and probe had already moved.
	if a.Spec.RestartedAt == "" {
		t.Error("a port change must bump restartedAt, or the running pods keep the old PORT")
	}
}

// TestPortIsReadableOnTheServiceView closes the other half of the gap: the
// number was only observable as the suffix of internalAddress, so a caller who
// had just set a port at create could not read it back. The two must agree —
// they are the same field, and a reader who compares them is entitled to find
// them equal.
func TestPortIsReadableOnTheServiceView(t *testing.T) {
	svc, _ := newService(nil, portApp("web", appv1alpha1.TypeWebService, 8080))
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.Port != 8080 {
		t.Fatalf("view.Port = %d, want 8080", view.Port)
	}
	if !strings.HasSuffix(view.InternalAddress, fmt.Sprintf(":%d", view.Port)) {
		t.Errorf("internalAddress %q does not end in the published port %d", view.InternalAddress, view.Port)
	}

	// A service that never named a port reads the platform default, not zero —
	// the number the operator actually injects.
	svc, _ = newService(nil, portApp("plain", appv1alpha1.TypeWebService, 0))
	view, err = svc.Get(context.Background(), "plain")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.Port != appv1alpha1.DefaultPort {
		t.Errorf("unset port reads %d, want the platform default %d", view.Port, appv1alpha1.DefaultPort)
	}
}

func TestImagePortViewFollowsServingRelease(t *testing.T) {
	for _, tc := range []struct {
		name               string
		candidate, serving *appv1alpha1.ImageNetworkStatus
		want               int32
	}{
		{name: "unresolved"},
		{name: "first discovery", candidate: &appv1alpha1.ImageNetworkStatus{PrimaryPort: 8123}, want: 8123},
		{name: "pending replacement", candidate: &appv1alpha1.ImageNetworkStatus{PrimaryPort: 9000}, serving: &appv1alpha1.ImageNetworkStatus{PrimaryPort: 8123}, want: 8123},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := portApp("clickhouse", appv1alpha1.TypePrivateService, 3000)
			app.Spec.PortMode = appv1alpha1.PortModeImageV1
			app.Status.ImageNetwork, app.Status.ServingNetwork = tc.candidate, tc.serving
			svc, _ := newService(nil, app)
			view, err := svc.Get(t.Context(), "clickhouse")
			if err != nil {
				t.Fatal(err)
			}
			if view.Port != tc.want {
				t.Fatalf("port = %d, want %d", view.Port, tc.want)
			}
			if tc.want == 0 && view.InternalAddress != "" || tc.want != 0 && !strings.HasSuffix(view.InternalAddress, fmt.Sprintf(":%d", tc.want)) {
				t.Fatalf("address %q disagrees with port %d", view.InternalAddress, tc.want)
			}
		})
	}
}

func TestSetImagePrimaryPortPersistsExplicitIntent(t *testing.T) {
	app := portApp("clickhouse", appv1alpha1.TypePrivateService, 3000)
	app.Spec.PortMode = appv1alpha1.PortModeImageV1
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-clickhouse"}
	st := &recordingStore{}
	svc, cl := newService(st, app)
	view, err := svc.SetPort(t.Context(), "clickhouse", 3000)
	if err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "clickhouse")
	if got.Spec.PortMode != appv1alpha1.PortModeImageConfiguredV1 || got.Spec.Port != 3000 {
		t.Fatalf("explicit 3000 lost: mode=%s port=%d", got.Spec.PortMode, got.Spec.Port)
	}
	if len(st.portCalls) != 1 || st.portCalls[0].id != "srv-clickhouse" || st.portCalls[0].port != 3000 || st.portCalls[0].mode != appv1alpha1.PortModeImageConfiguredV1 {
		t.Fatalf("missing durable port update: %+v", st.portCalls)
	}
	if view.Port != 8123 || !strings.HasSuffix(view.InternalAddress, ":8123") {
		t.Fatal("port update replaced the serving address before readiness")
	}
}

func TestImagePrimaryPortRejectsReservedUpdateWithoutWrites(t *testing.T) {
	for _, port := range []int32{18012, 18013, 19099} {
		for _, patch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/patch=%t", port, patch), func(t *testing.T) {
				app := portApp("clickhouse", appv1alpha1.TypePrivateService, 3000)
				app.Spec.PortMode = appv1alpha1.PortModeImageV1
				app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-clickhouse"}
				st := &recordingStore{}
				svc, cl := newService(st, app)
				before := getApp(t, cl, "clickhouse")
				var err error
				if patch {
					autoDeploy := !before.Spec.AutoDeploy
					_, err = svc.ApplyServicePatch(t.Context(), "clickhouse", ServicePatch{AutoDeploy: &autoDeploy, Port: &port})
				} else {
					_, err = svc.SetPort(t.Context(), "clickhouse", port)
				}
				if !errors.Is(err, core.ErrBadRequest) {
					t.Fatalf("reserved port %d: got %v, want bad request", port, err)
				}
				after := getApp(t, cl, "clickhouse")
				if !reflect.DeepEqual(before.Spec, after.Spec) || before.ResourceVersion != after.ResourceVersion || len(st.portCalls) != 0 {
					t.Fatal("rejected image port changed the service or durable port")
				}
			})
		}
	}
}

func TestLegacyPrivatePortKeepsConfiguredContract(t *testing.T) {
	svc, _ := newService(nil, portApp("legacy", appv1alpha1.TypePrivateService, 3000))
	svc.ImageCompatibilityWorkspaces = map[string]bool{"*": true}
	if _, err := svc.SetPort(t.Context(), "legacy", 18012); err != nil {
		t.Fatalf("creation gate changed an existing configured-port service: %v", err)
	}
}

// TestPortlessTypesHaveNoPortToChange pins the DoD's per-type table. A worker
// or cron job runs no server and a static site is served by the shared
// static-server on its own port, so for those a port is meaningless rather
// than merely unused: the verb refuses instead of writing a field the operator
// will ignore, and the read publishes nothing.
func TestPortlessTypesHaveNoPortToChange(t *testing.T) {
	for _, tc := range []struct {
		serviceType string
		addressable bool
	}{
		{"", true}, // the empty-type default is a web service
		{appv1alpha1.TypeWebService, true},
		{appv1alpha1.TypePrivateService, true},
		{appv1alpha1.TypeBackgroundWorker, false},
		{appv1alpha1.TypeCronJob, false},
		{appv1alpha1.TypeStaticSite, false},
	} {
		name := tc.serviceType
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			svc, _ := newService(nil, portApp("svc", tc.serviceType, 3000))
			_, err := svc.SetPort(context.Background(), "svc", 8080)
			view, getErr := svc.Get(context.Background(), "svc")
			if getErr != nil {
				t.Fatalf("Get: %v", getErr)
			}

			if tc.addressable {
				if err != nil {
					t.Fatalf("SetPort on an addressable type: %v", err)
				}
				if view.Port == 0 {
					t.Error("an addressable type must publish its port")
				}
				return
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("SetPort on %q = %v, want ErrBadRequest", tc.serviceType, err)
			}
			if view.Port != 0 {
				t.Errorf("%q published port %d; it has no listener", tc.serviceType, view.Port)
			}
		})
	}
}

// TestPortBoundsAreRefusedBeforeAnyWrite: an update has no "0 means default"
// case — an explicit zero is a caller asking for a port that cannot be bound,
// not an omission, which is the one place update's validation deliberately
// differs from create's.
func TestPortBoundsAreRefusedBeforeAnyWrite(t *testing.T) {
	for _, port := range []int32{0, -1, 65536, 1 << 20} {
		svc, cl := newService(nil, portApp("web", appv1alpha1.TypeWebService, 3000))
		if _, err := svc.SetPort(context.Background(), "web", port); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetPort(%d) = %v, want ErrBadRequest", port, err)
			continue
		}
		var a appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
			t.Fatalf("get: %v", err)
		}
		if a.Spec.Port != 3000 {
			t.Errorf("a refused port write changed the spec to %d", a.Spec.Port)
		}
	}
}

// TestReservedPortRefusalNamesAReachableControl is w4/m121's point, stated as a
// test: the sentence exists to redirect a caller, so it has to name something
// that exists. It is asserted on the sentence rather than on any one surface
// because every surface renders this exact string.
func TestReservedPortRefusalNamesAReachableControl(t *testing.T) {
	sentence := core.ReservedEnvKeySentence("PORT")
	for _, want := range []string{
		"setPort",        // the GraphQL mutation
		"update_service", // the MCP tool
		"/v1/services",   // the REST route
		"Settings",       // the dashboard control
	} {
		if !strings.Contains(sentence, want) {
			t.Errorf("the reserved-PORT sentence does not name %q — it sends the reader nowhere:\n%s", want, sentence)
		}
	}
}
