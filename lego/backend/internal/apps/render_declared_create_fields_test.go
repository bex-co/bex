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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
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
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "app").Spec.Autoscaling
	if a == nil || !a.Enabled || a.MinReplicas != 2 || a.MaxReplicas != 5 || a.TargetCPUPercent == nil || *a.TargetCPUPercent != 65 {
		t.Fatalf("spec.autoscaling = %+v, want enabled 2..5 at 65%% CPU", a)
	}
}

// w4/596: a read carries the policy where Render's schema puts it,
// serviceDetails.autoscaling, equal to GET …/autoscaling's body; a service
// without autoscaling and a cron job carry none.
func TestRESTServiceReadCarriesServiceDetailsAutoscaling(t *testing.T) {
	cron := sampleApp("nightly")
	cron.Spec.Type, cron.Spec.Schedule = appv1alpha1.TypeCronJob, "0 * * * *"
	svc, _ := newService(nil, paidWebApp("plain"), cron)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	h := validated(t, mux)
	get := func(path string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s => %d: %s", path, rec.Code, rec.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	details := func(path string) map[string]any {
		t.Helper()
		return get(path)["serviceDetails"].(map[string]any)
	}

	if _, present := details("/v1/services/plain")["autoscaling"]; present {
		t.Fatal("a service without autoscaling carries serviceDetails.autoscaling")
	}
	if _, err := svc.SetAutoscaling(context.Background(), "plain", SetAutoscalingRequest{MinInstances: 1, MaxInstances: 2, TargetCPUPercent: int32p(60)}); err != nil {
		t.Fatal(err)
	}
	if got, want := details("/v1/services/plain")["autoscaling"], get("/v1/services/plain/autoscaling"); !reflect.DeepEqual(got, want) {
		t.Fatalf("serviceDetails.autoscaling = %v, want GET …/autoscaling's %v", got, want)
	}
	if _, present := details("/v1/services/nightly")["autoscaling"]; present {
		t.Fatal("a cron job carries serviceDetails.autoscaling")
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
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code < 400 || rec.Code >= 500 || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("%s => %d %s, want a 4xx naming %q", tc.name, rec.Code, rec.Body, tc.want)
			}
		})
	}
}
