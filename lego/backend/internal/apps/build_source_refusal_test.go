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
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestRESTRefusesBuildSourceID: Render's buildSourceId (create:
// serviceDetails, PATCH: top level) names a Build Source bex does not serve
// yet (w2/m254). A value is refused by name and writes nothing; an empty or
// null one is "no build source", which is already true.
func TestRESTRefusesBuildSourceID(t *testing.T) {
	for _, tc := range []struct {
		method, body string
		refused      bool
	}{
		{http.MethodPost, `{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"buildSourceId":"bs-1"}}`, true},
		{http.MethodPost, `{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"buildSourceId":""}}`, false},
		{http.MethodPatch, `{"buildSourceId":"bs-1"}`, true},
		{http.MethodPatch, `{"buildSourceId":null}`, false},
		{http.MethodPatch, `{"buildSourceId":""}`, false},
	} {
		t.Run(tc.method+tc.body, func(t *testing.T) {
			original := sampleApp("web")
			svc, cl := newService(nil, original)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			path := "/v1/services/web"
			if tc.method == http.MethodPost {
				path = "/v1/services"
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, path, strings.NewReader(tc.body)))
			if !tc.refused {
				if rec.Code < 200 || rec.Code >= 300 {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "buildSourceId is not supported") {
				t.Fatalf("status=%d body=%s, want 400 naming buildSourceId", rec.Code, rec.Body)
			}
			if tc.method == http.MethodPatch && !reflect.DeepEqual(original.Spec, getApp(t, cl, "web").Spec) {
				t.Fatal("refused patch changed the service")
			}
			var apps appv1alpha1.AppList
			if err := cl.List(t.Context(), &apps); err != nil {
				t.Fatal(err)
			}
			if len(apps.Items) != 1 {
				t.Fatalf("refused %s left %d Apps, want only the original", tc.method, len(apps.Items))
			}
		})
	}
}
