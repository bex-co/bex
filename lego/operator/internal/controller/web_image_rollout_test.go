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

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
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

// Every tenant container Always-pulls, a digest-pinned one included: a node
// that already holds a private image must still have the registry check the
// pod's own credentials before it runs (w4/426).
func TestTenantContainersAlwaysPullEvenADigest(t *testing.T) {
	const digest = "ghcr.io/acme/private@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	web := project(projectionApp(), deploymentParams{image: digest, port: 10000})
	if got := web.Spec.Template.Spec.Containers[0].ImagePullPolicy; got != corev1.PullAlways {
		t.Fatalf("web container pull policy = %q, want Always", got)
	}
	cron := (&AppReconciler{}).cronPodSpec(projectionApp(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeCronJob }), digest, 0, nil, launchMode(""), "")
	if got := cron.Spec.Containers[0].ImagePullPolicy; got != corev1.PullAlways {
		t.Fatalf("cron container pull policy = %q, want Always", got)
	}
	job := predeploy.Job(predeploy.Options{Name: "web", Namespace: "tea-a", Image: digest, Command: []string{"migrate"}})
	if got := job.Spec.Template.Spec.Containers[0].ImagePullPolicy; got != corev1.PullAlways {
		t.Fatalf("pre-deploy container pull policy = %q, want Always", got)
	}
}

// A prebuilt-image web service runs the digest its saved tag resolved to; that
// is the saved image, not an undeployed change (w4/m243 t014).
func TestServedDigestOfTheSavedTagIsTheSavedImage(t *testing.T) {
	const digest = "docker.io/mendhak/http-https-echo@sha256:82b7"
	app := webImageApp(2)
	app.Spec.Image = "docker.io/mendhak/http-https-echo:latest"
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: digest, Source: app.Spec.Image, Revision: "rev-2", PrimaryPort: 8080, Ports: []int32{8080}}
	for _, tc := range []struct {
		name   string
		mutate func(*appv1alpha1.App)
		served string
		want   bool
	}{
		{"the digest resolved from the saved tag", func(*appv1alpha1.App) {}, digest, true},
		{"the saved tag itself", func(*appv1alpha1.App) {}, "docker.io/mendhak/http-https-echo:latest", true},
		{"a saved image change", func(a *appv1alpha1.App) { a.Spec.Image = "docker.io/mendhak/http-https-echo:v2" }, digest, false},
		{"an override resolved from another reference", func(a *appv1alpha1.App) {
			a.Status.ServingNetwork.Source = "docker.io/mendhak/http-https-echo:canary"
		}, digest, false},
		{"a network recorded before Source", func(a *appv1alpha1.App) { a.Status.ServingNetwork.Source = "" }, digest, false},
		{"a configured-port image service runs its tag", func(a *appv1alpha1.App) { a.Spec.PortMode = "" }, digest, false},
	} {
		a := app.DeepCopy()
		tc.mutate(a)
		if got := servesSavedImage(a, tc.served); got != tc.want {
			t.Errorf("%s: servesSavedImage = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Through a same-port rollout the stable Services select every App pod, so
// the new pod is an endpoint before the Deployment drains the old one; only a
// port change pins the serving revision, so no request reaches a pod without
// its listener (w4/m243 t012).
func TestImagePortSelectorPinsARevisionOnlyForAPortChange(t *testing.T) {
	serving := &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:a", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80}}
	for _, tc := range []struct {
		name      string
		candidate *appv1alpha1.ImageNetworkStatus
		pinned    bool
	}{
		{"a redeploy on the same ports", &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:a", Revision: "rev-2", PrimaryPort: 80, Ports: []int32{80}}, false},
		{"a new image on the same ports", &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:b", Revision: "rev-2", PrimaryPort: 80, Ports: []int32{80}}, false},
		{"a new primary", &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:a", Revision: "rev-2", PrimaryPort: 9999, Ports: []int32{80, 9999}}, true},
		{"a new port set", &appv1alpha1.ImageNetworkStatus{Image: "whoami@sha256:b", Revision: "rev-2", PrimaryPort: 80, Ports: []int32{80, 443}}, true},
	} {
		app := webImageApp(2)
		app.Status.ServingNetwork, app.Status.ImageNetwork = serving.DeepCopy(), tc.candidate
		selector, _, err := privateServiceProjection(app, 80)
		if err != nil {
			t.Fatal(err)
		}
		if got, pinned := selector[labelRevision]; pinned != tc.pinned || (pinned && got != "rev-1") {
			t.Errorf("%s: selector %v, pinned=%v want %v (serving rev-1)", tc.name, selector, pinned, tc.pinned)
		}
	}
}
