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
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func webImageApp(generation int64) *appv1alpha1.App {
	return projectionApp(func(a *appv1alpha1.App) {
		a.Name = "whoami"
		a.UID = "fixture"
		a.Generation = generation
		a.Spec.PortMode = appv1alpha1.PortModeWebImageV1
		a.Spec.Image = "docker.io/traefik/whoami:v1.10.3"
		a.Status.ReleaseGeneration = generation
	})
}

// A prebuilt tag is resolved once per release, not once per reconcile: the
// cluster shares one anonymous Docker Hub quota (w4/m243 t008).
func TestWebImageTagReadsTheRegistryOncePerRelease(t *testing.T) {
	requests := 0
	registry := imageConfigRegistry(t, `"80/tcp":{}`)
	r := &AppReconciler{HTTPClient: &http.Client{Transport: imageNetworkTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return registry.Transport.RoundTrip(req)
	})}}
	app := webImageApp(1)
	image, port, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
	if err != nil {
		t.Fatal(err)
	}
	if port != 80 || !strings.HasPrefix(image, "docker.io/traefik/whoami@sha256:") {
		t.Fatalf("image=%q port=%d, want the pinned digest on 80", image, port)
	}
	perRead := requests
	for range 3 {
		again, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
		if err != nil || again != image {
			t.Fatalf("reconcile re-resolved to %q (%v), want the cached %q", again, err, image)
		}
	}
	if requests != perRead {
		t.Fatalf("registry requests = %d after repeated reconciles, want %d", requests, perRead)
	}
	// A new deploy re-reads the tag, as Render re-pulls it.
	app.Status.ReleaseGeneration = 2
	if _, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err != nil {
		t.Fatal(err)
	}
	if requests != 2*perRead {
		t.Fatalf("registry requests = %d after a new release, want %d", requests, 2*perRead)
	}
}

// A registry 429 keeps the deploy open with a waiting reason and honors
// Retry-After; a definitive 404 still fails (w4/m243 t008).
func TestImageRegistryThrottleWaitsInsteadOfFailing(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	for _, tc := range []struct {
		name    string
		status  int
		header  string
		waiting bool
		reason  string
		requeue time.Duration
	}{
		{"rate limited with Retry-After", http.StatusTooManyRequests, "120", true, reasonImageRegistryRateLimited, 2 * time.Minute},
		{"rate limited without Retry-After", http.StatusTooManyRequests, "", true, reasonImageRegistryRateLimited, imageRegistryRetry},
		{"Retry-After is capped", http.StatusTooManyRequests, "86400", true, reasonImageRegistryRateLimited, imageRegistryRetryMax},
		{"server error", http.StatusBadGateway, "", true, reasonImageRegistryUnavailable, imageRegistryRetry},
		{"missing manifest fails", http.StatusNotFound, "", false, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := webImageApp(1)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
			r := &AppReconciler{Client: cl, Scheme: scheme, HTTPClient: &http.Client{Transport: imageNetworkTransport(func(*http.Request) (*http.Response, error) {
				header := make(http.Header)
				if tc.header != "" {
					header.Set("Retry-After", tc.header)
				}
				return &http.Response{StatusCode: tc.status, Header: header, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}}
			_, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
			if err == nil {
				t.Fatal("a refused read resolved")
			}
			res, waiting := r.imageRegistryWait(t.Context(), app, err)
			if waiting != tc.waiting {
				t.Fatalf("waiting = %v for %v, want %v", waiting, err, tc.waiting)
			}
			if !waiting {
				return
			}
			ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
			if res.RequeueAfter != tc.requeue || app.Status.Phase != appv1alpha1.PhaseDeploying || ready == nil || ready.Reason != tc.reason {
				t.Fatalf("requeue=%v phase=%q ready=%+v, want %v Deploying %s", res.RequeueAfter, app.Status.Phase, ready, tc.requeue, tc.reason)
			}
		})
	}
}

// The Ingress and every status port follow the serving network until the
// candidate is promoted, so a PORT=9999 deploy of an image listening on 80
// never takes the host down, and a failed one leaves it on 80 (w4/m243 t007).
func TestWebImageRouteFollowsTheServingNetworkUntilPromotion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := webImageApp(2)
	app.Spec.PortMode = appv1alpha1.PortModeWebImageConfiguredV1
	app.Status.EnvPort = 9999
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:old", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80}}
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:new", Revision: "rev-2", PrimaryPort: 9999, Ports: []int32{80, 9999}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}

	assertServing := func(want int32) {
		t.Helper()
		svc, port, err := r.ingressBackend(t.Context(), app, 9999, false, true)
		if err != nil || svc != app.Name || port != want {
			t.Fatalf("ingress backend = %s:%d (%v), want %s:%d", svc, port, err, app.Name, want)
		}
		if app.ServicePort() != want || !reflect.DeepEqual(app.OpenPorts(), []int32{want}) {
			t.Fatalf("status port=%d openPorts=%v, want %d", app.ServicePort(), app.OpenPorts(), want)
		}
	}
	assertServing(80)

	moved, err := r.promoteImageNetwork(t.Context(), app, "whoami@sha256:new", 9999)
	if err != nil || !moved {
		t.Fatalf("promotion moved=%v err=%v, want the route to move", moved, err)
	}
	assertServing(9999)
	if moved, err := r.promoteImageNetwork(t.Context(), app, "whoami@sha256:new", 9999); err != nil || moved {
		t.Fatalf("re-promotion moved=%v err=%v, want a no-op", moved, err)
	}
}

// A configured-port web service keeps routing to the port it was given.
func TestConfiguredWebRouteIsUnchanged(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.ObjectMeta = metav1.ObjectMeta{Name: "plain", Namespace: "default"} })
	r := &AppReconciler{}
	svc, port, err := r.ingressBackend(t.Context(), app, 8080, false, true)
	if err != nil || svc != "plain" || port != 8080 {
		t.Fatalf("ingress backend = %s:%d (%v), want plain:8080", svc, port, err)
	}
}

// Activity detection reads the Traefik series of the port the App's Ingress
// routes to: an image-port App's image network (w4/m243 t009).
func TestImagePortActivityReadsThePublishedPortSeries(t *testing.T) {
	app := webImageApp(1)
	app.Namespace = "tea-a"
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:x", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80}}
	if q := activityQuery(app, 15*time.Minute); !strings.Contains(q, `tea-a-whoami-80@kubernetes`) || strings.Contains(q, "-10000@") {
		t.Fatalf("activity query = %q, want the published port 80", q)
	}
}
