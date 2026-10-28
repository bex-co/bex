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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestWebImagePendingHTTPSelectionGatesEveryProbe(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selection string
		mode      string
		envPort   int32
		path      string
		wantHTTP  bool
	}{
		{name: "pending binary primary", mode: appv1alpha1.PortModeWebImageV1, wantHTTP: true},
		{name: "HTTP selected", mode: appv1alpha1.PortModeWebImageV1, selection: "http"},
		{name: "fallback selected", mode: appv1alpha1.PortModeWebImageV1, selection: "fallback"},
		{name: "existing serving choice retained", mode: appv1alpha1.PortModeWebImageV1, selection: "retained"},
		{name: "configured choice", mode: appv1alpha1.PortModeWebImageConfiguredV1},
		{name: "explicit PORT", mode: appv1alpha1.PortModeWebImageV1, envPort: 10000},
		{name: "configured health path", mode: appv1alpha1.PortModeWebImageV1, path: "/healthz", wantHTTP: true},
		{name: "private service", mode: appv1alpha1.PortModeImageV1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := webImageApp(1)
			app.Spec.PortMode = tc.mode
			app.Spec.HealthCheckPath = tc.path
			app.Status.EnvPort = tc.envPort
			if tc.mode == appv1alpha1.PortModeImageV1 {
				app.Spec.Type = appv1alpha1.TypePrivateService
			}
			app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "candidate@sha256:new", Revision: "rev-1", PrimaryPort: 4369, Ports: []int32{4369, 15672}, HTTPSelection: tc.selection}
			container := appContainer(app, deploymentParams{image: "candidate@sha256:new", port: 4369}, "web.example.test")
			for _, probe := range probesOf(container) {
				if probe == nil {
					t.Fatal("network service has no probe")
				}
				if tc.wantHTTP {
					wantPath := tc.path
					if wantPath == "" {
						wantPath = "/"
					}
					if probe.TCPSocket != nil || probe.HTTPGet == nil || probe.HTTPGet.Path != wantPath || probe.HTTPGet.Port.IntValue() != 4369 {
						t.Fatalf("probe=%+v, want HTTP %s on provisional4369", probe.ProbeHandler, wantPath)
					}
					if len(probe.HTTPGet.HTTPHeaders) != 1 || probe.HTTPGet.HTTPHeaders[0].Name != "Host" || probe.HTTPGet.HTTPHeaders[0].Value != "web.example.test" {
						t.Fatalf("probe lost platform Host: %+v", probe.HTTPGet)
					}
				} else if probe.HTTPGet != nil || probe.TCPSocket == nil || probe.TCPSocket.Port.IntValue() != 4369 {
					t.Fatalf("resolved/configured probe=%+v, want ordinaryTCP", probe.ProbeHandler)
				}
			}
		})
	}
}

func TestWebImagePendingHTTPSelectionHoldsTheServingPods(t *testing.T) {
	for _, tc := range []struct {
		name      string
		serving   bool
		selection string
		ports     []int32
		primary   int32
		want      int32
	}{
		{"pending replacement with different ports", true, "", []int32{80, 3000}, 3000, 150},
		{"pending replacement with the same ports", true, "", []int32{3000}, 3000, 150},
		{"resolved port change", true, "http", []int32{80, 3000}, 80, portChangeHoldSeconds},
		{"resolved unchanged port", true, "http", []int32{3000}, 3000, 0},
		{"first release", false, "", []int32{4369, 15672}, 4369, 0},
		{"legacy choice retained", true, "retained", []int32{3000}, 3000, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := webImageApp(2)
			if tc.serving {
				app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image@sha256:old", Revision: "rev-1", PrimaryPort: 3000, Ports: []int32{3000}}
			}
			app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "image@sha256:new", Revision: releaseRevision(app), PrimaryPort: tc.primary, Ports: tc.ports, HTTPSelection: tc.selection}
			dep := project(app, deploymentParams{image: app.Status.ImageNetwork.Image, port: int(tc.primary), replicas: 1})
			if dep.Spec.MinReadySeconds != tc.want {
				t.Fatalf("minReadySeconds=%d, want%d", dep.Spec.MinReadySeconds, tc.want)
			}
		})
	}
}
