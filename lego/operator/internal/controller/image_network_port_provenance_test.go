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
	"net/http"
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The digest stays the same through these changes. A new release, including a
// rollback, must resolve the selected release's choice rather than the cached
// PORT; otherwise the runtime injects that removed value back into the process.
func TestImageNetworkPortConfigurationTransitions(t *testing.T) {
	for _, source := range []string{"PORT environment", "explicit port"} {
		t.Run(source, func(t *testing.T) {
			requests := 0
			registry := imageConfigRegistry(t, `"80/tcp":{}`)
			r := &AppReconciler{HTTPClient: &http.Client{Transport: imageNetworkTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				return registry.Transport.RoundTrip(req)
			})}}
			app := webImageApp(1)
			image := app.Spec.Image
			for _, configured := range []int32{0, 8080, 9090, 0} {
				app.Status.ReleaseGeneration++
				app.Status.EnvPort = configured
				if source == "explicit port" {
					app.Status.EnvPort = 0
					app.Spec.PortMode = appv1alpha1.PortModeWebImageV1
					if configured != 0 {
						app.Spec.PortMode, app.Spec.Port = appv1alpha1.PortModeWebImageConfiguredV1, configured
					}
				}
				before := requests
				var err error
				var port int
				image, port, err = r.resolveImageNetwork(t.Context(), app, image)
				if err != nil {
					t.Fatal(err)
				}
				wantPort, wantPorts := int(configured), []int32{80, configured}
				if configured == 0 {
					wantPort, wantPorts = 80, []int32{80}
				}
				network := app.Status.ImageNetwork
				if port != wantPort || !reflect.DeepEqual(network.Ports, wantPorts) || network.ConfiguredPort == nil || *network.ConfiguredPort != configured {
					t.Fatalf("configuration %d resolved port=%d network=%+v, want %d %v with provenance %d", configured, port, network, wantPort, wantPorts, configured)
				}
				if requests == before {
					t.Fatalf("changed configuration %d reused the previous network", configured)
				}
				before = requests
				if _, _, err := r.resolveImageNetwork(t.Context(), app, image); err != nil {
					t.Fatal(err)
				}
				if requests != before {
					t.Fatalf("unchanged configuration %d reread immutable metadata", configured)
				}
			}
		})
	}
}

// Missing provenance is deliberately ambiguous: m141 could mark an old PORT as
// detected HTTP after that stale value had already been injected into the pod.
func TestImageNetworkLegacyPortChoiceRefreshesOnce(t *testing.T) {
	for _, selection := range []string{"", appv1alpha1.HTTPSelectionDetected, appv1alpha1.HTTPSelectionFallback, appv1alpha1.HTTPSelectionRetained} {
		for _, location := range []string{"cached", "serving"} {
			t.Run(location+"/"+selection, func(t *testing.T) {
				registry := imageConfigRegistry(t, `"80/tcp":{}`)
				r := &AppReconciler{HTTPClient: registry}
				app := webImageApp(1)
				image, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
				if err != nil {
					t.Fatal(err)
				}
				legacy := app.Status.ImageNetwork.DeepCopy()
				legacy.ConfiguredPort = nil
				legacy.PrimaryPort, legacy.Ports, legacy.HTTPSelection = 9090, []int32{80, 9090}, selection
				app.Status.ImageNetwork = legacy
				if location == "serving" {
					app.Status.ServingNetwork, app.Status.ImageNetwork = legacy, nil
				}
				requests := 0
				r.HTTPClient = &http.Client{Transport: imageNetworkTransport(func(req *http.Request) (*http.Response, error) {
					requests++
					return registry.Transport.RoundTrip(req)
				})}
				for range 2 {
					_, port, err := r.resolveImageNetwork(t.Context(), app, image)
					if err != nil {
						t.Fatal(err)
					}
					network := app.Status.ImageNetwork
					if port != 80 || !reflect.DeepEqual(network.Ports, []int32{80}) || network.HTTPSelection != "" || network.ConfiguredPort == nil || *network.ConfiguredPort != 0 {
						t.Fatalf("ambiguous legacy state reintroduced removed PORT: %+v", network)
					}
				}
				if requests != 2 {
					t.Fatalf("migration made %d registry requests, want one manifest/config read", requests)
				}
			})
		}
	}
}

func TestImageNetworkMatchingLegacyHTTPDecisionSurvivesRefresh(t *testing.T) {
	for _, location := range []string{"cached", "serving"} {
		t.Run(location, func(t *testing.T) {
			registry := imageConfigRegistry(t, `"80/tcp":{}`)
			r := &AppReconciler{HTTPClient: registry}
			app := webImageApp(1)
			image, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
			if err != nil {
				t.Fatal(err)
			}
			legacy := app.Status.ImageNetwork.DeepCopy()
			legacy.ConfiguredPort, legacy.HTTPSelection = nil, appv1alpha1.HTTPSelectionDetected
			app.Status.ImageNetwork = legacy
			if location == "serving" {
				app.Status.ServingNetwork, app.Status.ImageNetwork = legacy, nil
			}
			_, port, err := r.resolveImageNetwork(t.Context(), app, image)
			if err != nil {
				t.Fatal(err)
			}
			network := app.Status.ImageNetwork
			if port != 80 || network.ConfiguredPort == nil || *network.ConfiguredPort != 0 || network.HTTPSelection != appv1alpha1.HTTPSelectionDetected {
				t.Fatalf("fresh matching metadata discarded a completed legacy HTTP choice: %+v", network)
			}
		})
	}
}
