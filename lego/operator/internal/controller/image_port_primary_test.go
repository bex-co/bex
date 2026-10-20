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
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// imageConfigRegistry serves one image whose config declares exposed.
func imageConfigRegistry(t *testing.T, exposed string) *http.Client {
	t.Helper()
	config := fmt.Sprintf(`{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{%s}}}`, exposed)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(config)))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest)
	return &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		body := manifest
		if strings.HasSuffix(request.URL.Path, "/blobs/"+configDigest) {
			body = config
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

// The primary rule (ADR089 § Port slice decisions, w6/m148): an explicit port,
// else 3000 when declared, else the lowest declared TCP port — ports below 1024
// included, because tenant nodes run ip_unprivileged_port_start=0 — else 3000.
// Every declared TCP port is published, and the strict policy adds nothing.
func TestImagePortPrimarySelection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		exposed   string
		explicit  int32
		primary   int
		published []int32
	}{
		{"low port is bindable and lowest", `"80/tcp":{},"8123/tcp":{},"9000/tcp":{}`, 0, 80, []int32{80, 8123, 9000}},
		{"declared 3000 wins", `"3000/tcp":{},"8080/tcp":{}`, 0, 3000, []int32{3000, 8080}},
		{"no declarations fall back to 3000", ``, 0, 3000, []int32{3000}},
		{"udp is not published", `"53/udp":{},"9000/tcp":{}`, 0, 9000, []int32{9000}},
		{"explicit port selects the primary and keeps the rest", `"8123/tcp":{},"9000/tcp":{}`, 9100, 9100, []int32{8123, 9000, 9100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, tc.exposed)}
			app := projectionApp(func(a *appv1alpha1.App) {
				a.Name = "multi"
				a.Generation = 1
				a.Spec.Type = appv1alpha1.TypePrivateService
				a.Spec.PortMode = appv1alpha1.PortModeImageV1
				if tc.explicit != 0 {
					a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeImageConfiguredV1, tc.explicit
				}
			})
			image, port, err := r.resolveImageNetwork(t.Context(), app, "registry.test/multi:gen-1")
			if err != nil {
				t.Fatal(err)
			}
			if port != tc.primary || !reflect.DeepEqual(app.Status.ImageNetwork.Ports, tc.published) {
				t.Fatalf("primary=%d ports=%v, want %d %v", port, app.Status.ImageNetwork.Ports, tc.primary, tc.published)
			}
			container := appContainer(app, deploymentParams{image: image, port: port, replicas: 1}, "")
			if container.StartupProbe.TCPSocket == nil || container.StartupProbe.TCPSocket.Port.IntValue() != tc.primary {
				t.Fatalf("health check is not on the primary: %+v", container.StartupProbe)
			}
			if caps := container.SecurityContext.Capabilities; len(caps.Add) != 0 || !reflect.DeepEqual(caps.Drop, appSecCtx(appv1alpha1.AppSpec{}).Capabilities.Drop) {
				t.Fatalf("an image-port service under the strict policy gained capabilities: %+v", caps)
			}
			for _, env := range container.Env {
				if env.Name == portEnvName && env.Value != fmt.Sprint(tc.primary) {
					t.Fatalf("PORT=%s, want the primary %d", env.Value, tc.primary)
				}
			}
		})
	}
}

// A TCP check that never connects on an image-port service names the primary
// and the declared ports (w6/m148), and the terminal rewrite still applies.
func TestImagePortStallHint(t *testing.T) {
	image := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Type = appv1alpha1.TypePrivateService
		a.Spec.PortMode = appv1alpha1.PortModeImageV1
		a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "i", Revision: "r", PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	})
	hint := imagePortStallHint(image, 8123)
	for _, want := range []string{"port 8123", "(8123, 9000, 9009)", "0.0.0.0"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint %q lacks %q", hint, want)
		}
	}
	msg := "the container is running but its readiness health check " + probeStallWaiting + ": a TCP connect to port 8123." + hint
	if got := terminalStallMessage(reasonHealthCheckFailing, msg); !strings.Contains(got, probeStallFailed) || !strings.Contains(got, "(8123, 9000, 9009)") {
		t.Fatalf("terminal diagnosis lost the outcome or the ports: %q", got)
	}
	httpCheck := image.DeepCopy()
	httpCheck.Spec.HealthCheckPath = "/ping"
	configured := projectionApp(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService })
	pending := image.DeepCopy()
	pending.Status.ImageNetwork = nil
	for name, app := range map[string]*appv1alpha1.App{"http check": httpCheck, "configured port": configured, "pending metadata": pending} {
		if hint := imagePortStallHint(app, 8123); hint != "" {
			t.Errorf("%s: unexpected hint %q", name, hint)
		}
	}
}
