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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBlueprintPreviewOptOut(t *testing.T) {
	base := "services:\n  - name: api\n    type: web\n    runtime: image\n    image: {url: nginx}\n"
	for _, tc := range []struct {
		name, root, service string
		accepted            bool
	}{
		{"root off", "previews: {generation: off}\n", "", true},
		{"root off with expiry", "previews: {generation: off, expireAfterDays: 3}\n", "", true},
		{"root false", "previewsEnabled: false\n", "", true},
		{"service off", "", "previews: {generation: off}", true},
		{"service false", "", "pullRequestPreviewsEnabled: false", true},
		{"root automatic", "previews: {generation: automatic}\n", "", false},
		{"root manual", "previews: {generation: manual}\n", "", false},
		{"root enabled", "previewsEnabled: true\n", "", false},
		{"service enabled", "", "pullRequestPreviewsEnabled: true", false},
		{"service automatic", "", "previews: {generation: automatic}", false},
		{"preview plan", "", "previewPlan: free", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := tc.root + base
			if tc.service != "" {
				manifest += "    " + tc.service + "\n"
			}
			_, problems := CompileBlueprintSource(manifest)
			if tc.accepted {
				if len(problems) != 0 {
					t.Fatalf("opt-out refused: %+v", problems)
				}
				return
			}
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
			if problem == nil || problem.Line == 0 || problem.Column == 0 || !strings.Contains(problem.Message, "preview") {
				t.Fatalf("missing located refusal: %+v", problems)
			}
		})
	}
}

func TestRESTPreviewOptOut(t *testing.T) {
	for _, tc := range []struct {
		details  string
		accepted bool
	}{
		{`"previews":{"generation":"off"}`, true},
		{`"previews":{"generation":"off","expireAfterDays":3}`, true},
		{`"pullRequestPreviewsEnabled":"no"`, true},
		{`"previews":{"generation":"automatic"}`, false},
		{`"previews":{"generation":"manual"}`, false},
		{`"pullRequestPreviewsEnabled":"yes"`, false},
		{`"previews":false`, false},
		{`"previews":"no"`, false},
		{`"previews":{"generation":"off","numInstances":2}`, false},
		{`"previews":{"generation":"off","expireAfterDays":0}`, false},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPatch} {
			t.Run(method+tc.details, func(t *testing.T) {
				original := sampleApp("web")
				svc, cl := newService(nil, original)
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				path := "/v1/services/web"
				body := fmt.Sprintf(`{"serviceDetails":{%s}}`, tc.details)
				if method == http.MethodPost {
					path = "/v1/services"
					body = fmt.Sprintf(`{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{%s}}`, tc.details)
				}
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
				if tc.accepted {
					if rec.Code < 200 || rec.Code >= 300 {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
					}
					if method == http.MethodPatch && !reflect.DeepEqual(original.Spec, getApp(t, cl, "web").Spec) {
						t.Fatal("opt-out changed service spec")
					}
				} else if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
				}
			})
		}
	}
}
