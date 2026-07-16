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

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func maintenanceApp(uri string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "bex-system"},
		Spec: appv1alpha1.AppSpec{
			MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true, URI: uri},
			Hosts:           []string{"custom.example.com"},
		},
		Status: appv1alpha1.AppStatus{
			URL:  "https://web.onbex.co",
			URLs: []string{"https://web.onbex.co", "https://custom.example.com"},
		},
	}
}

func TestMaintenanceHandlerDoesNotWakeOrMutateWorkload(t *testing.T) {
	app := maintenanceApp("")
	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "bex-system"},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, dep).Build()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://web.onbex.co/arbitrary/path", strings.NewReader("ignored"))
	newHandler(cl, logr.Discard()).ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "currently under maintenance") {
		t.Fatalf("response = %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var got appsv1.Deployment
	if err := cl.Get(context.Background(), clientKey("web"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Replicas == nil || *got.Spec.Replicas != 0 {
		t.Fatalf("maintenance request woke Deployment: replicas=%v", got.Spec.Replicas)
	}
	var gotApp appv1alpha1.App
	if err := cl.Get(context.Background(), clientKey("web"), &gotApp); err != nil {
		t.Fatal(err)
	}
	if gotApp.Annotations[annotLastActive] != "" {
		t.Fatalf("maintenance request touched last-active: %#v", gotApp.Annotations)
	}
}

func TestWakeHandlerNegotiatesContentAndAlwaysWakes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accept   string
		wantType string
		wantBody string
	}{
		{
			name:     "browser gets polling interstitial",
			accept:   "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			wantType: "text/html; charset=utf-8",
			wantBody: "Application loading",
		},
		{
			name:     "API gets retryable JSON",
			accept:   "application/json",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
		{
			name:     "wildcard stays on API default",
			accept:   "*/*",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
		{
			name:     "explicit HTML refusal stays on API default",
			accept:   "text/html;q=0,*/*;q=0.8",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zero := int32(0)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
				Status:     appv1alpha1.AppStatus{URL: "https://sleeping.onbex.co"},
			}
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
				Spec:       appsv1.DeploymentSpec{Replicas: &zero},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, dep).Build()

			req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/ready", nil)
			req.Header.Set("Accept", tc.accept)
			rr := httptest.NewRecorder()
			newHandler(cl, logr.Discard()).ServeHTTP(rr, req)

			if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "5" {
				t.Fatalf("response = %d, Retry-After %q", rr.Code, rr.Header().Get("Retry-After"))
			}
			if got := rr.Header().Get("Content-Type"); got != tc.wantType {
				t.Fatalf("Content-Type = %q, want %q", got, tc.wantType)
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
			if !strings.Contains(rr.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want substring %q", rr.Body.String(), tc.wantBody)
			}
			if tc.wantType == "text/html; charset=utf-8" {
				for _, want := range []string{
					"probeIntervalMs = 5000",
					"fallbackReloadMs = 45000",
					"method: \"HEAD\"",
				} {
					if !strings.Contains(rr.Body.String(), want) {
						t.Errorf("wake page missing %q", want)
					}
				}
			}

			var gotDep appsv1.Deployment
			if err := cl.Get(context.Background(), clientKey("sleeping"), &gotDep); err != nil {
				t.Fatal(err)
			}
			if gotDep.Spec.Replicas == nil || *gotDep.Spec.Replicas != 1 {
				t.Fatalf("wake replicas = %v, want 1", gotDep.Spec.Replicas)
			}
			var gotApp appv1alpha1.App
			if err := cl.Get(context.Background(), clientKey("sleeping"), &gotApp); err != nil {
				t.Fatal(err)
			}
			if gotApp.Annotations[annotLastActive] == "" {
				t.Fatalf("wake did not stamp %s: %#v", annotLastActive, gotApp.Annotations)
			}
		})
	}
}

func TestWakeHTMLHeadHasNoBody(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "https://sleeping.onbex.co/", nil)
	req.Header.Set("Accept", "text/html")
	writeWakeResponse(rr, req)
	if rr.Code != http.StatusServiceUnavailable || rr.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", rr.Code, rr.Body.String())
	}
}

func TestMaintenanceHandlerCoversPlatformAndCustomHostsOnEveryMethod(t *testing.T) {
	app := maintenanceApp("")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	handler := newHandler(cl, logr.Discard())

	for _, tc := range []struct {
		host   string
		method string
		path   string
	}{
		{host: "web.onbex.co", method: http.MethodGet, path: "/"},
		{host: "custom.example.com", method: http.MethodPost, path: "/api/orders"},
		{host: "web.onbex.co", method: http.MethodPut, path: "/deep/path?x=1"},
		{host: "custom.example.com", method: http.MethodDelete, path: "/anything"},
	} {
		t.Run(tc.method+" "+tc.host+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "https://"+tc.host+tc.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "currently under maintenance") {
				t.Fatalf("response = %d %q", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCustomMaintenancePageSuccessAndOriginError(t *testing.T) {
	app := maintenanceApp("https://status.example.com/page")
	for _, tc := range []struct {
		name       string
		originCode int
		wantCode   int
	}{
		{name: "success becomes maintenance 503", originCode: http.StatusOK, wantCode: http.StatusServiceUnavailable},
		{name: "origin error passes through", originCode: http.StatusTeapot, wantCode: http.StatusTeapot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetch := func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.originCode,
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Body:       io.NopCloser(strings.NewReader("custom body")),
				}, nil
			}
			rr := httptest.NewRecorder()
			serveMaintenanceWithFetcher(rr, httptest.NewRequest(http.MethodGet, "/anything", nil), app, fetch)
			if rr.Code != tc.wantCode || rr.Body.String() != "custom body" || rr.Header().Get("Content-Type") != "text/html" {
				t.Fatalf("response = %d %q %#v", rr.Code, rr.Body.String(), rr.Header())
			}
		})
	}

	rr := httptest.NewRecorder()
	serveMaintenanceWithFetcher(rr, httptest.NewRequest(http.MethodGet, "/", nil), app,
		func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
			return nil, errors.New("timeout")
		})
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("fetch failure status = %d", rr.Code)
	}
}

func TestCustomMaintenancePageHeadAndSizeLimit(t *testing.T) {
	app := maintenanceApp("https://status.example.com/page")
	fetchBody := func(body string) func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
		return func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
	}

	head := httptest.NewRecorder()
	headReq := httptest.NewRequest(http.MethodHead, "/any", nil)
	serveMaintenanceWithFetcher(head, headReq, app, fetchBody("must not be written"))
	if head.Code != http.StatusServiceUnavailable || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", head.Code, head.Body.String())
	}

	oversized := httptest.NewRecorder()
	oversizedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	oversizedBody := fetchBody(strings.Repeat("x", customPageMaxSize+1))
	serveMaintenanceWithFetcher(oversized, oversizedReq, app, oversizedBody)
	if oversized.Code != http.StatusBadGateway || !strings.Contains(oversized.Body.String(), "unavailable") {
		t.Fatalf("oversized response = %d %q", oversized.Code, oversized.Body.String())
	}
}

func TestMaintenanceURLSafety(t *testing.T) {
	app := maintenanceApp("")
	for _, raw := range []string{
		"/relative",
		"ftp://example.com/page",
		"https://user:password@status.example.com/page",
		"https://web.onbex.co/page",
		"https://custom.example.com/page",
	} {
		if _, err := validateCustomPageURL(raw, app); err == nil {
			t.Errorf("validateCustomPageURL(%q) succeeded", raw)
		}
	}
	for _, raw := range []string{"https://status.example.com/page", "http://status.example.com"} {
		if _, err := validateCustomPageURL(raw, app); err != nil {
			t.Errorf("validateCustomPageURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1"} {
		if !netutil.UnsafeOriginIP(net.ParseIP(raw)) {
			t.Errorf("netutil.UnsafeOriginIP(%s) = false", raw)
		}
	}
	dial := netutil.SafeDialContext(customPageTimeout)
	_, err := dial(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil || !strings.Contains(err.Error(), "private address") {
		t.Fatalf("netutil.SafeDialContext(loopback) error = %v", err)
	}
}

func TestMaintenanceRedirectPolicyRejectsSelfAndLongChains(t *testing.T) {
	app := maintenanceApp("")
	policy := customPageRedirectPolicy(app)
	self := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/redirected", nil)
	selfVia := []*http.Request{httptest.NewRequest(http.MethodGet, "https://status.example.com", nil)}
	if err := policy(self, selfVia); err == nil || !strings.Contains(err.Error(), "this service") {
		t.Fatalf("redirect-to-self error = %v", err)
	}

	external := httptest.NewRequest(http.MethodGet, "https://status.example.com/final", nil)
	via := make([]*http.Request, maxRedirects+1)
	for i := range via {
		via[i] = httptest.NewRequest(http.MethodGet, "https://status.example.com/hop", nil)
	}
	if err := policy(external, via); err == nil || !strings.Contains(err.Error(), "too many redirects") {
		t.Fatalf("long redirect chain error = %v", err)
	}
}

func TestFindAppByHostCoversSpecAndStatusHosts(t *testing.T) {
	app := maintenanceApp("")
	app.Spec.Host = "legacy.example.com"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	for _, host := range []string{"web.onbex.co", "custom.example.com", "legacy.example.com"} {
		got, err := findAppByHost(context.Background(), cl, host)
		if err != nil || got == nil || got.Name != "web" {
			t.Fatalf("findAppByHost(%q) = %+v, %v", host, got, err)
		}
	}
}

func clientKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: "bex-system"}
}
