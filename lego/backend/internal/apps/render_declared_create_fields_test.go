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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Render's spec declares routes and headers inside the static-site
// serviceDetails variant, not only at the top level. A client following the
// spec must get the rules it sent, not a site without them.
func TestRESTCreateStaticSiteTakesRoutesAndHeadersFromServiceDetails(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"site","type":"static_site","repo":"https://github.com/acme/site",
		"serviceDetails":{"publishPath":"dist",
			"routes":[{"type":"rewrite","source":"/*","destination":"/index.html"}],
			"headers":[{"path":"/*","name":"X-Frame-Options","value":"DENY"}]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "site")
	if len(app.Spec.Routes) != 1 || app.Spec.Routes[0].Destination != "/index.html" {
		t.Errorf("spec.routes = %+v, want the nested rewrite", app.Spec.Routes)
	}
	if len(app.Spec.Headers) != 1 || app.Spec.Headers[0].Name != "X-Frame-Options" {
		t.Errorf("spec.headers = %+v, want the nested header", app.Spec.Headers)
	}
}

func TestRESTCreateTakesAutoscalingFromServiceDetails(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"app","type":"web_service","image":{"imagePath":"nginx:v1"},
		"serviceDetails":{"autoscaling":{"enabled":true,"min":2,"max":5,
			"criteria":{"cpu":{"enabled":true,"percentage":65},"memory":{"enabled":false,"percentage":0}}}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "app").Spec.Autoscaling
	if a == nil || !a.Enabled || a.MinReplicas != 2 || a.MaxReplicas != 5 || a.TargetCPUPercent == nil || *a.TargetCPUPercent != 65 {
		t.Fatalf("spec.autoscaling = %+v, want enabled 2..5 at 65%% CPU", a)
	}
}

func TestRESTRefusesDeclaredButUnservedFieldsByName(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, want string
	}{
		{"create pullRequestPreviewsEnabled", http.MethodPost, "/v1/services",
			`{"name":"app","type":"web_service","image":{"imagePath":"nginx:v1"},"serviceDetails":{"pullRequestPreviewsEnabled":"yes"}}`,
			"pullRequestPreviewsEnabled"},
		{"patch an unknown runtime", http.MethodPatch, "/v1/services/app", `{"serviceDetails":{"runtime":"cobol"}}`, "unsupported runtime"},
		{"patch an unknown cache profile", http.MethodPatch, "/v1/services/app", `{"serviceDetails":{"cache":{"profile":"aggressive"}}}`, "cache.profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, sampleApp("app"))
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code < 400 || rec.Code >= 500 || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("%s => %d %s, want a 4xx naming %q", tc.name, rec.Code, rec.Body, tc.want)
			}
		})
	}
}
