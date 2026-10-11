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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const probeTestBaseDomain = "onbex.test"

// probeHostOf is the Host header the container's probe sends, failing unless
// all three probes agree on it.
func probeHostOf(t *testing.T, c corev1.Container) string {
	t.Helper()
	hosts := map[string]bool{}
	for _, p := range []*corev1.Probe{c.StartupProbe, c.ReadinessProbe, c.LivenessProbe} {
		if p == nil {
			t.Fatal("a web service must carry all three probes")
		}
		host := ""
		if p.HTTPGet != nil {
			if p.HTTPGet.Host != "" {
				t.Fatalf("HTTPGet.Host = %q; the probe must connect to the pod, not the public host", p.HTTPGet.Host)
			}
			for _, h := range p.HTTPGet.HTTPHeaders {
				if h.Name == probeHostHeader {
					host = h.Value
				}
			}
		}
		hosts[host] = true
	}
	if len(hosts) != 1 {
		t.Fatalf("probes disagree on Host: %v", hosts)
	}
	for h := range hosts {
		return h
	}
	return ""
}

func httpHealthApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return projectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.Spec.HealthCheckPath = "/cgi-bin/health"
		a.Spec.Expose = true
		a.Spec.Subdomain = "shop-x1"
	}}, mutate...)...)
}

func probeParams() deploymentParams {
	p := webParams()
	p.baseDomain = probeTestBaseDomain
	return p
}

// Render: "the health check request's Host header is your service's verified
// custom domain, or its onrender.com subdomain otherwise". Custom-first is not
// EffectiveHosts' public-URL order, which puts the platform host before
// spec.hosts.
func TestHealthProbeHostSelection(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*appv1alpha1.App)
		want   string
	}{
		{"platform host from the canonical subdomain", func(*appv1alpha1.App) {}, "shop-x1." + probeTestBaseDomain},
		{"platform host falls back to the CR name without a subdomain", func(a *appv1alpha1.App) { a.Spec.Subdomain = "" }, "hello." + probeTestBaseDomain},
		{"primary custom domain first", func(a *appv1alpha1.App) {
			a.Spec.Host = "shop.example.com"
			a.Spec.Hosts = []string{"www.example.com"}
		}, "shop.example.com"},
		{"an additional custom domain beats the platform host", func(a *appv1alpha1.App) { a.Spec.Hosts = []string{"www.example.com"} }, "www.example.com"},
		{"a redirect source is skipped", func(a *appv1alpha1.App) {
			a.Spec.Host = "www.example.com"
			a.Spec.Hosts = []string{"example.com"}
			a.Spec.HostRedirects = map[string]string{"www.example.com": "example.com"}
		}, "example.com"},
		{"legacy empty type is a web service", func(a *appv1alpha1.App) { a.Spec.Type = "" }, "shop-x1." + probeTestBaseDomain},
		{"disabled subdomain and no custom domain sends none", func(a *appv1alpha1.App) { a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled }, ""},
		{"disabled subdomain still uses a custom domain", func(a *appv1alpha1.App) {
			a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled
			a.Spec.Hosts = []string{"api.example.com"}
		}, "api.example.com"},
		{"an unexposed web service sends none", func(a *appv1alpha1.App) { a.Spec.Expose = false }, ""},
		{"a private service keeps the pod-IP Host", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService }, ""},
		{"a private service ignores a stray spec.host", func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypePrivateService
			a.Spec.Host = "shop.example.com"
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := probeHostOf(t, appContainerOf(t, project(httpHealthApp(tc.mutate), probeParams()))); got != tc.want {
				t.Errorf("probe Host = %q; want %q", got, tc.want)
			}
		})
	}

	t.Run("TCP mode carries no header", func(t *testing.T) {
		c := appContainerOf(t, project(httpHealthApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "" }), probeParams()))
		if c.StartupProbe.TCPSocket == nil || c.StartupProbe.HTTPGet != nil {
			t.Fatal("an unset path must stay a TCP probe")
		}
	})
	t.Run("a worker has no probes", func(t *testing.T) {
		p := probeParams()
		p.worker = true
		c := appContainerOf(t, project(httpHealthApp(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeBackgroundWorker }), p))
		if c.StartupProbe != nil || c.ReadinessProbe != nil || c.LivenessProbe != nil {
			t.Error("a worker must stay probe-free")
		}
	})
}

// The finding's fixture: an app that answers its health route only for its own
// hostname. The request is built the way kubelet's prober builds it
// (pkg/probe/http/request.go): connect to the pod address, take Host from the
// probe's headers.
func TestHostnameRestrictedHealthRouteAcceptsTheProbe(t *testing.T) {
	const allowed = "shop-x1." + probeTestBaseDomain
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != allowed {
			w.WriteHeader(http.StatusMisdirectedRequest)
			return
		}
		_, _ = w.Write([]byte("QA_A32_HOST_OK\n"))
	}))
	defer srv.Close()
	podAddr := srv.Listener.Addr().String()
	_, portText, _ := net.SplitHostPort(podAddr)
	port, _ := strconv.Atoi(portText)

	kubeletProbe := func(get *corev1.HTTPGetAction) int {
		t.Helper()
		target := url.URL{Scheme: "http", Host: podAddr, Path: get.Path}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range get.HTTPHeaders {
			if h.Name == probeHostHeader {
				req.Host = h.Value
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	p := probeParams()
	p.port = port
	c := appContainerOf(t, project(httpHealthApp(), p))
	if got := kubeletProbe(c.StartupProbe.HTTPGet); got != http.StatusOK {
		t.Fatalf("probe with the service hostname = %d; want 200", got)
	}
	// Control: the pre-fix probe (no header) presents the pod address and is
	// rejected, which is what kept the release from ever becoming Ready.
	bare := c.StartupProbe.HTTPGet.DeepCopy()
	bare.HTTPHeaders = nil
	if got := kubeletProbe(bare); got != http.StatusMisdirectedRequest {
		t.Fatalf("headerless probe = %d; want the fixture's 421", got)
	}
}

// Domains are operational: editing them opens no deploy, so it must not roll the
// pods either. The Host is chosen when a release is first projected and kept for
// that release.
func TestProbeHostIsFrozenForTheRelease(t *testing.T) {
	release := func(gen int64, mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
		return httpHealthApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) { a.Status.ReleaseGeneration = gen }}, mutate...)...)
	}
	addDomain := func(a *appv1alpha1.App) { a.Spec.Host = "shop.example.com" }
	p := probeParams()
	live := project(release(4), p)

	t.Run("repeated reconcile is a no-op", func(t *testing.T) {
		again := live.DeepCopy()
		applyDeploymentSpec(again, release(4), p)
		if !equalPodTemplate(again.Spec.Template, live.Spec.Template) {
			t.Error("re-projecting the same release changed the template")
		}
	})
	t.Run("a domain-only edit keeps the release's Host", func(t *testing.T) {
		edited := live.DeepCopy()
		applyDeploymentSpec(edited, release(4, addDomain), p)
		if !equalPodTemplate(edited.Spec.Template, live.Spec.Template) {
			t.Errorf("a domain edit rolled the pods (probe Host %q)", probeHostOf(t, appContainerOf(t, edited)))
		}
	})
	t.Run("a removed domain is not retained across releases either", func(t *testing.T) {
		custom := project(release(4, addDomain), p)
		next := custom.DeepCopy()
		applyDeploymentSpec(next, release(5), p)
		if got := probeHostOf(t, appContainerOf(t, next)); got != "shop-x1."+probeTestBaseDomain {
			t.Errorf("next release probe Host = %q; want the current platform host", got)
		}
	})
	t.Run("the next release selects from current domains", func(t *testing.T) {
		next := live.DeepCopy()
		applyDeploymentSpec(next, release(5, addDomain), p)
		if got := probeHostOf(t, appContainerOf(t, next)); got != "shop.example.com" {
			t.Errorf("new release probe Host = %q; want the current custom domain", got)
		}
	})
	t.Run("an operator upgrade does not add the header to a running release", func(t *testing.T) {
		legacy := live.DeepCopy()
		for _, probe := range []*corev1.Probe{
			legacy.Spec.Template.Spec.Containers[0].StartupProbe,
			legacy.Spec.Template.Spec.Containers[0].ReadinessProbe,
			legacy.Spec.Template.Spec.Containers[0].LivenessProbe,
		} {
			probe.HTTPGet.HTTPHeaders = nil
		}
		upgraded := legacy.DeepCopy()
		applyDeploymentSpec(upgraded, release(4), p)
		if !equalPodTemplate(upgraded.Spec.Template, legacy.Spec.Template) {
			t.Error("the upgrade rolled a running release to add the probe Host")
		}
		restarted := legacy.DeepCopy()
		applyDeploymentSpec(restarted, release(5, func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-10-10T00:00:00Z" }), p)
		if got := probeHostOf(t, appContainerOf(t, restarted)); got != "shop-x1."+probeTestBaseDomain {
			t.Errorf("restart release probe Host = %q; want it adopted on the next release", got)
		}
	})
}

// A rollback restores the target release's health path but serves today's
// domains (ADR004), so its probe Host comes from current intent, not history.
func TestRollbackProbesHistoricalPathWithCurrentHost(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	app.Spec.Host = "shop.example.com"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	c := materializeSelectedTestRelease(t, r, app).Spec.Template.Spec.Containers[0]
	if c.StartupProbe.HTTPGet == nil || c.StartupProbe.HTTPGet.Path != "/A" {
		t.Fatalf("rollback probe = %+v; want the historical /A", c.StartupProbe.ProbeHandler)
	}
	if got := probeHostOf(t, c); got != "shop.example.com" {
		t.Errorf("rollback probe Host = %q; want the current custom domain", got)
	}
}

// Cancel restores the served template verbatim — including its probe Host —
// even when the domains changed since.
func TestCancelRestoresTheServedProbeHost(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	app.Spec.Host = "old.example.com"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	live := materializeSelectedTestRelease(t, r, app)
	served := probeHostOf(t, live.Spec.Template.Spec.Containers[0])
	if served != "old.example.com" {
		t.Fatalf("served probe Host = %q; want old.example.com", served)
	}
	app.Status.ActiveRevision = "rev-3"
	app.Status.Image = "image:A"
	app.Generation = 4
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
	app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "4"
	app.Spec.HealthCheckPath = "/canceled"
	app.Spec.Host = "shop.example.com"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ActiveRevision = "rev-3"
	app.Status.ReleaseGeneration = 3
	app.Status.Image = "image:A"
	restore, err := r.servedPodTemplateForCancel(context.Background(), app)
	if err != nil || restore == nil {
		t.Fatalf("restore=%v err=%v", restore, err)
	}
	if err := r.applyServingDeployment(context.Background(), app, live, deploymentParams{image: "image:A", port: 3000, replicas: 2}, restore); err != nil {
		t.Fatal(err)
	}
	if got := probeHostOf(t, live.Spec.Template.Spec.Containers[0]); got != served {
		t.Errorf("cancel probe Host = %q; want the served release's %q", got, served)
	}
}
