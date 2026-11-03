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
// serviceDetails, PATCH: top level) links a Build Source (w2/m254 t005). A
// create that also names its own source, or a link to no build source, is
// refused and writes nothing; an empty or null value is "no build source".
func TestRESTRefusesBuildSourceID(t *testing.T) {
	for _, tc := range []struct {
		method, body, want string
	}{
		{http.MethodPost, `{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"buildSourceId":"bds-1"}}`, "image cannot be set with buildSourceId"},
		{http.MethodPost, `{"name":"new","type":"web_service","serviceDetails":{"buildSourceId":"bds-missing","envSpecificDetails":{"startCommand":"x"}}}`, "names no build source"},
		{http.MethodPost, `{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"buildSourceId":""}}`, ""},
		{http.MethodPatch, `{"buildSourceId":"bds-missing","serviceDetails":{"envSpecificDetails":{"startCommand":"x"}}}`, "names no build source"},
		{http.MethodPatch, `{"buildSourceId":null}`, ""},
		{http.MethodPatch, `{"buildSourceId":""}`, ""},
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
			if tc.want == "" {
				if rec.Code < 200 || rec.Code >= 300 {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s, want 400 %q", rec.Code, rec.Body, tc.want)
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
