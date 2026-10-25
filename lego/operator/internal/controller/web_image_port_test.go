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
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m243: a web service in web-image mode serves on its image's exposed
// port, not the platform default.
func TestWebImagePortResolution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		exposed string
		primary int
		ports   []int32
	}{
		{"hello-go exposes 3000", `"3000/tcp":{}`, 3000, []int32{3000}},
		{"whoami exposes 80", `"80/tcp":{}`, 80, []int32{80}},
		{"no declarations fall back to the default port", ``, 10000, []int32{10000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, tc.exposed)}
			app := projectionApp(func(a *appv1alpha1.App) {
				a.Name = "webby"
				a.Generation = 1
				a.Spec.Type = appv1alpha1.TypeWebService
				a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
			})
			_, port, err := r.resolveImageNetwork(t.Context(), app, "registry.test/webby:gen-1")
			if err != nil {
				t.Fatal(err)
			}
			if port != tc.primary || !reflect.DeepEqual(app.Status.ImageNetwork.Ports, tc.ports) {
				t.Fatalf("primary=%d ports=%v, want %d %v", port, app.Status.ImageNetwork.Ports, tc.primary, tc.ports)
			}
			if got := app.ServicePort(); got != int32(tc.primary) {
				t.Fatalf("ServicePort = %d, want %d", got, tc.primary)
			}
		})
	}
}

// A web service without the mode keeps the configured-port behavior.
func TestWebWithoutImageModeKeepsDefaultPort(t *testing.T) {
	r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, `"3000/tcp":{}`)}
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Name = "plain"
		a.Spec.Type = appv1alpha1.TypeWebService
	})
	_, port, err := r.resolveImageNetwork(t.Context(), app, "registry.test/plain:gen-1")
	if err != nil {
		t.Fatal(err)
	}
	if port != 10000 || app.Status.ImageNetwork != nil {
		t.Fatalf("port=%d network=%v, want the default with no image network", port, app.Status.ImageNetwork)
	}
}
